// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import "math"

// System 4A: Evaluates pitch slides, tone portamento, and vibrato LFOs.
func PitchEffectSystem(w *Tracker) {
	for t := 0; t < w.Assets.NumTracks; t++ {
		vID := w.TrackCursors[t].ActiveVoice
		cmd := w.PendingCmds[t]
		mem := &w.TrackMemories[t]

		if w.Assets.s3m && (cmd.EffectCmd == 0x04 || cmd.EffectCmd == 0x21) {
			rememberS3MVibrato(mem, cmd, w.Assets.s3mSharedEffectMemory)
		}

		if w.Assets.ITCompatibility.mptMixProfile != nil && cmd.EffectCmd == 0x24 && w.itRepeatedRowTick() == 0 {
			microTuning := int16(int(cmd.EffectArg)*256 - 32768)
			w.TrackCursors[t].mptMicroTuning = microTuning
			if vID != -1 && w.VoiceLives[vID].Active {
				w.VoicePitches[vID].mptMicroTuning = microTuning
			}
		}

		w.applyITWaveformSelection(mem, cmd)

		if w.Assets.IT && (w.Assets.ITCompatibility.LegacyModPlug || w.Assets.ITCompatibility.mptSharedVolumeColumnMemory) && cmd.HasVolume {
			var param uint8
			switch {
			case cmd.Volume >= 65 && cmd.Volume <= 74:
				param = cmd.Volume - 65
			case cmd.Volume >= 75 && cmd.Volume <= 84:
				param = cmd.Volume - 75
			case cmd.Volume >= 85 && cmd.Volume <= 94:
				param = cmd.Volume - 85
			case cmd.Volume >= 95 && cmd.Volume <= 104:
				param = cmd.Volume - 95
			case cmd.Volume >= 105 && cmd.Volume <= 114:
				param = cmd.Volume - 105
			case cmd.Volume >= 115 && cmd.Volume <= 124:
				param = cmd.Volume - 115
			case cmd.Volume >= 203 && cmd.Volume <= 212:
				param = cmd.Volume - 203
			}
			if param != 0 {
				mem.itSharedVolParam = param
			}
			if w.Assets.ITCompatibility.LegacyModPlug && cmd.Volume > 203 && cmd.Volume <= 212 {
				mem.LastVibrato = (cmd.Volume-203)<<4 | mem.LastVibrato&0x0f
			} else if cmd.Volume == 203 && mem.itSharedVolParam != 0 {
				mem.LastVibrato = mem.LastVibrato&0xf0 | mem.itSharedVolParam
				mem.itFineVibratoDepth = false
			}
		}

		if w.Assets.IT && !w.Assets.ITCompatibility.LegacyModPlug && cmd.HasVolume &&
			cmd.Volume > 203 && cmd.Volume <= 212 {
			mem.LastVibrato = mem.LastVibrato&0xf0 | (cmd.Volume - 203)
			mem.itFineVibratoDepth = false
		}

		if (cmd.EffectCmd == 0x1a || cmd.EffectCmd == 0x1b) && !w.mptRetriggerAfterVolume(cmd) {
			applyITRetrigger(w, TrackID(t), vID, cmd)
		}

		if w.Assets.IT && !w.Assets.ITCompatibility.LegacyModPlug && cmd.EffectCmd == 0x10 && cmd.EffectArg != 0 && w.Clock.Tick == 0 {
			mem.LastArpeggio = cmd.EffectArg
		}

		if w.Assets.IT && (cmd.EffectCmd == 0x04 || cmd.EffectCmd == 0x21) && w.Clock.Tick == 0 {
			if cmd.EffectArg&0xf0 != 0 {
				mem.LastVibrato = cmd.EffectArg&0xf0 | mem.LastVibrato&0x0f
			}
			if cmd.EffectArg&0x0f != 0 {
				mem.LastVibrato = mem.LastVibrato&0xf0 | cmd.EffectArg&0x0f
				mem.itFineVibratoDepth = cmd.EffectCmd == 0x21
			}
		}

		doublePortamento := w.Assets.IT && w.Assets.ITCompatibility.mptDoublePortamentoSlides
		volPorta := w.Assets.IT && itHasVolumeTonePortamento(cmd)
		volumeSlide := w.Assets.IT && cmd.HasVolume && cmd.Volume >= 105 && cmd.Volume <= 124

		if doublePortamento && w.Clock.Tick == 0 {
			w.initMPTDoublePortamentoMemory(mem, cmd, volPorta, volumeSlide)
		}

		if !doublePortamento && w.Assets.IT && cmd.EffectCmd == 0x03 && !w.Assets.ITCompatibility.LegacyModPlug {
			initITTonePortamentoMemory(w, mem, cmd.EffectArg)
		}

		if !doublePortamento && volPorta && (!w.Assets.ITCompatibility.LegacyModPlug || vID == -1 || !w.VoiceLives[vID].Active) {
			initITTonePortamentoMemory(w, mem, itVolumePortamentoSpeed(cmd.Volume))
		}

		if !doublePortamento && w.Assets.IT && (cmd.EffectCmd == 0x01 || cmd.EffectCmd == 0x02) && cmd.EffectArg != 0 {
			mem.LastPortaUp, mem.LastPortaDown = cmd.EffectArg, cmd.EffectArg
			if w.Assets.ITCompatibility.itSharesPortamentoMemory() {
				mem.LastPortaSpeed = cmd.EffectArg
			}
		}
		if w.Assets.s3m && (cmd.EffectCmd == 0x01 || cmd.EffectCmd == 0x02) && cmd.EffectArg != 0 {
			mem.LastPortaUp, mem.LastPortaDown = cmd.EffectArg, cmd.EffectArg
		}

		var volumeSlideParam uint8
		volumeSlideDirection := 1

		if volumeSlide {
			base := uint8(115)
			if cmd.Volume < 115 {
				base, volumeSlideDirection = 105, -1
			}
			volumeSlideParam = (cmd.Volume - base) * 4
			if doublePortamento {
				volumeSlideParam = mem.LastPortaUp
			} else if volumeSlideParam != 0 {
				mem.LastPortaUp, mem.LastPortaDown = volumeSlideParam, volumeSlideParam
				if !w.Assets.ITCompatibility.CompatibleGxx && !w.Assets.ITCompatibility.mptSeparatePortamentoMemory {
					mem.LastPortaSpeed = volumeSlideParam
				}
			} else {
				volumeSlideParam = mem.LastPortaUp
			}
		}

		if w.Assets.IT && w.Assets.ITCompatibility.LegacyModPlug && cmd.EffectCmd == 0x03 && !volPorta {
			initITTonePortamentoMemory(w, mem, cmd.EffectArg)
		}

		if vID == -1 || !w.VoiceLives[vID].Active {
			cursor := &w.TrackCursors[t]

			if cursor.oplHasEndedPitch && volumeSlide {
				if !itVolumePitchSlideState(&cursor.oplEndedPitch, &w.Assets.Samples[cursor.oplEndedSample],
					w.Assets.ITCompatibility, volumeSlideParam, volumeSlideDirection, itPitchSlideTick(w, volumeSlideParam)) {
					cursor.oplHasEndedPitch = false
				}
			} else if cursor.oplHasEndedPitch && (cmd.EffectCmd == 0x01 || cmd.EffectCmd == 0x02) {
				direction, param := 1, mem.LastPortaUp
				if cmd.EffectCmd == 0x02 {
					direction, param = -1, mem.LastPortaDown
				}
				if !itPitchSlideState(&cursor.oplEndedPitch, &w.Assets.Samples[cursor.oplEndedSample],
					w.Assets.ITCompatibility, param, direction, itPitchSlideTick(w, param)) {
					cursor.oplHasEndedPitch = false
				}
			}
			continue
		}

		pitch := &w.VoicePitches[vID]
		if volumeSlide && !itVolumePitchSlideState(pitch, &w.Assets.Samples[w.VoiceSamples[vID].SampleID],
			w.Assets.ITCompatibility, volumeSlideParam, volumeSlideDirection, itPitchSlideTick(w, volumeSlideParam)) {
			w.stopVoice(vID)
			continue
		}

		// Reset temporary per-tick pitch offsets
		pitch.VibratoOffset = 0.0
		pitch.itVibratoDelta = 0
		if w.Assets.IT && cmd.HasVolume && cmd.Volume >= 203 && cmd.Volume <= 212 {
			itVibrato(w, vID, mem)
		}

		if volPorta {
			startTick := itNoteTriggerTick(cmd, mem, w.Assets.ITCompatibility.LegacyModPlug)
			if w.Clock.Tick >= startTick && startTick < w.Clock.Speed {
				if w.Assets.ITCompatibility.LegacyModPlug {
					initITTonePortamentoMemory(w, mem, itVolumePortamentoSpeed(cmd.Volume))
				}
				itTonePortamento(w, TrackID(t), vID, cmd, mem.LastPortaSpeed, startTick)
			}
		}

		switch cmd.EffectCmd {
		case 0x01: // Portamento Up (1xx)
			if w.Assets.IT {
				itPitchSlide(w, vID, mem.LastPortaUp, 1)
				break
			}
			if w.Assets.s3m {
				w.s3mPitchSlide(vID, mem.LastPortaUp, 1)
				break
			}
			if cmd.EffectArg != 0 {
				mem.LastPortaUp = cmd.EffectArg
			}
			if w.Clock.Tick > 0 {
				// Multiply frequency upward by fractional semitone step
				pitch.BaseFreqHz *= math.Pow(2.0, float64(mem.LastPortaUp)/(12.0*64.0))
			}

		case 0x02: // Portamento Down (2xx)
			if w.Assets.IT {
				itPitchSlide(w, vID, mem.LastPortaDown, -1)
				break
			}
			if w.Assets.s3m {
				w.s3mPitchSlide(vID, mem.LastPortaDown, -1)
				break
			}
			if cmd.EffectArg != 0 {
				mem.LastPortaDown = cmd.EffectArg
			}
			if w.Clock.Tick > 0 {
				pitch.BaseFreqHz /= math.Pow(2.0, float64(mem.LastPortaDown)/(12.0*64.0))
			}

		case 0x03, 0x1f: // Tone portamento, optionally with volume slide
			if w.Assets.IT {
				if volPorta && w.Assets.ITCompatibility.LegacyModPlug {
					initITTonePortamentoMemory(w, mem, cmd.EffectArg)
				}
				itTonePortamento(w, TrackID(t), vID, cmd, mem.LastPortaSpeed, 0)
				if cmd.EffectCmd == 0x03 && pitch.itVibratoDelta != 0 && w.VoiceLives[vID].Active {
					offset, ok := itVibratoOffset(
						*pitch,
						&w.Assets.Samples[w.VoiceSamples[vID].SampleID],
						w.Assets.ITCompatibility, pitch.itVibratoDelta,
					)

					if !ok {
						w.stopVoice(vID)
						break
					}
					pitch.VibratoOffset = offset
				}
				break
			}

			if cmd.EffectCmd == 0x03 && cmd.EffectArg != 0 {
				mem.LastPortaSpeed = cmd.EffectArg
			}
			// On Tick 0, if a note is present, set it as target without retriggering
			if w.Clock.Tick == 0 && cmd.Note >= 1 && cmd.Note <= 120 {
				pitch.PortaTargetHz = noteToHz(cmd.Note)
				if w.Assets.s3m {
					pitch.Note = cmd.Note
					speed := w.Assets.Samples[w.VoiceSamples[vID].SampleID].C5Speed
					if pitch.itFineTuneSpeed != 0 {
						speed = pitch.itFineTuneSpeed
					}
					pitch.PortaTargetHz = s3mNoteFrequency(cmd.Note, speed, w.Assets.s3mFrequencyPitch)
				}
			}
			if w.Clock.Tick > 0 && pitch.PortaTargetHz > 0 {
				if w.Assets.s3m {
					sample := &w.Assets.Samples[w.VoiceSamples[vID].SampleID]
					speed := sample.C5Speed
					if pitch.itFineTuneSpeed != 0 {
						speed = pitch.itFineTuneSpeed
					}
					if speed == 0 || pitch.BaseFreqHz <= 0 {
						break
					}
					if w.Assets.s3mFrequencyPitch {
						frequency := math.Round(pitch.BaseFreqHz * float64(speed) / 8363)
						target := math.Round(pitch.PortaTargetHz * float64(speed) / 8363)
						direction := 1.0
						if frequency > target {
							direction = -1
						}
						denominator := 1712*8363 - direction*float64(mem.LastPortaSpeed)*4*frequency
						next := float64(math.MaxInt32)
						if denominator > 0 {
							next = math.Min(math.Floor(1712*8363*frequency/denominator), math.MaxInt32)
						}
						if direction > 0 {
							next = math.Min(target, next)
						} else {
							next = math.Max(target, next)
						}
						pitch.BaseFreqHz = math.Max(1, next) * 8363 / float64(speed)
						break
					}
					period := 1712 * 8363 * 8363 / (pitch.BaseFreqHz * float64(speed))
					target := 1712 * 8363 * 8363 / (pitch.PortaTargetHz * float64(speed))
					if period > target {
						period = math.Max(target, period-float64(mem.LastPortaSpeed)*4)
					} else {
						period = math.Min(target, period+float64(mem.LastPortaSpeed)*4)
					}
					pitch.BaseFreqHz = 1712 * 8363 * 8363 / (period * float64(speed))
					break
				}
				factor := math.Pow(2.0, float64(mem.LastPortaSpeed)/(12.0*64.0))

				switch {
				case pitch.BaseFreqHz < pitch.PortaTargetHz:
					pitch.BaseFreqHz = math.Min(pitch.BaseFreqHz*factor, pitch.PortaTargetHz)
				case pitch.BaseFreqHz > pitch.PortaTargetHz:
					pitch.BaseFreqHz = math.Max(pitch.BaseFreqHz/factor, pitch.PortaTargetHz)
				}
			}

		case 0x04, 0x11, 0x21: // Vibrato, fine vibrato, or vibrato with volume slide
			if w.Assets.IT {
				itVibrato(w, vID, mem)
				break
			}
			if w.Assets.s3m {
				w.s3mVibrato(vID, mem, cmd)
				break
			}
			if cmd.EffectCmd == 0x04 && cmd.EffectArg&0xF0 != 0 {
				mem.LastVibrato = (cmd.EffectArg & 0xF0) | (mem.LastVibrato & 0x0F)
			}
			if cmd.EffectCmd == 0x04 && cmd.EffectArg&0x0F != 0 {
				mem.LastVibrato = (mem.LastVibrato & 0xF0) | (cmd.EffectArg & 0x0F)
			}

			speed := float64(mem.LastVibrato >> 4)
			depth := float64(mem.LastVibrato & 0x0F)

			if w.Clock.Tick > 0 {
				// Standard tracker sine LFO: 64 ticks per full cycle at speed 1
				pitch.VibratoPhase += (2.0 * math.Pi * speed) / 64.0
				if pitch.VibratoPhase >= 2.0*math.Pi {
					pitch.VibratoPhase -= 2.0 * math.Pi
				}
			}
			// Depth modulates pitch by up to +/- (depth / 16) semitones
			semitoneShift := math.Sin(pitch.VibratoPhase) * (depth / 16.0)
			pitch.VibratoOffset = pitch.BaseFreqHz * (math.Pow(2.0, semitoneShift/12.0) - 1.0)

		case 0x10: // Arpeggio
			if cmd.EffectArg != 0 {
				mem.LastArpeggio = cmd.EffectArg
			}
			shift := 0
			switch w.Clock.Tick % 3 {
			case 1:
				shift = int(mem.LastArpeggio >> 4)
			case 2:
				shift = int(mem.LastArpeggio & 15)
			}
			if w.Assets.IT {
				itArpeggio(w, TrackID(t), vID, shift)
				break
			}
			if w.Assets.s3m && (shift != 0 || w.Assets.s3mSharedEffectMemory) {
				speed := w.Assets.Samples[w.VoiceSamples[vID].SampleID].C5Speed
				if pitch.itFineTuneSpeed != 0 {
					speed = pitch.itFineTuneSpeed
				}
				note := min(int(pitch.Note)+shift, 119)
				pitch.VibratoOffset = s3mNoteFrequency(uint8(note), speed, w.Assets.s3mFrequencyPitch) - pitch.BaseFreqHz
				break
			}
			pitch.VibratoOffset = pitch.BaseFreqHz * (math.Pow(2, float64(shift)/12) - 1)

		case 0x13: // Retrigger
			if cmd.EffectArg != 0 {
				mem.LastRetrig = cmd.EffectArg
			}

			interval := int(mem.LastRetrig & 15)
			if interval == 0 {
				interval = 1
			}

			if mem.RetrigCount > 0 && mem.RetrigCount%interval == 0 {
				if channel := w.VoiceSamples[vID].oplChannel; channel != 0 {
					w.opl.noteOff(channel - 1)
				}
				w.VoiceSamples[vID].Position = 0
				w.VoiceSamples[vID].Reverse = false
				w.VoiceSamples[vID].mptLooped = false
				amp := &w.VoiceAmps[vID]
				volume := amp.Volume * 64
				switch mem.LastRetrig >> 4 {
				case 1, 2, 3, 4, 5:
					volume -= float32(uint32(1) << ((mem.LastRetrig >> 4) - 1))
				case 6:
					volume *= 2.0 / 3.0
				case 7:
					volume *= 0.5
				case 9, 10, 11, 12, 13:
					volume += float32(uint32(1) << ((mem.LastRetrig >> 4) - 9))
				case 14:
					volume *= 1.5
				case 15:
					volume *= 2
				}
				amp.Volume = clamp32(volume/64, 0, 1)
			}
			mem.RetrigCount++
		}
	}
}

