package mixer

import (
	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/help"
	"github.com/erik-adelbert/duh/internal/id"
	mod "github.com/erik-adelbert/duh/internal/tracker/components/module"
	"github.com/erik-adelbert/duh/internal/tracker/components/sample"
	ac "github.com/erik-adelbert/duh/internal/tracker/components/voice"
)

const maxVoices = 64

func voiceMixer(w *ecs.World, mixBus *mod.MixBus, offset *ac.Offset, voices []id.ID, features ac.Flags, sampleCount int) int {
	mixedCount := 0

VoiceScan:
	for _, vid := range voices {
		controls, _ := ecs.ComponentAs[ac.MixingControl](w, auto, vid)
		active := controls.Active

		if !active {
			continue
		}

		flags, ok1 := ecs.ComponentAs[ac.Flags](w, auto, vid)
		mixParms, ok2 := ecs.ComponentAs[ac.MixingParams](w, auto, vid)

		if !ok1 || !ok2 {
			continue
		}

		updateVoice := func() {
			ecs.SetComponent(w, auto, vid, controls)
			ecs.SetComponent(w, auto, vid, flags)
			ecs.SetComponent(w, auto, vid, mixParms)
		}

		fastmono := isFastmono(flags, mixParms)
		withRamp := mixParms.Ramp.Length > 0
		withReverb := features.Has(ac.VoiceReverb) || flags.Has(ac.VoiceReverb)

		rid := mixingRoutine(features, flags, fastmono, withRamp)
		buf := prepareBuffer(mixBus, withReverb, sampleCount)

		mixedCount++

		// cur, nxt := buf, buf

		for n := sampleCount; n > 0; {
			// cur, nxt = nxt, cur

			rampLen := min(n, int(mixParms.Ramp.Length))
			mixLen := mixableCount(&flags, &mixParms, rampLen)

			if mixLen <= 0 {
				// Stop mixing this channel
				// active = false
				flags := &flags // local pointer

				flags.Set(ac.VoiceBounce, false)

				mixParms.Ramp.Length = 0
				mixParms.FPart, mixParms.IPart = 0, 0

				o := mixParms.Offset

				buf.StereoFill(&o.Left, &o.Right, n)

				offset.Left += o.Left
				offset.Right += o.Right

				mixParms.Offset = ac.Offset{} // zero it out since it's been applied to the output buffer

				// Stop mixing this channel and continue to the next one
				updateVoice()

				continue VoiceScan
			}

			if mixedCount >= maxVoices || isMute(mixParms) {
				δ := mixParms.Step*int32(mixLen) + mixParms.FPart
				mixParms.IPart += hi16(δ)
				mixParms.FPart = lo16(δ)
				mixParms.Offset = ac.Offset{}
			} else {
				instru, _ := ecs.ComponentAs[ac.Instrument](w, auto, vid)
				sample, _ := ecs.ComponentAs[sample.Data](w, auto, instru.Sample)
				mixer := Patch[rid]

				mixer(buf.S284[:mixLen*2], &mixParms.Cue, &mixParms.Gain, &mixParms.Filter, sample, flags)

				mixedCount++
			}

			n -= mixLen
			buf = buf.Slice(mixLen*2, buf.Len())
		}

		updateVoice() // Update the channel components after processing
	}

	return mixedCount
}

func isMute(vmix ac.MixingParams) bool {
	return vmix.Volume.Left|vmix.Volume.Right == 0 && vmix.Ramp.Length == 0
}

func isFastmono(voice ac.Flags, vmix ac.MixingParams) bool {
	return voice < ac.VoiceStereo &&
		vmix.Volume.Left == vmix.Volume.Right &&
		(vmix.Ramp.Length != 0 || vmix.Ramp.Left == vmix.Ramp.Right)
}

func mixingRoutine(features, voice ac.Flags, fastmono bool, ramp bool) PatchID {
	bit16 := voice.Has(ac.Voice16Bit)
	stereo := !fastmono && voice.Has(ac.VoiceStereo)
	filter := !fastmono && voice.Has(ac.VoiceFiltering)

	var fir, spline, linear bool

	if !voice.Has(ac.VoiceNearest) {
		switch {
		case features.Has(ac.VoiceHQResampling | ac.VoiceHQSource):
			fir = true
		case features.Has(ac.VoiceHQResampling):
			spline = true
		default:
			linear = true
		}
	}

	return ac.HashPatchID(fastmono, stereo, bit16, ramp, filter, linear, spline, fir)
}

