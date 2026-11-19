// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import "fmt"

func (w *Tracker) mptRowTicks() int {
	return (w.Clock.Speed + w.mptFrameDelay) * (1 + w.mptRowDelay)
}

// The scheduler keeps an absolute tick so pattern rows and notes are fetched only once.
func (w *Tracker) itRepeatedRowTick() int {
	if w.mptRowDelay != 0 {
		return w.Clock.Tick % (w.Clock.Speed + w.mptFrameDelay)
	}
	return w.Clock.Tick
}

func validateITPatternDelays(patterns []PatternAsset, mptm, instrumentMode bool) error {
	format := "IT"
	if mptm {
		format = "MPTM"
	}

	var delayTrack, memoryTrack [MaxTracks]bool

	for pi, pattern := range patterns {
		for ri, row := range pattern.Rows {
			delayed := false

			for track, cmd := range row {
				if isMPTPatternDelay(cmd) {
					delayTrack[track] = true
					delayed = true
				}
				if cmd.EffectCmd == 0x15 && cmd.EffectArg == 0 {
					memoryTrack[track] = true
				}
			}

			if !delayed {
				continue
			}

			for track, cmd := range row {
				compatible := cmd.EffectCmd == 0 || isMPTPatternDelay(cmd)
				if mptm {
					compatible = mptDelayCompatibleEffect(cmd)
				}

				if !itDelayedNoteSupported(cmd, mptm, instrumentMode) || cmd.HasVolume || !compatible ||
					(cmd.Note != 0 || cmd.Instrument != 0) && cmd.EffectCmd != 0 && !isMPTPatternDelay(cmd) {
					return fmt.Errorf("%s pattern %d row %d channel %d: pattern delay with other row commands is not supported", format, pi, ri, track+1)
				}
			}
		}
	}

	for track := range delayTrack {
		if delayTrack[track] && memoryTrack[track] {
			return fmt.Errorf("%s channel %d: pattern delay with S00 memory is not supported", format, track+1)
		}
	}

	return nil
}

func itDelayedNoteSupported(cmd ModCommand, mptm, instrumentMode bool) bool {
	if cmd.Note < 253 {
		return true
	}

	return !mptm && instrumentMode && cmd.Note == 255 && cmd.Instrument == 0
}

func isMPTPatternDelay(cmd ModCommand) bool {
	return cmd.EffectCmd == 0x15 && (cmd.EffectArg&0xf0 == 0x60 || cmd.EffectArg&0xf0 == 0xe0)
}

func mptDelayCompatibleEffect(cmd ModCommand) bool {
	switch cmd.EffectCmd {
	case 0, 0x01, 0x02, 0x03, 0x04, 0x0a, 0x11, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20, 0x21, 0x24:
		return true
	case 0x15:
		switch cmd.EffectArg & 0xf0 {
		case 0x10, 0x60, 0xe0:
			return true
		}
	}

	return false
}