func (w *Tracker) initMPTDoublePortamentoMemory(mem *EffectMemory, cmd ModCommand, volPorta, volumeSlide bool) {
	if cmd.EffectCmd == 0x03 || cmd.EffectCmd == 0x1f {
		speed := cmd.EffectArg
		if cmd.EffectCmd == 0x1f {
			speed = 0
		}
		initITTonePortamentoMemory(w, mem, speed)
	}
	if volPorta {
		initITTonePortamentoMemory(w, mem, itVolumePortamentoSpeed(cmd.Volume))
	}
	if volumeSlide {
		base := uint8(115)
		if cmd.Volume < 115 {
			base = 105
		}
		param := (cmd.Volume - base) * 4
		if param != 0 {
			mem.LastPortaUp, mem.LastPortaDown = param, param
			if w.Assets.ITCompatibility.itSharesPortamentoMemory() &&
				cmd.EffectCmd != 0x03 && cmd.EffectCmd != 0x1f {
				mem.LastPortaSpeed = param
			}
		}
	}
	if (cmd.EffectCmd == 0x01 || cmd.EffectCmd == 0x02) && cmd.EffectArg != 0 {
		mem.LastPortaUp, mem.LastPortaDown = cmd.EffectArg, cmd.EffectArg
		if w.Assets.ITCompatibility.itSharesPortamentoMemory() {
			mem.LastPortaSpeed = cmd.EffectArg
		}
	}
}