func prepareBuffer(vbus *mod.MixBus, hasReverb bool, sampleCount int) *Buffer {
	buf := vbus.SoundBuf

	if hasReverb {
		buf = vbus.ReverbBuf

		if vbus.ReverbSend == 0 {
			buf.Resize(sampleCount)
			buf.Clear()
		}

		vbus.ReverbSend += sampleCount
	}

	return buf
}

func mixableCount(voice *ac.Flags, vmix *ac.MixingParams, sampleCount int) int {
	var loopStart int32

	if voice.Has(ac.VoiceLoop) {
		loopStart = vmix.Start
	}

	step := vmix.Step

	if sampleCount <= 0 || step == 0 || vmix.Length == 0 {
		return 0
	}

	switch {
	case vmix.IPart < loopStart:
		switch {
		case step >= 0 && vmix.IPart < 0:
			vmix.IPart = 0
		case step < 0:
			δpos := fp16(loopStart-vmix.IPart, vmix.FPart)
			vmix.IPart = loopStart + hi16(δpos)
			vmix.FPart = lo16(δpos)

			if vmix.IPart < loopStart || vmix.IPart >= (loopStart+vmix.Length)/2 {
				vmix.IPart = loopStart
				vmix.FPart = 0
			}

			step = -step
			vmix.Step = step

			voice.Set(ac.VoiceBounce, false)

			if !voice.Has(ac.VoiceLoop) || vmix.IPart > vmix.Length {
				vmix.IPart = vmix.Length
				vmix.FPart = 0

				return 0
			}
		}
	case vmix.IPart >= vmix.Length:
		switch {
		case !voice.Has(ac.VoiceLoop):
			return 0
		case voice.Has(ac.VoiceBounce):
			if step > 0 {
				step = -step
				vmix.Step = step
			}

			voice.Set(ac.VoiceBounce, true)

			δ := fp16(vmix.IPart-vmix.Length, (1<<16)-vmix.FPart)
			vmix.IPart = vmix.Length - hi16(δ)
			vmix.FPart = lo16(δ)

			if vmix.IPart <= loopStart || vmix.IPart >= vmix.Length {
				vmix.IPart = vmix.Length - 1
			}
		default:
			vmix.IPart += loopStart - vmix.Length
			vmix.IPart = max(vmix.IPart, loopStart)
		}
	}

	ip, fp := vmix.IPart, vmix.FPart

	if ip < 0 || ip >= vmix.Length || (ip < loopStart && step <= 0) {
		return 0
	}

	count := sampleCount

	step = abs(step)

	δi := hi16(step) * int32(sampleCount-1)
	δf := lo16(step) * int32(sampleCount-1)
	maxSamples := max((1<<14)/(step>>16+1), 2)
	sampleCount = min(sampleCount, int(maxSamples))

	var dst int32

	if vmix.Step < 0 {
		dst = hi16(fp16(ip-δi, fp-δf))

		if dst < loopStart {
			count = int(fp16(ip-loopStart, fp-1)/step + 1)
		}
	} else {
		dst = hi16(fp16(ip+δi, fp+δf))

		if dst >= vmix.Length {
			count = int(fp16(vmix.Length-ip, 1-fp)/step + 1)
		}
	}

	return help.Clamp(count, 1, sampleCount)
}

func fp16(i, f int32) int32 {
	return (i << 16) | (f & 0xFFFF)
}

func hi16(fp1616 int32) int32 {
	return fp1616 >> 16
}

func lo16(fp1616 int32) int32 {
	return fp1616 & 0xFFFF
}

type numeric interface {
	~int | ~int32 | ~int64 | ~uint | ~uint32 | ~uint64 | ~float32 | ~float64
}

func abs[T numeric](x T) T {
	if x < 0 {
		return -x
	}

	return x
}
