// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

// Only sample duplicate checks with cut are accepted by the instrument loader.
func projectMPTDuplicateSampleBehaviour(saved mptSavedPlayBehaviours, version uint32, strict bool) (bool, error) {
	if _, err := projectMPTBehaviours(saved, version, true, false, false); err != nil {
		return false, err
	}

	if version < 0x01290000 || version > 0x01290000 && version < 0x01290057 {
		strict = false
	}

	return !strict, nil
}
