package mod

import "math"

// MPTCutoffMacroSystem handles parametered and fixed filter macros, including smooth slides.
func MPTCutoffMacroSystem(w *Tracker) {
	if !w.Assets.IT || w.Assets.ITCompatibility.mptMixProfile == nil && w.Assets.ITCompatibility.mptMacroConfig == nil {
		return
	}

	config := w.Assets.ITCompatibility.mptMacroConfig
	if config == nil {
		defaults := defaultMPTMacroConfig()
		config = &defaults
	}

	for t := range w.Assets.NumTracks {
		cmd := w.PendingCmds[t]

		smooth := cmd.EffectCmd == 0x26
		if !smooth && (cmd.EffectCmd != 0x19 || w.Clock.Tick != 0) {
			continue
		}

		cursor := &w.TrackCursors[t]
		voice := cursor.ActiveVoice
		activeVoice := voice != -1 && w.VoiceLives[voice].Active

		var macro mptMacro

		value := cmd.EffectArg
		if value >= 0x80 {
			macro = config.fixed[value-0x80]
			value = macro.value
		} else {
			macro = config.parametered[w.TrackMemories[t].mptActiveMacro]
		}

		resonance := macro.kind == mptMacroResonance
		if smooth && (macro.kind == mptMacroCutoff || resonance) {
			current := cursor.ITFilterCutoff
			if resonance {
				current = cursor.ITFilterResonance
			}
			// Modern filter swing is applied to the voice before the macro runs.
			if activeVoice && !w.Assets.ITCompatibility.mptOldSwingBehaviour {
				life := &w.VoiceLives[voice]
				if resonance {
					current = life.ITFilterResonance
				} else {
					current = life.ITFilterCutoff
				}
			}

			ticksLeft := w.mptRowTicks() - w.Clock.Tick
			if ticksLeft > 1 {
				step := (float32(value) - float32(current)) / float32(ticksLeft)
				value = uint8(math.Round(float64(float32(current) + step)))
			}
		}

		switch macro.kind {
		case mptMacroCutoff:
			cursor.ITFilterCutoff = value
		case mptMacroResonance:
			cursor.ITFilterResonance = value
		default:
			continue
		}

		if !activeVoice {
			continue
		}

		life := &w.VoiceLives[voice]

		if resonance {
			life.ITFilterResonance = mptApplyFilterSwing(cursor.ITFilterResonance, life.mptResonanceSwing)
			life.mptRestoreResonance = false
		} else {
			life.ITFilterCutoff = value
		}

		if !w.Assets.ITCompatibility.LegacyModPlug && life.ITFilterCutoff == 127 && life.ITFilterResonance == 0 {
			if w.Clock.Tick == 0 && cmd.Note >= 1 && cmd.Note <= 120 && !itHasTonePortamento(cmd) {
				w.VoiceFilters[voice] = ITFilterState{}
				w.VoiceRightFilters[voice] = ITFilterState{}
			}
			continue
		}

		filter := itFilterCoefficients(
			life.ITFilterCutoff, life.ITFilterResonance, w.Clock.SampleRate,
			w.Assets.ITCompatibility.LegacyModPlug,
			w.Assets.ITCompatibility.ExtendedFilter,
			256,
		)

		setITFilterMode(&filter, life.mptHighPass)

		left, right := &w.VoiceFilters[voice], &w.VoiceRightFilters[voice]
		if left.Enabled {
			filter.Y1, filter.Y2 = left.Y1, left.Y2
		}

		*left = filter
		filter.Y1, filter.Y2 = 0, 0
		if right.Enabled {
			filter.Y1, filter.Y2 = right.Y1, right.Y2
		}

		*right = filter
		if channel := w.VoiceSamples[voice].oplChannel; channel != 0 && !resonance {
			w.opl.setModulatorCutoff(channel-1, value/2)
		}
	}
}
