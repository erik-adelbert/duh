// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.

package mod

import "math"

const (
	modArpeggio       = 0x0
	modSlideUp        = 0x1
	modSlideDown      = 0x2
	modTonePorta      = 0x3
	modVibrato        = 0x4
	modPortaVolume    = 0x5
	modVibratoVolume  = 0x6
	modTremolo        = 0x7
	modSetPanning     = 0x8
	modSampleOffset   = 0x9
	modVolumeSlide    = 0xA
	modPositionJump   = 0xB
	modSetVolume      = 0xC
	modPatternBreak   = 0xD
	modExtended       = 0xE
	modSpeedTempo     = 0xF
	modLEDFilter      = 0x0
	modFineSlideUp    = 0x1
	modFineSlideDown  = 0x2
	modPatternLoop    = 0x6
	modRetrigger      = 0x9
	modFineVolumeUp   = 0xA
	modFineVolumeDown = 0xB
	modNoteCut        = 0xC
	modNoteDelay      = 0xD
	modPatternDelay   = 0xE
)

func (w *Tracker) applyProTrackerFlow() {
	c := &w.Clock
	if c.Tick != 0 {
		return
	}
	for track, cmd := range w.PendingCmds[:w.Assets.NumTracks] {
		arg := int(cmd.EffectArg)
		switch cmd.EffectCmd {
		case modSpeedTempo:
			if arg >= 32 {
				c.Tempo = float64(arg)
			} else if arg != 0 {
				c.Speed = arg
			}
		case modPositionJump:
			w.modJumpOrder = arg & 127
			if w.modJumpOrder >= len(w.Assets.OrderList) {
				w.modJumpOrder = 0
			}
			w.modBreakRow = 0
		case modPatternBreak:
			row := 10*(arg>>4) + (arg & 15)
			if row >= modPatternRows {
				row = 0
			}
			w.modBreakRow = row
		case modExtended:
			switch arg >> 4 {
			case modLEDFilter:
				w.modFilterEnabled = arg&1 == 0
			case modPatternLoop:
				cursor := &w.TrackCursors[track]
				if count := uint8(arg & 15); count == 0 {
					cursor.modLoopStart = uint8(c.Row)
				} else {
					if cursor.modLoopCount == 0 {
						cursor.modLoopCount = count
					} else {
						cursor.modLoopCount--
					}
					if cursor.modLoopCount != 0 {
						w.modLoopRow = int(cursor.modLoopStart)
					}
				}
			case modPatternDelay:
				w.mptRowDelay = arg & 15
			}
		}
	}
	c.SamplesPerTick = samplesPerTick(c, true)
	c.SamplesRemaining = c.SamplesPerTick
}

func (w *Tracker) allocateProTrackerVoice(track int, cmd ModCommand) {
	cursor := &w.TrackCursors[track]
	mem := &w.TrackMemories[track]
	tick := w.Clock.Tick
	if tick == 0 {
		if cmd.EffectCmd == modSetPanning {
			pan := int(cmd.EffectArg) - 128
			panning := float32(pan) / 128
			if pan > 0 {
				panning = float32(pan) / 127
			}
			setTrackPanning(w, TrackID(track), cursor.ActiveVoice, panning)
		}
		if cmd.Instrument != 0 {
			mem.LastInstrument = cmd.Instrument
			cursor.modVolume = w.Assets.Samples[cmd.Instrument-1].Volume
		}
		if cmd.EffectCmd == modSampleOffset && cmd.EffectArg != 0 {
			mem.LastOffset = cmd.EffectArg
		}
	}
	sample := int(mem.LastInstrument) - 1
	if sample < 0 {
		return
	}
	triggerNote := tick == 0
	if cmd.EffectCmd == modExtended {
		// The scheduler keeps an absolute tick; timed effects use the tick within each repetition.
		rowTick := tick % w.Clock.Speed
		switch cmd.EffectArg >> 4 {
		case modRetrigger:
			interval := int(cmd.EffectArg & 15)
			if interval != 0 && rowTick%interval == 0 && (rowTick != 0 || cmd.modPeriod == 0) && cursor.modCurrentPeriod != 0 {
				w.startSampleVoice(TrackID(track), cmd, sample, 0, 0)
				return
			}
		case modNoteDelay:
			triggerNote = rowTick == int(cmd.EffectArg&15)
		}
	}
	if !triggerNote || cmd.modPeriod == 0 {
		return
	}

	asset := &w.Assets.Samples[sample]
	period := proTrackerPeriod(cmd.modPeriod, asset.modFineTune)
	if cmd.EffectCmd == modTonePorta || cmd.EffectCmd == modPortaVolume {
		cursor.modTargetPeriod = period
		return
	}
	cursor.modCurrentPeriod = period
	if cmd.EffectCmd == modSampleOffset && int(mem.LastOffset)*256 >= len(asset.Data) {
		if cursor.ActiveVoice >= 0 {
			w.stopVoice(cursor.ActiveVoice)
		}
		return
	}
	w.startSampleVoice(TrackID(track), cmd, sample, 0, 0)
	cursor.modVibratoPhase = 0
	cursor.modTremoloPhase = 0
}

func proTrackerPeriod(period uint16, fineTune int8) int {
	// Approximate the integer finetune table without quantizing untuned notes.
	return max(1, int(math.Round(float64(period)*math.Exp2(-float64(fineTune)/96))))
}

func proTrackerFrequency(period int) float64 {
	return 3546895 / float64(period)
}