// System 4B: Evaluates volume slides, direct volume sets, and panning commands.
func VolumeEffectSystem(w *Tracker) {
	for t := 0; t < w.Assets.NumTracks; t++ {
		w.applyTrackVolumeEffects(TrackID(t), w.PendingCmds[t])
	}
}

func (w *Tracker) applyTrackVolumeEffects(t TrackID, cmd ModCommand) {
	vID := w.TrackCursors[t].ActiveVoice
	triggerTick := 0

	if w.Assets.s3m {
		switch cmd.EffectCmd {
		case 0x16:
			if w.Clock.Tick == 0 && cmd.EffectArg <= 64 {
				w.ITChannelVolume[t] = cmd.EffectArg
			}
		case 0x1e:
			applyITChannelVolumeSlide(w, t, cmd.EffectArg)
		case 0x15:
			if w.Clock.Tick == 0 {
				switch {
				case cmd.EffectArg&0xf0 == 0x10:
					w.TrackMemories[t].itGlissando = cmd.EffectArg&15 != 0
				case cmd.EffectArg&0xf0 == 0x20 && vID != -1 && w.VoiceLives[vID].Active:
					pitch := &w.VoicePitches[vID]
					pitch.itFineTuneSpeed = uint32(mptFineTuneSpeeds[cmd.EffectArg&15])
					pitch.BaseFreqHz = s3mNoteFrequency(pitch.Note, pitch.itFineTuneSpeed, w.Assets.s3mFrequencyPitch)
				case cmd.EffectArg&0xf0 == 0x80:
					setTrackPanning(w, t, vID, float32(cmd.EffectArg&15)*2/15-1)
				case cmd.EffectArg == 0x90 || cmd.EffectArg == 0x91:
					setITSurround(w, t, vID, cmd.EffectArg == 0x91)
				case cmd.EffectArg&0xf0 == 0xa0:
					w.TrackMemories[t].ITHighOffset = cmd.EffectArg & 15
				}
			}
		case 0x17:
			if w.Clock.Tick == 0 && cmd.EffectArg <= 64 {
				w.ITGlobalVolume = cmd.EffectArg * 2
			}
		case 0x1c:
			mem := &w.TrackMemories[t].itGlobalVolSlide
			if cmd.EffectArg != 0 {
				*mem = cmd.EffectArg
			}
			delta := itVolumeSlideDelta(*mem, w.itRepeatedRowTick()) * 2
			w.ITGlobalVolume = uint8(max(0, min(128, int(w.ITGlobalVolume)+delta)))
		case 0x1d:
			applyITPanningSlide(w, t, vID, cmd.EffectArg)
		}
	}

	if w.Assets.IT {
		w.applyITTremor(t, vID, cmd)
	}
	if w.Assets.s3m {
		w.applyS3MTremor(t, vID, cmd)
		w.applyITTremolo(t, vID, cmd)
	}
	if w.Assets.IT {
		w.applyITTremolo(t, vID, cmd)
		triggerTick = itNoteTriggerTick(cmd, &w.TrackMemories[t], w.Assets.ITCompatibility.LegacyModPlug)
	}

	if w.Assets.IT && cmd.EffectCmd == 0x16 && w.Clock.Tick == 0 && cmd.EffectArg <= 64 {
		w.ITChannelVolume[t] = cmd.EffectArg
	}

	if w.Assets.IT && cmd.EffectCmd == 0x17 && w.Clock.Tick == 0 {
		if cmd.EffectArg <= 128 || w.Assets.ITCompatibility.LegacyModPlug {
			w.ITGlobalVolume = min(cmd.EffectArg, 128)
		}
	}

	if w.Assets.IT && cmd.EffectCmd == 0x1c {
		applyITGlobalVolumeSlide(w, t, cmd.EffectArg)
	}

	if w.Assets.IT && cmd.EffectCmd == 0x1e {
		previous := w.ITChannelVolume[t]
		applyITChannelVolumeSlide(w, t, cmd.EffectArg)
		if previous != w.ITChannelVolume[t] && vID != -1 && w.VoiceLives[vID].Active {
			w.VoiceAmps[vID].itSmoothRamp = true
		}
	}

	if w.Assets.IT && cmd.EffectCmd == 0x1d {
		applyITPanningSlide(w, t, vID, cmd.EffectArg)
	}

	if w.Assets.IT && cmd.HasVolume && cmd.Volume >= 65 && cmd.Volume <= 84 &&
		w.Clock.Tick == triggerTick && triggerTick < w.Clock.Speed {
		applyITFineVolumeSlide(w, t, vID, cmd.Volume)
	}

	w.applyITVolumeColumnSlide(t, vID, cmd, triggerTick)

	if w.mptRetriggerAfterVolume(cmd) {
		applyITRetrigger(w, t, vID, cmd)
	}

	if w.Assets.IT && cmd.EffectCmd == 0x15 {
		w.applyITSpecialVolumeEffect(t, vID, cmd)
		return
	}
	if w.Assets.IT && cmd.EffectCmd == 0x08 && w.Clock.Tick == 0 {
		setTrackPanning(w, t, vID, float32(cmd.EffectArg)/128-1)
		return
	}
	if w.Assets.IT && (cmd.EffectCmd == 0x0a || cmd.EffectCmd == 0x11 || cmd.EffectCmd == 0x1f) {
		applyITEffectVolumeSlide(w, t, vID, cmd.EffectArg)
		return
	}
	w.applyBasicVolumeEffect(t, vID, cmd)
}

