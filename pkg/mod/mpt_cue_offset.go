// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

func isMPTCueOffset(cmd ModCommand) bool {
	return cmd.HasVolume && cmd.Volume >= 224 && cmd.Volume <= 232
}