func (w *Tracker) applyProTrackerPitch(track int, cmd ModCommand) {
	cursor := &w.TrackCursors[track]
	tick := w.Clock.Tick
	if tick == 0 && cmd.EffectCmd == modTonePorta && cmd.EffectArg != 0 {
		cursor.modPortaSpeed = int(cmd.EffectArg)
	}
	if tick == 0 && cmd.EffectCmd == modVibrato {
		if speed := cmd.EffectArg >> 4; speed != 0 {
			cursor.modVibratoSpeed = speed
		}
		if depth := cmd.EffectArg & 15; depth != 0 {
			cursor.modVibratoDepth = depth
		}
	}
	period := cursor.modCurrentPeriod
	if period == 0 {
		return
	}
	if cmd.EffectCmd == modExtended && tick%w.Clock.Speed == 0 {
		switch cmd.EffectArg >> 4 {
		case modFineSlideUp:
			period = max(113, period-int(cmd.EffectArg&15))
		case modFineSlideDown:
			period = min(856, period+int(cmd.EffectArg&15))
		}
	}
	if tick != 0 {
		switch cmd.EffectCmd {
		case modSlideUp:
			if cmd.EffectArg != 0 {
				period = max(113, period-int(cmd.EffectArg))
			}
		case modSlideDown:
			if cmd.EffectArg != 0 {
				period = min(856, period+int(cmd.EffectArg))
			}
		case modTonePorta, modPortaVolume:
			target, speed := cursor.modTargetPeriod, cursor.modPortaSpeed
			if target != 0 {
				if period > target {
					period = max(target, period-speed)
				} else {
					period = min(target, period+speed)
				}
			}
		}
	}
	cursor.modCurrentPeriod = period
	if tick != 0 && (cmd.EffectCmd == modVibrato || cmd.EffectCmd == modVibratoVolume) {
		delta := proTrackerSineDelta(cursor.modVibratoPhase, cursor.modVibratoDepth, 7)
		period = max(1, period+delta)
		cursor.modVibratoPhase += cursor.modVibratoSpeed
	}
	if voice := cursor.ActiveVoice; voice >= 0 && w.VoiceLives[voice].Active {
		if cmd.EffectCmd == modArpeggio && cmd.EffectArg != 0 {
			var semitones uint8
			switch (tick % w.Clock.Speed) % 3 {
			case 1:
				semitones = cmd.EffectArg >> 4
			case 2:
				semitones = cmd.EffectArg & 15
			}
			period = proTrackerPeriod(uint16(period), int8(semitones*8))
		}
		w.VoicePitches[voice].BaseFreqHz = proTrackerFrequency(period)
	}
}

func proTrackerSineDelta(phase, depth uint8, shift uint) int {
	sine := int(255 * math.Sin(float64(phase&31)*math.Pi/32))
	delta := sine * int(depth) >> shift
	if phase&32 != 0 {
		delta = -delta
	}
	return delta
}

func (w *Tracker) filterProTrackerMix(stereo []float32) {
	if !w.modFilterEnabled {
		w.modFilterState = [2][2]float64{}
		return
	}
	// Two-pole low-pass approximation of the switchable Amiga LED filter.
	rate := w.Clock.SampleRate
	k := math.Tan(math.Pi * min(3275, rate*0.45) / rate)
	norm := 1 / (1 + math.Sqrt2*k + k*k)
	b0 := k * k * norm
	a1, a2 := 2*(k*k-1)*norm, (1-math.Sqrt2*k+k*k)*norm
	for i, x := range stereo {
		z := &w.modFilterState[i&1]
		in := float64(x) * b0
		out := in + z[0]
		z[0], z[1] = 2*in-a1*out+z[1], in-a2*out
		stereo[i] = float32(out)
	}
}

func (w *Tracker) applyProTrackerVolume(track TrackID, cmd ModCommand) {
	cursor := &w.TrackCursors[track]
	switch cmd.EffectCmd {
	case modTremolo:
		if w.Clock.Tick == 0 {
			if speed := cmd.EffectArg >> 4; speed != 0 {
				cursor.modTremoloSpeed = speed
			}
			if depth := cmd.EffectArg & 15; depth != 0 {
				cursor.modTremoloDepth = depth
			}
		}
	case modExtended:
		rowTick := w.Clock.Tick % w.Clock.Speed
		arg := cmd.EffectArg & 15
		switch cmd.EffectArg >> 4 {
		case modFineVolumeUp:
			if rowTick == 0 {
				cursor.modVolume = min(64, cursor.modVolume+arg)
			}
		case modFineVolumeDown:
			if rowTick == 0 {
				cursor.modVolume -= min(cursor.modVolume, arg)
			}
		case modNoteCut:
			if rowTick == int(arg) {
				cursor.modVolume = 0
			}
		}
	case modSetVolume:
		if w.Clock.Tick == 0 {
			cursor.modVolume = min(cmd.EffectArg, 64)
		}
	case modVolumeSlide, modPortaVolume, modVibratoVolume:
		if w.Clock.Tick != 0 {
			delta := int(cmd.EffectArg >> 4)
			if delta == 0 {
				delta = -int(cmd.EffectArg & 15)
			}
			cursor.modVolume = uint8(max(0, min(64, int(cursor.modVolume)+delta)))
		}
	}
	volume := int(cursor.modVolume)
	if cmd.EffectCmd == modTremolo && w.Clock.Tick != 0 {
		volume = max(0, min(64, volume+proTrackerSineDelta(cursor.modTremoloPhase, cursor.modTremoloDepth, 6)))
		cursor.modTremoloPhase += cursor.modTremoloSpeed
	}
	if voice := cursor.ActiveVoice; voice >= 0 && w.VoiceLives[voice].Active {
		w.VoiceAmps[voice].Volume = float32(volume) / 64
	}
}