func (w *Tracker) applyITVolumeColumnSlide(t TrackID, vID VoiceID, cmd ModCommand, triggerTick int) {
	if !w.Assets.IT || w.Clock.Tick < triggerTick || triggerTick >= w.Clock.Speed ||
		!cmd.HasVolume || cmd.Volume < 85 || cmd.Volume > 104 {
		return
	}
	mem := &w.TrackMemories[t]
	up := cmd.Volume < 95
	base := uint8(95)
	if up {
		base = 85
	}
	slide := cmd.Volume - base
	if slide == 0 && w.Assets.ITCompatibility.mptSharedVolumeColumnMemory {
		slide = mem.itSharedVolParam
	}
	if slide != 0 {
		mem.ITVolSlide = slide
	} else {
		slide = mem.ITVolSlide
	}
	if w.Assets.ITCompatibility.mptVolumeSlidePropagation && !w.Assets.ITCompatibility.LegacyModPlug {
		if slide == 0 && !w.Assets.ITCompatibility.mptSharedVolumeColumnMemory {
			return
		}
		param := slide
		if up {
			param <<= 4
		}
		tick := w.itRepeatedRowTick()
		if w.Clock.Tick == triggerTick {
			tick = 0
		}
		applyITEffectVolumeSlideAtTick(w, t, vID, param, tick)
		return
	}
	if w.Assets.ITCompatibility.LegacyModPlug {
		if slide != 0 {
			if up {
				mem.LastVolSlide = slide << 4
			} else {
				mem.LastVolSlide = slide
			}
		} else {
			up = mem.LastVolSlide&0xf0 != 0
			if up {
				slide = mem.LastVolSlide >> 4
			} else {
				slide = mem.LastVolSlide & 0x0f
			}
		}
	}
	slideTick := w.Clock.Tick > triggerTick || w.Assets.ITCompatibility.LegacyModPlug && triggerTick > 0 && w.Clock.Tick == triggerTick
	if slideTick && vID != -1 && w.VoiceLives[vID].Active {
		amp := &w.VoiceAmps[vID]
		delta := float32(slide) / 64
		if !up {
			delta = -delta
		}
		amp.Volume = clamp32(amp.Volume+delta, 0, 1)
		amp.itSmoothRamp = true
	}
}

