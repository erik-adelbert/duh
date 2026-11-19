// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import "fmt"

// Reject pending-instrument and note-cut cases until their retained channel
// state can be represented faithfully.
func validateMPTPendingInstrumentAndNoteCut(assets *SongAssets, pluginRouting bool) error {
	ignoreEmpty := assets.ITCompatibility.mptEmptyNoteMapIgnoreCell && len(assets.ITInstruments) != 0
	ignoredCell, noteCut := false, false
	for pi, pattern := range assets.Patterns {
		for ri, row := range pattern.Rows {
			for ch, cmd := range row {
				noteCut = noteCut || assets.ITCompatibility.mptNoteCutWithPorta &&
					cmd.EffectCmd == 0x15 && cmd.EffectArg&0xf0 == 0xc0
				if !ignoreEmpty || cmd.Instrument == 0 {
					continue
				}
				if cmd.Note < 1 || cmd.Note > 120 || int(cmd.Instrument) > len(assets.ITInstruments) ||
					cmd.EffectCmd == 0x15 && (cmd.EffectArg == 0 || cmd.EffectArg&0xf0 == 0xd0) {
					return fmt.Errorf("MPTM pattern %d row %d channel %d: pending instrument with playback behaviour bit 123 is not supported", pi, ri, ch+1)
				}
				if assets.ITInstruments[cmd.Instrument-1].Samples[cmd.Note-1] == 0 {
					if pluginRouting {
						return fmt.Errorf("MPTM pattern %d row %d channel %d: empty note map with plug-in routing and playback behaviour bit 123 is not supported", pi, ri, ch+1)
					}
					ignoredCell = true
				}
			}
		}
	}
	if ignoredCell || noteCut {
		for pi, pattern := range assets.Patterns {
			for ri, row := range pattern.Rows {
				for ch, cmd := range row {
					if noteCut && (cmd.Note == 0 && cmd.Instrument != 0 || cmd.EffectCmd == 0x1a || cmd.EffectCmd == 0x1b) {
						return fmt.Errorf("MPTM pattern %d row %d channel %d: note cut with instrument-only or retrigger and playback behaviour bit 134 is not supported", pi, ri, ch+1)
					}
					if ignoredCell && cmd.Note >= 1 && cmd.Note <= 120 && cmd.Instrument == 0 {
						return fmt.Errorf("MPTM pattern %d row %d channel %d: instrumentless note after an empty note map with playback behaviour bit 123 is not supported", pi, ri, ch+1)
					}
				}
			}
		}
	}
	return nil
}
