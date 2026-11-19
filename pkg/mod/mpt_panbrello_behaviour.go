package mod

func projectMPTPanbrelloProfile(saved mptSavedPlayBehaviours, version uint32) (itPanbrelloProfile, error) {
	projection, err := projectMPTBehaviours(saved, version, true, false, false)
	if err != nil {
		return itPanbrelloProfile{}, err
	}

	return itPanbrelloProfile{
		preciseWaveform: projection.known.enabled(mptITVibratoTremoloPanbrello),
		hold:            projection.known.enabled(mptITPanbrelloHold),
		panOverride:     projection.known.enabled(mptPanOverride),
		sampleAndHold:   projection.known.enabled(mptITSampleAndHoldPanbrello),
	}, nil
}