func (w *Tracker) applyITSpecialVolumeEffect(t TrackID, vID VoiceID, cmd ModCommand) {
	arg := cmd.EffectArg
	if arg&0xf0 == 0xa0 && w.Assets.ITCompatibility.HighOffsetUnsupported {
		return
	}
	if arg != 0 && w.Clock.Tick == 0 {
		w.TrackMemories[t].ITLastSpecial = arg
	} else if arg == 0 {
		arg = w.TrackMemories[t].ITLastSpecial
	}
	w.applyITNoteCut(t, vID, arg, cmd)
	if w.Clock.Tick != 0 {
		return
	}
	if arg&0xf0 == 0xf0 && (w.Assets.ITCompatibility.mptMixProfile != nil || w.Assets.ITCompatibility.mptMacroConfig != nil) {
		w.TrackMemories[t].mptActiveMacro = arg & 15
	}
	if arg&0xf0 == 0x10 {
		w.TrackMemories[t].itGlissando = arg&15 != 0
	} else if arg&0xf0 == 0x20 && vID != -1 && w.VoiceLives[vID].Active {
		w.applyITFineTune(t, vID, cmd, arg)
	} else if arg >= 0x70 && arg <= 0x7c {
		w.applyITInstrumentControl(t, vID, arg)
	} else if arg&0xf0 == 0x80 {
		pan := (int(arg&15)*256 + 8) / 15
		surround := w.TrackCursors[t].Surround
		setTrackPanning(w, t, vID, float32(pan)/128-1)
		if surround && w.Assets.ITCompatibility.LegacyModPlug {
			w.TrackCursors[t].Surround = true
			if vID != -1 && w.VoiceLives[vID].Active {
				w.VoiceAmps[vID].Surround = true
			}
		}
	} else if arg == 0x90 || arg == 0x91 {
		setITSurround(w, t, vID, arg == 0x91)
	} else if arg == 0x9f && vID != -1 && w.VoiceLives[vID].Active {
		w.applyITReversePlayback(vID, cmd)
	} else if arg&0xf0 == 0xa0 {
		offset := arg & 0x0f
		w.TrackMemories[t].ITHighOffset = offset
		if w.Assets.ITCompatibility.LegacyModPlug && cmd.Note >= 1 && cmd.Note <= 120 && vID != -1 && w.VoiceLives[vID].Active {
			pb := &w.VoiceSamples[vID]
			sample := &w.Assets.Samples[pb.SampleID]
			end := len(sample.Data)
			if sample.LoopEnable {
				end = min(end, int(sample.LoopEnd))
			}
			if pos := int(offset) << 16; pos < end {
				pb.Position = float64(pos)
				pb.mptLooped = false
			}
		}
	}
}

