package mod

import "fmt"

func unpackITPattern(data []byte, count int) (PatternAsset, error) {
	return unpackITPatternMode(data, count, false)
}

func unpackITPatternMode(data []byte, count int, mptm bool) (PatternAsset, error) {
	return unpackITPatternChannels(data, count, mptm, itHeaderChannels)
}

func projectMPTDuplicateSampleAcrossInstruments(saved mptSavedPlayBehaviours, version uint32) (bool, error) {
	if version < 0x01260000 || version >= 0x01290000 {
		return false, fmt.Errorf("unsupported MPTM saved version %#08x for duplicate-sample projection", version)
	}

	return projectMPTDuplicateSampleBehaviour(saved, version, false)
}

func parseMPTSavedPlayBehaviours(data []byte) (mptSavedPlayBehaviours, error) {
	flags, _, err := parseMPTPlayFlags(data, false)
	return flags, err
}

func applyMPTResampling(instruments []ITInstrument, modes []uint32) error {
	return applyMPTSongResampling(instruments, modes, nil)
}
