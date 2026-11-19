package mod

import "fmt"

// The newer DCT behavior bit is outside the decoded prefix. Before 1.29,
// OpenMPT clears that bit and checks duplicate samples across instruments.
func projectMPTDuplicateSampleAcrossInstruments(saved mptSavedPlayBehaviours, version uint32) (bool, error) {
	if version < 0x01260000 || version >= 0x01290000 {
		return false, fmt.Errorf("unsupported MPTM saved version %#08x for duplicate-sample projection", version)
	}

	return projectMPTDuplicateSampleBehaviour(saved, version, false)
}

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