func (w *Tracker) applyITFineTune(t TrackID, vID VoiceID, cmd ModCommand, arg uint8) {
	pitch := &w.VoicePitches[vID]
	sample := &w.Assets.Samples[w.VoiceSamples[vID].SampleID]
	if pitch.itPortamento && w.TrackMemories[t].LastNote != 0 {
		note := w.TrackMemories[t].LastNote
		if _, mapped, _, ok := resolveSampleNote(&w.Assets, w.TrackMemories[t].LastInstrument, note); ok {
			pitch.Note = mapped
			pitch.itRealNote = note
		}
	}
	if itHasVolumeTonePortamento(cmd) && cmd.Note >= 1 && cmd.Note <= 120 {
		if _, note, _, ok := resolveSampleNote(&w.Assets, w.TrackMemories[t].LastInstrument, cmd.Note); ok {
			pitch.Note = note
			pitch.itRealNote = cmd.Note
		}
	}
	note := pitch.Note
	if w.Assets.ITCompatibility.mptRealNoteMapping && pitch.itRealNote != 0 {
		note = pitch.itRealNote
	}
	if w.Assets.ITCompatibility.LegacyModPlug && w.Assets.ITCompatibility.LinearSlides && pitch.PortaTargetHz != 0 {
		oldSpeed := sample.C5Speed
		if pitch.itFineTuneSpeed != 0 {
			oldSpeed = pitch.itFineTuneSpeed
		}
		pitch.PortaTargetHz *= float64(mptFineTuneSpeeds[arg&15]) / float64(oldSpeed)
	}
	pitch.BaseFreqHz = noteToHz(note-1) * float64(mptFineTuneSpeeds[arg&15]) / float64(sample.C5Speed)
	pitch.ITSlidePitch = 0
	pitch.itFineTuneSpeed = uint32(mptFineTuneSpeeds[arg&15])
	if w.Assets.ITCompatibility.LegacyModPlug {
		tunedSample := *sample
		tunedSample.C5Speed = pitch.itFineTuneSpeed
		pitch.BaseFreqHz, pitch.ITSlidePitch = itLegacyNotePitch(note-1, &tunedSample, w.Assets.ITCompatibility.LinearSlides)
		pitch.BaseFreqHz *= float64(tunedSample.C5Speed) / float64(sample.C5Speed)
	}
}

func (w *Tracker) applyITReversePlayback(vID VoiceID, cmd ModCommand) {
	const fractionUnit = 1.0 / 4294967296.0
	pb := &w.VoiceSamples[vID]
	sample := &w.Assets.Samples[pb.SampleID]
	end := len(sample.Data)
	loop := sample.LoopEnable
	if sample.mptSustainLoop && !w.VoiceLives[vID].KeyOff && !w.VoiceLives[vID].mptSustainSuppressed {
		loop = true
		end = min(end, int(sample.mptSustainEnd))
	} else if loop {
		end = min(end, int(sample.LoopEnd))
	}
	if pb.Position >= 0 && pb.Position < fractionUnit && end > 0 && (cmd.Note >= 1 && cmd.Note <= 120 || !loop) {
		pb.Position = float64(end) - fractionUnit
	}
	pb.Reverse = true
}

func (w *Tracker) applyBasicVolumeEffect(t TrackID, vID VoiceID, cmd ModCommand) {
	if vID == -1 || !w.VoiceLives[vID].Active {
		return
	}

	mem := &w.TrackMemories[t]
	amp := &w.VoiceAmps[vID]

	switch cmd.EffectCmd {
	case 0x15:
		if w.Assets.s3m && cmd.EffectArg&0xf0 == 0xc0 &&
			cmd.EffectArg&15 != 0 && w.Clock.Tick == int(cmd.EffectArg&15) {
			amp.Volume = 0
		}

	case 0x08: // S3M X80 is full right.
		if w.Clock.Tick == 0 {
			setTrackPanning(w, t, vID, float32(min(cmd.EffectArg, 128))/64-1)
		}

	case 0x0A, 0x11, 0x1f: // Volume slide, optionally with vibrato or tone portamento
		if cmd.EffectArg != 0 {
			mem.LastVolSlide = cmd.EffectArg
		}
		up, down := mem.LastVolSlide>>4, mem.LastVolSlide&15
		tick := w.Clock.Tick
		if w.Assets.s3m {
			tick = w.itRepeatedRowTick()
		}
		delta := 0
		switch {
		case down == 15 && up > 0:
			if tick == 0 {
				delta = int(up)
			}
		case up == 15 && down > 0:
			if tick == 0 {
				delta = -int(down)
			}
		case tick > 0 && down > 0:
			delta = -int(down)
		case tick > 0:
			delta = int(up)
		}
		amp.Volume = clamp32(amp.Volume+float32(delta)/64, 0, 1)

	case 0x0C: // Set Volume (Cxx: 00..40)
		if w.Clock.Tick == 0 {
			vol := float32(cmd.EffectArg) / 64.0
			amp.Volume = clamp32(vol, 0.0, 1.0)
		}
	}
}

