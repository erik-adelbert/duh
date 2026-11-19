// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

func (compat ITCompatibility) itCompatibleTremor() bool {
	return !compat.LegacyModPlug && !compat.mptLegacyTremor
}

func (w *Tracker) applyITTremor(track TrackID, voice VoiceID, cmd ModCommand) {
	active := voice != -1 && w.VoiceLives[voice].Active
	if active {
		w.VoiceAmps[voice].itTremorMuted = false
	}

	if cmd.EffectCmd != 0x22 {
		return
	}

	compatible := w.Assets.ITCompatibility.itCompatibleTremor()
	mem := &w.TrackMemories[track]
	if w.Clock.Tick == 0 {
		param := cmd.EffectArg
		if param != 0 {
			if compatible && !w.Assets.ITCompatibility.OldEffects {
				if param&0xf0 != 0 {
					param -= 0x10
				}
				if param&0x0f != 0 {
					param--
				}
			}
			mem.itTremorParam = param
		}

		if compatible {
			mem.itTremorCount |= 0x80
		}
	}

	if !active {
		return
	}

	if !compatible {
		on := mem.itTremorParam >> 4
		length := int(on) + int(mem.itTremorParam&15)
		if w.Assets.ITCompatibility.OldEffects {
			on++
			length += 2
		}
		count := int(mem.itTremorCount)
		if count >= length {
			count = 0
		}
		w.VoiceAmps[voice].itTremorMuted = count >= int(on)
		mem.itTremorCount = uint8(count + 1)
		return
	}

	count := mem.itTremorCount
	switch count {
	case 0x80:
		count = (mem.itTremorParam >> 4) | 0xc0
	case 0xc0:
		count = (mem.itTremorParam & 15) | 0x80
	default:
		count--
	}
	mem.itTremorCount = count
	w.VoiceAmps[voice].itTremorMuted = count&0xc0 == 0x80
}
