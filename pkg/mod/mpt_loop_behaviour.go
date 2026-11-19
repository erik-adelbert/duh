package mod

func projectMPTLoopProfile(saved mptSavedPlayBehaviours, version uint32, positionSensitive bool) (itPatternLoopProfile, error) {
	projection, err := projectMPTBehaviours(saved, version, true, false, false)
	if err != nil {
		return itPatternLoopProfile{}, err
	}

	flags := projection.known
	profile := itPatternLoopProfile{
		resetTarget:     flags.enabled(mptITPatternLoopTargetReset),
		allowNested:     flags.enabled(mptITFT2PatternLoop),
		preserveOnBreak: flags.enabled(mptITPatternLoopBreak),
	}

	switch {
	case positionSensitive && mptBehaviourAvailable(version, 0x01290032):
		profile.jumpPriority = itJumpRightWins
	case flags.enabled(mptITPatternLoopWithJumpsOld):
		profile.jumpPriority = itJumpAlwaysWins
	}

	return profile, nil
}