func setTrackPanning(w *Tracker, track TrackID, voice VoiceID, panning float32) {
	w.TrackCursors[track].itHasRestorePan = false
	w.TrackCursors[track].Panning = panning
	if w.Assets.IT || w.Assets.s3m {
		w.TrackCursors[track].Surround = false
		if w.Assets.IT && w.panbrelloProfile().panOverride {
			w.TrackMemories[track].itPanbrelloOffset = 0
		}
	}
	if voice == -1 || !w.VoiceLives[voice].Active {
		return
	}
	amp := &w.VoiceAmps[voice]
	amp.Panning = panning
	if w.Assets.IT || w.Assets.s3m {
		amp.Surround = false
	}
	if w.Assets.IT && !w.Assets.ITCompatibility.LegacyModPlug && !w.Assets.ITCompatibility.mptLegacySwing {
		amp.ITPanSwing = 0
	}
}

func setITSurround(w *Tracker, track TrackID, voice VoiceID, enabled bool) {
	cursor := &w.TrackCursors[track]
	cursor.Surround = enabled
	if enabled {
		cursor.Panning = 0
	}
	if voice != -1 && w.VoiceLives[voice].Active {
		amp := &w.VoiceAmps[voice]
		amp.Surround = enabled
		amp.itSmoothRamp = true
		if enabled {
			amp.Panning = 0
		}
	}
}

// System 4C: Updates voice release/fadeout lifecycles across all active voices.
func VoiceLifecycleSystem(w *Tracker) {
	for t := 0; t < w.Assets.NumTracks; t++ {
		cursor := &w.TrackCursors[t]
		if cursor.oplHasEndedPitch && cursor.oplEndedTicks > 0 {
			cursor.oplEndedTicks--
			if cursor.oplEndedTicks == 0 {
				cursor.oplHasEndedPitch = false
			}
		}
	}

	for v := range MaxVoices {
		if !w.VoiceLives[v].Active {
			continue
		}
		if w.VoiceLives[v].ITInstrument != 0 {
			processITInstrument(w, v)
			continue
		}
		if w.VoiceSamples[v].oplChannel != 0 {
			continue
		}
		if w.VoiceLives[v].KeyOff {
			w.VoiceLives[v].FadeOutVol -= 0.05
			if w.VoiceLives[v].FadeOutVol <= 0.0 {
				w.stopVoice(VoiceID(v))
			}
		}
	}
}

// System 4D: Translates final musical pitch & amplitude into DSP step/gain values.
func VoiceKinematicsSystem(w *Tracker) {
	if w.Assets.IT {
		w.updateITPanbrello()
	}
	if w.Assets.s3m {
		w.updateS3MPanbrello()
	}
	for v := range MaxVoices {
		if !w.VoiceLives[v].Active {
			continue
		}

		sampID := w.VoiceSamples[v].SampleID
		if sampID < 0 || sampID >= len(w.Assets.Samples) {
			w.stopVoice(VoiceID(v))
			continue
		}

		// 1. Combine base pitch + vibrato LFO offset -> Resampler StepIncrement
		pitch := &w.VoicePitches[v]
		baseHz := pitch.BaseFreqHz
		vibratoOffset := pitch.VibratoOffset
		life := &w.VoiceLives[v]
		if w.Assets.s3m && w.TrackMemories[life.ParentTrack].itGlissando &&
			pitch.PortaTargetHz > 0 && baseHz > 0 {
			speed := w.Assets.Samples[sampID].C5Speed
			if pitch.itFineTuneSpeed != 0 {
				speed = pitch.itFineTuneSpeed
			}
			baseHz = s3mGlissandoFrequency(baseHz, speed, w.Assets.s3mFrequencyPitch)
		}
		if w.Assets.IT && !life.ITPastNote && w.TrackCursors[life.ParentTrack].ActiveVoice == VoiceID(v) &&
			w.TrackMemories[life.ParentTrack].itGlissando && pitch.itPortamento {
			baseHz = itGlissandoPitch(pitch, &w.Assets.Samples[sampID], w.Assets.ITCompatibility)
			if pitch.itVibratoDelta != 0 {
				temp := *pitch
				temp.BaseFreqHz, temp.ITSlidePitch = baseHz, 0
				offset, ok := itVibratoOffset(temp, &w.Assets.Samples[sampID], w.Assets.ITCompatibility, pitch.itVibratoDelta)
				if !ok {
					w.stopVoice(VoiceID(v))
					continue
				}
				vibratoOffset = offset
			}
		}
		effectiveHz := baseHz + vibratoOffset
		if life := &w.VoiceLives[v]; life.ITInstrument != 0 && !w.Assets.ITInstruments[life.ITInstrument-1].FilterEnvelope {
			effectiveHz *= math.Pow(2, float64(life.ITPitchEnvelopeValue)/24)
		}
		if w.Assets.IT {
			effectiveHz *= itSampleVibrato(w, pitch, &w.Assets.Samples[sampID])
		}
		if pitch.mptMicroTuning != 0 {
			depth := 1
			if life.ITInstrument != 0 {
				depth = int(w.Assets.ITInstruments[life.ITInstrument-1].mptPitchWheelDepth)
			}
			effectiveHz *= math.Pow(2, float64(int(pitch.mptMicroTuning)*depth)/(12*256*128))
		}
		c5Ratio := float64(w.Assets.Samples[sampID].C5Speed) / 8363.0
		if w.Assets.s3m && pitch.itFineTuneSpeed != 0 {
			c5Ratio = float64(pitch.itFineTuneSpeed) / 8363
		}
		pitch.StepIncrement = (effectiveHz * c5Ratio) / w.Clock.SampleRate

		// 2. Combine channel volume + NNA fadeout + panning -> Stereo Target Gains
		amp := &w.VoiceAmps[v]
		if amp.mptApplyLegacySwing {
			amp.Volume = clamp32(amp.Volume+float32(amp.ITVolumeSwing)/256, 0, 1)
			amp.Panning = clamp32(amp.Panning+float32(amp.ITPanSwing)/128, -1, 1)
			life := &w.VoiceLives[v]
			cursor := &w.TrackCursors[life.ParentTrack]
			if !life.ITPastNote && cursor.ActiveVoice == VoiceID(v) {
				cursor.Panning = amp.Panning
			}
			amp.ITVolumeSwing, amp.ITPanSwing = 0, 0
			amp.mptApplyLegacySwing = false
		}
		totalVol := voiceVolume(w, v, sampID)

		panning := clamp32(amp.Panning+float32(amp.ITPanSwing)/128, -1, 1)
		if life := &w.VoiceLives[v]; life.ITInstrument != 0 {
			panning = clamp32(panning+life.ITPanEnvelopeValue/32*(1-float32(math.Abs(float64(panning)))), -1, 1)
		}
		if life := &w.VoiceLives[v]; w.Assets.ITCompatibility.LegacyModPlug && life.ITInstrument != 0 {
			panning = itPitchPan(panning, pitch.Note, &w.Assets.ITInstruments[life.ITInstrument-1], true)
		}
		panning = clamp32(panning+amp.itPanbrelloOffset, -1, 1)
		if amp.Surround && w.Assets.IT && !w.Assets.ITCompatibility.LegacyModPlug {
			panning = 0
		}
		amp.TargetLeft = totalVol * (1.0 - panning) * 0.5
		amp.TargetRight = totalVol * (1.0 + panning) * 0.5
		if profile := w.Assets.ITCompatibility.mptMixProfile; profile != nil && w.VoiceSamples[v].oplChannel == 0 {
			if profile.softPanning {
				amp.TargetLeft = totalVol * min(1, 1-panning)
				amp.TargetRight = totalVol * min(1, 1+panning)
			}
			amp.TargetLeft *= profile.pcmGain
			amp.TargetRight *= profile.pcmGain
		}
		if amp.Surround {
			amp.TargetRight = -amp.TargetRight
		}
		amp.updatePCMRampDuration(w.Clock.SampleRate, w.Clock.SamplesPerTick, w.Assets.IT)
		if w.Assets.IT && w.Assets.ITCompatibility.mptMixProfile == nil && !life.ITPastNote &&
			w.TrackCursors[life.ParentTrack].ActiveVoice == VoiceID(v) && w.Clock.Tick == 0 {
			cmd := w.PendingCmds[life.ParentTrack]
			if itFadeDirectVolume(cmd) {
				amp.rampDuration = pcmGainRampFrames(w.Clock.SampleRate)
			}
		}
		amp.itSmoothRamp = false
		if channel := w.VoiceSamples[v].oplChannel; channel != 0 {
			if w.VoiceLives[v].itOPLLegacyFade {
				continue
			}
			w.opl.setVolume(channel-1, uint8(math.Round(float64(clamp32(totalVol, 0, 1)*63))))
			w.opl.setPanning(channel-1, int(math.Round(float64((panning+1)*128))))
			w.opl.setFrequency(channel-1, effectiveHz*c5Ratio/32, !w.VoiceLives[v].KeyOff)
		}
	}
}

