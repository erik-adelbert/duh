// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import "errors"

func projectMPTClock(mode *uint8, rowsPerBeat uint16, swing []uint32) (uint16, error) {
	if mode == nil || *mode == 0 {
		if len(swing) != 0 {
			return 0, errors.New("MPTM song swing with classic tempo is not supported")
		}
		return 0, nil
	}

	return projectMPTModernRowsPerBeat(mode, rowsPerBeat)
}

func projectMPTModernRowsPerBeat(mode *uint8, rowsPerBeat uint16) (uint16, error) {
	if mode == nil || *mode != 2 {
		return 0, errors.New("MPTM modern tempo mode is not explicitly selected")
	}

	if rowsPerBeat == 0 {
		return 0, errors.New("invalid MPTM rows per beat")
	}

	return rowsPerBeat, nil
}

func mptSongTempoSwing(values []uint32, rowsPerBeat uint16) []uint32 {
	if len(values) == 0 {
		return nil
	}

	swing := append([]uint32(nil), values...)
	normalizeMPTTempoSwing(swing)
	return resizeMPTTempoSwing(swing, int(rowsPerBeat))
}
