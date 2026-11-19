// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import "fmt"

// mptBehaviourProjection covers only the named subset, not a playback profile.
// Upgrades to other behaviours and to patterns, instruments and samples are
// intentionally outside its scope.
type mptBehaviourProjection struct {
	known                   mptSavedPlayBehaviours
	oplNoResetAtEnvelopeEnd bool
}

// projectMPTBehaviours applies the relevant OpenMPT upgrades for files with
// explicitly serialized flags. Other version-dependent behaviours may differ.
func projectMPTBehaviours(saved mptSavedPlayBehaviours, version uint32, linearSlides, hasOPL, hasInstruments bool) (mptBehaviourProjection, error) {
	if uint64(saved)>>mptKnownPlayBehaviourCount != 0 {
		return mptBehaviourProjection{}, fmt.Errorf("unsupported MPTM saved behaviour bits %#x", uint64(saved)>>mptKnownPlayBehaviourCount)
	}

	result := mptBehaviourProjection{known: saved}

	if !linearSlides && version < 0x01300036 && version != 0x01300000 {
		result.known &^= 1 << mptPeriodsAreHertz
	}

	result.oplNoResetAtEnvelopeEnd = projectMPTOPLNoResetAtEnvelopeEnd(false, version, hasOPL, hasInstruments)

	return result, nil
}

func projectMPTOPLNoResetAtEnvelopeEnd(saved bool, version uint32, hasOPL, hasInstruments bool) bool {
	return saved || hasOPL && hasInstruments && version >= 0x01280020 && version <= 0x01290055
}