var mptFineTuneSpeeds = [...]uint16{
	7895, 7941, 7985, 8046, 8107, 8169, 8232, 8280,
	8363, 8413, 8463, 8529, 8581, 8651, 8723, 8757,
}

func voiceVolume(w *Tracker, voice, sampleID int) float32 {
	life, amp := &w.VoiceLives[voice], &w.VoiceAmps[voice]
	if !w.Assets.IT {
		if w.Assets.s3m {
			if amp.itTremorMuted {
				return 0
			}
			return clamp32(amp.Volume+amp.itTremoloOffset, 0, 1) * life.FadeOutVol *
				float32(w.ITGlobalVolume) / 128 * float32(w.ITChannelVolume[life.ParentTrack]) / 64 * w.Assets.s3mMixGain
		}
		return amp.Volume * life.FadeOutVol
	}
	volume := clamp32(amp.Volume+amp.itTremoloOffset, 0, 1)
	totalVol := volume * life.FadeOutVol
	if amp.itTremorMuted {
		totalVol = 0
	}
	track := life.ParentTrack
	channelVolume := w.ITChannelVolume[track]
	if life.ITPastNote {
		channelVolume = life.ITPastChannelVolume
	}
	globalVolume := float32(w.ITGlobalVolume) / 128
	if w.Assets.ITCompatibility.mptMixProfile != nil {
		globalVolume = 1
	}
	totalVol *= w.Assets.ITMixGain
	totalVol *= globalVolume
	totalVol *= float32(channelVolume) / 64
	if life.ITInstrument == 0 {
		return totalVol * (float32(w.Assets.Samples[sampleID].ITGlobalVolume) / 64)
	}
	insVol := amp.ITInstrumentVolume
	if w.Assets.ITCompatibility.LegacyModPlug {
		totalVol = clamp32(volume+float32(amp.ITVolumeSwing)/256, 0, 1) * life.FadeOutVol * w.Assets.ITMixGain * globalVolume * float32(channelVolume) / 64
		if amp.itTremorMuted {
			totalVol = 0
		}
	} else {
		insVol = clamp32(insVol+float32(amp.ITVolumeSwing)/64, 0, 1)
	}
	return totalVol * (life.ITEnvelopeVolume * insVol)
}

func itPitchPan(panning float32, note uint8, instrument *ITInstrument, legacy bool) float32 {
	separation := int(instrument.PitchPanSeparation)
	if legacy {
		// Old ModPlug versions applied one quarter of IT's separation depth.
		for range 2 {
			if separation < 0 {
				separation = (separation - 1) / 2
			} else {
				separation = (separation + 1) / 2
			}
		}
	}
	return itPitchPanDepth(panning, note, instrument.PitchPanCenter, separation)
}

func itPitchPanDepth(panning float32, note, center uint8, separation int) float32 {
	delta := (int(note) - 1 - int(center)) * separation / 2
	return clamp32(panning+float32(delta)/128, -1, 1)
}
