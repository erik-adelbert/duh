// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

func (inst *ITInstrument) fadeoutStep() float32 {
	if inst.hasMPTFadeout {
		return float32(inst.mptFadeout) / 32768
	}
	return float32(inst.Fadeout) / 1024
}

func processITInstrument(w *Tracker, voice int) {
	life := &w.VoiceLives[voice]
	amp := &w.VoiceAmps[voice]
	previousVolume, previousPan := life.ITEnvelopeVolume, life.ITPanEnvelopeValue
	amp.itEnvelopeRamping = false
	inst := &w.Assets.ITInstruments[life.ITInstrument-1]
	env := &inst.VolumeEnvelope

	if itEnvelopeEnabled(env.Enabled, life.itVolumeEnvelopeControl) && len(env.Points) != 0 {
		tick := life.ITEnvelopeTick
		value := itEnvelopeValue(env, tick)

		life.ITEnvelopeVolume = value / 64
		if inst.mptHasVolumeRelease && life.mptEnvelopeReleased {
			life.ITEnvelopeVolume = mptReleasedVolume(env, inst.mptVolumeReleaseNode, tick, life.mptReleaseValue)
		}

		lastTick := int(env.Points[len(env.Points)-1].Tick)
		ended := !env.Loop && (!env.Sustain || life.KeyOff)
		if ended && tick >= lastTick && value == 0 {
			w.stopVoice(VoiceID(voice))
			return
		}
		if ended && tick > lastTick {
			life.ITFade = true
		}

		life.ITEnvelopeTick = itNextEnvelopeTick(env, tick, life.KeyOff)
		if ended && tick >= lastTick {
			life.ITEnvelopeTick = lastTick + 1
		}

		life.mptEnvelopePosition = tick + 1
		last := env.Points[len(env.Points)-1]
		if w.Assets.ITCompatibility.oplEnvelopeEndCut && w.VoiceSamples[voice].oplChannel != 0 &&
			!env.Loop && !env.Sustain && last.Value == 0 && life.ITEnvelopeTick >= int(last.Tick) {
			track := life.ParentTrack

			if w.Assets.ITCompatibility.oplNoResetAtEnvelopeEnd && track >= 0 && int(track) < len(w.TrackCursors) &&
				w.TrackCursors[track].ActiveVoice == VoiceID(voice) {
				cursor := &w.TrackCursors[track]
				cursor.oplEndedPitch = w.VoicePitches[voice]
				cursor.oplEndedSample = w.VoiceSamples[voice].SampleID
				cursor.oplHasEndedPitch = true
				// OpenMPT drops the sample length after its mixer volumes have
				// reached zero, then clears the playback increment one tick later.
				amp := &w.VoiceAmps[voice]
				cursor.oplEndedTicks = 1
				mix := w.Assets.ITCompatibility.mptMixProfile
				if (amp.TargetLeft != 0 || amp.TargetRight != 0) && (mix == nil || mix.pcmGain != 0) {
					cursor.oplEndedTicks++
					if voiceVolume(w, voice, cursor.oplEndedSample) != 0 {
						cursor.oplEndedTicks++
					}
				}
			}

			w.stopVoice(VoiceID(voice))

			return
		}
	}

	if env := &inst.PanEnvelope; itEnvelopeEnabled(env.Enabled, life.itPanEnvelopeControl) && len(env.Points) != 0 {
		life.ITPanEnvelopeValue = itEnvelopeValue(env, life.ITPanEnvelopeTick)
		life.ITPanEnvelopeTick = itNextEnvelopeTick(env, life.ITPanEnvelopeTick, life.KeyOff)
	}

	if env := &inst.PitchEnvelope; itEnvelopeEnabled(env.Enabled, life.itPitchEnvelopeControl) && len(env.Points) != 0 {
		life.ITPitchEnvelopeValue = itEnvelopeValue(env, life.ITPitchEnvelopeTick)
		if inst.FilterEnvelope {
			updateITFilterEnvelope(w, voice, life.ITPitchEnvelopeValue)
		}
		life.ITPitchEnvelopeTick = itNextEnvelopeTick(env, life.ITPitchEnvelopeTick, life.KeyOff)
	} else if inst.FilterEnvelope && inst.PitchEnvelope.Enabled && len(inst.PitchEnvelope.Points) != 0 &&
		w.Assets.ITCompatibility.mptEnvelopePositionHandling && w.Assets.ITCompatibility.mptStoppedFilterEnvAtStart &&
		life.ITPitchEnvelopeTick == 0 {
		updateITFilterEnvelope(w, voice, 0)
	}

	amp.itEnvelopeRamping = life.mptPanEnvelopeChanged || life.ITEnvelopeVolume != previousVolume || life.ITPanEnvelopeValue != previousPan
	life.mptPanEnvelopeChanged = false
	if life.ITFade {
		previousFade := life.FadeOutVol
		life.FadeOutVol -= inst.fadeoutStep()
		amp.itEnvelopeRamping = amp.itEnvelopeRamping || life.FadeOutVol != previousFade
		if life.FadeOutVol <= 0 {
			w.stopVoice(VoiceID(voice))
		}
	}
}

type itEnvelopeControl uint8

const (
	itEnvelopeDefault itEnvelopeControl = iota
	itEnvelopeDisabled
	itEnvelopeEnabledOverride
)

func itEnvelopeEnabled(initial bool, control itEnvelopeControl) bool {
	return control == itEnvelopeEnabledOverride || control == itEnvelopeDefault && initial
}

func itEnvelopeValue(env *ITVolumeEnvelope, tick int) float32 {
	value := float32(env.Points[0].Value)

	if tick <= int(env.Points[0].Tick) {
		return value
	}

	for i := 1; i < len(env.Points); i++ {
		prev, next := env.Points[i-1], env.Points[i]
		if tick == int(next.Tick) {
			return float32(next.Value)
		}

		if tick < int(next.Tick) {
			if tick > int(prev.Tick) {
				value = float32(prev.Value) + float32(next.Value-prev.Value)*float32(tick-int(prev.Tick))/float32(next.Tick-prev.Tick)
			}
			break
		}

		value = float32(next.Value)
	}
	return value
}

func itNextEnvelopeTick(env *ITVolumeEnvelope, tick int, keyOff bool) int {
	tick++

	if env.Sustain && !keyOff {
		if tick > int(env.Points[env.SustainEnd].Tick) {
			return int(env.Points[env.SustainStart].Tick)
		}
	} else if env.Loop && tick > int(env.Points[env.LoopEnd].Tick) {
		return int(env.Points[env.LoopStart].Tick)
	}

	return min(tick, int(env.Points[len(env.Points)-1].Tick))
}
