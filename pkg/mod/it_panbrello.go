// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

type itPanbrelloProfile struct {
	preciseWaveform bool
	hold            bool
	panOverride     bool
	sampleAndHold   bool
}

func (w *Tracker) panbrelloProfile() itPanbrelloProfile {
	if profile := w.Assets.ITCompatibility.itPanbrelloProfile; profile != nil {
		return *profile
	}

	enabled := !w.Assets.ITCompatibility.LegacyModPlug

	return itPanbrelloProfile{preciseWaveform: enabled, hold: enabled, panOverride: true, sampleAndHold: enabled}
}

func (w *Tracker) updateITPanbrello() {
	profile := w.panbrelloProfile()

	for t := 0; t < w.Assets.NumTracks; t++ {
		mem := &w.TrackMemories[t]
		cmd := w.PendingCmds[t]

		if w.Clock.Tick == 0 && !profile.preciseWaveform {
			mem.itPanbrelloOffset = 0
		}

		if cmd.EffectCmd == 0x18 {
			if cmd.EffectArg&0xf0 != 0 {
				mem.itPanbrello = mem.itPanbrello&0x0f | cmd.EffectArg&0xf0
			}
			if cmd.EffectArg&0x0f != 0 {
				mem.itPanbrello = mem.itPanbrello&0xf0 | cmd.EffectArg&0x0f
			}
		}

		voice := w.TrackCursors[t].ActiveVoice
		if voice == -1 || !w.VoiceLives[voice].Active {
			continue
		}

		delta := int(mem.itPanbrelloOffset)
		if cmd.EffectCmd == 0x18 {
			pos := mem.itPanbrelloPos

			if !profile.preciseWaveform {
				pos = uint8(((int(pos) + 16) >> 2) & 63)
			}

			delta = itEffectWaveform(w, mem.itPanbrelloWaveform, pos, profile.preciseWaveform, !profile.preciseWaveform)

			if profile.sampleAndHold && mem.itPanbrelloWaveform == 3 {
				if mem.itPanbrelloPos == 0 || mem.itPanbrelloPos >= mem.itPanbrello>>4 {
					mem.itPanbrelloPos = 0
					mem.itPanbrelloRandom = int8(delta)
				}
				mem.itPanbrelloPos++
				delta = int(mem.itPanbrelloRandom)
			} else {
				mem.itPanbrelloPos += mem.itPanbrello >> 4
			}

			if profile.hold {
				mem.itPanbrelloOffset = int8(delta)
			}
		}

		offset := 0

		if delta != 0 {
			offset = (delta*int(mem.itPanbrello&15) + 2) / 8
		}

		w.VoiceAmps[voice].itPanbrelloOffset = float32(offset) / 128
	}
}
