package mod

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

// LoadMPTM loads a limited, experimental subset of MPTM. Unsupported features
// are rejected; playback fidelity is incomplete. It selects sequence zero.
func LoadMPTM(path string) (*S3MLoadResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return loadMPTM(data)
}

func loadMPTM(data []byte) (*S3MLoadResult, error) {
	return loadMPTMSequence(data, 0)
}

func loadMPTMSequence(data []byte, sequence int) (*S3MLoadResult, error) {
	if sequence < 0 {
		return nil, fmt.Errorf("invalid MPTM sequence %d", sequence)
	}

	bounds, err := locateMPTExtensions(data)
	if err != nil {
		return nil, err
	}

	u16 := func(i int) int { return int(binary.LittleEndian.Uint16(data[i:])) }

	ext, err := parseMPTExtensions(data[bounds.propertiesStart:bounds.serializedStart], u16(34))
	if err != nil {
		return nil, err
	}
	if err := ext.validatePlaybackFields(u16(36)); err != nil {
		return nil, err
	}

	hasPluginRouting := false

	for _, plugin := range ext.instrumentPlugins {
		hasPluginRouting = hasPluginRouting || plugin != 0
	}

	if ext.lastSavedVersion == nil || ext.channels == nil || *ext.channels == 0 ||
		ext.mixLevels == nil || ext.pluginVolume == nil {
		return nil, errors.New("MPTM is missing required extension properties")
	}

	if *ext.channels > MaxTracks {
		return nil, fmt.Errorf("MPTM channel count %d exceeds supported limit %d", *ext.channels, MaxTracks)
	}

	channels := int(*ext.channels)
	if ext.extraChannelSettings != nil {
		channels = max(channels, itHeaderChannels+len(ext.extraChannelSettings)/2)
	}

	flags, additional, err := resolveMPTPlaybackProfileFlags(ext.playbackFlags, *ext.lastSavedVersion, u16(44)&8 != 0)
	if err != nil {
		return nil, err
	}

	for i := 0; i < u16(34); i++ {
		ptr := int(binary.LittleEndian.Uint32(data[192+u16(32)+4*i:]))
		if ptr != 0 {
			if additional.itPitchPanSeparation && !mptBehaviourAvailable(*ext.lastSavedVersion, 0x01300053) && data[ptr+22] != 0 {
				return nil, fmt.Errorf("MPTM instrument %d: pitch-pan separation with an unavailable playback behaviour is not supported", i+1)
			}

			channel := data[ptr+60]

			if channel >= 128 {
				hasPluginRouting = hasPluginRouting || channel > 128
			} else if channel != 0 {
				return nil, fmt.Errorf("MPTM instrument %d uses unsupported MIDI output", i+1)
			}
		}
	}

	loop, err := projectMPTLoopProfile(flags, *ext.lastSavedVersion, additional.itPatternLoopWithJumps)
	if err != nil {
		return nil, err
	}

	pan, err := projectMPTPanbrelloProfile(flags, *ext.lastSavedVersion)
	if err != nil {
		return nil, err
	}

	duplicate, err := projectMPTDuplicateSampleBehaviour(flags, *ext.lastSavedVersion, additional.itDuplicateCheck)
	if err != nil {
		return nil, err
	}

	samplePreamp := uint32(min(data[49], 128))
	if ext.samplePreamp != nil {
		samplePreamp = *ext.samplePreamp
	}

	mix, err := projectMPTMixProfile(*ext.mixLevels, samplePreamp, *ext.pluginVolume)
	if err != nil {
		return nil, err
	}

	rowsPerBeatHeader := uint16(4)
	if data[46]&4 != 0 {
		rowsPerBeatHeader = uint16(data[30])
	}

	if ext.rowsPerBeat != nil {
		if *ext.rowsPerBeat > 65535 {
			return nil, fmt.Errorf("unsupported MPTM rows per beat %d", *ext.rowsPerBeat)
		}
		rowsPerBeatHeader = uint16(*ext.rowsPerBeat)
	}

	if ext.rowsPerMeasure != nil && *ext.rowsPerMeasure > 65536 {
		return nil, fmt.Errorf("unsupported MPTM rows per measure %d", *ext.rowsPerMeasure)
	}

	rowsPerBeat, err := projectMPTClock(ext.tempoMode, rowsPerBeatHeader, ext.tempoSwing)
	if err != nil {
		return nil, err
	}

	sequences, err := parseMPTSequences(data[bounds.serializedStart:bounds.serializedEnd], u16(38))
	if err != nil {
		return nil, err
	}

	if sequence >= len(sequences.sequences) {
		return nil, fmt.Errorf("MPTM sequence %d out of range (count %d)", sequence, len(sequences.sequences))
	}

	seq := sequences.sequences[sequence]
	legacyDefaults, err := mptLegacySequenceDefaults(data[51], data[50], ext.defaultTempo, ext.defaultTempoFraction)
	if err != nil {
		return nil, err
	}

	if ext.legacyRestart != nil {
		legacyDefaults.restart = *ext.legacyRestart
	}

	defaults, err := resolveMPTSequenceDefaults(seq, legacyDefaults)
	if err != nil {
		return nil, err
	}

	loaded, err := loadITDataChannels(data, true, max(itHeaderChannels, channels))
	if err != nil {
		return nil, err
	}

	if additional.oplRealRetrig {
		for i, sample := range loaded.Assets.Samples {
			if sample.opl != nil {
				return nil, fmt.Errorf("MPTM sample %d: OPL retrigger behavior is not supported", i+1)
			}
		}
	}

	for i := range loaded.Assets.ITInstruments {
		loaded.Assets.ITInstruments[i].mptPitchWheelDepth = int8(data[53])
		if ext.instrumentMIDIPWD != nil {
			loaded.Assets.ITInstruments[i].mptPitchWheelDepth = ext.instrumentMIDIPWD[i]
		}
	}

	// OpenMPT builds loop lookahead before upgrading older playback flags.
	loaded.Assets.ITCompatibility.mptPingPongLookahead = flags.enabled(mptITPingPongMode)
	loaded.Assets.ITCompatibility.mptPingPongMode = loaded.Assets.ITCompatibility.mptPingPongLookahead
	if flags.enabled(mptCompatiblePlay) && *ext.lastSavedVersion < 0x01260000 {
		loaded.Assets.ITCompatibility.mptPingPongMode = *ext.lastSavedVersion >= 0x01190021 || *ext.lastSavedVersion == 0x01190000
	}
	loaded.Assets.ITCompatibility.mptImprecisePingPong = *ext.lastSavedVersion < 0x01300054

	if err := applyMPTVolumeReleaseNodes(loaded.Assets.ITInstruments, ext.instrumentVolumeReleaseNode, *ext.lastSavedVersion); err != nil {
		return nil, err
	}

	if *ext.lastSavedVersion < 0x01260000 {
		for i, sample := range loaded.Assets.Samples {
			if sample.mptSustainLoop {
				return nil, fmt.Errorf("MPTM sample %d: sustain loop with a pre-1.26 playback profile is not supported", i+1)
			}
		}
	}

	for i, instrument := range loaded.Assets.ITInstruments {
		if instrument.DuplicateCheck == 1 && instrument.DuplicateAction == 0 {
			if *ext.lastSavedVersion < 0x01260000 {
				return nil, fmt.Errorf("MPTM instrument %d: duplicate-note cut with a pre-1.26 playback profile is not supported", i+1)
			}

			ptr := int(binary.LittleEndian.Uint32(data[192+u16(32)+4*i:]))

			if i < len(ext.instrumentPlugins) && ext.instrumentPlugins[i] != 0 || data[ptr+60] > 128 {
				return nil, fmt.Errorf("MPTM instrument %d: duplicate-note cut with plugin routing is not supported", i+1)
			}
		}

		if instrument.DuplicateCheck != 3 || instrument.DuplicateAction != 1 {
			continue
		}

		if *ext.lastSavedVersion < 0x01260000 {
			return nil, fmt.Errorf("MPTM instrument %d: duplicate-instrument note-off with a pre-1.26 playback profile is not supported", i+1)
		}

		ptr := int(binary.LittleEndian.Uint32(data[192+u16(32)+4*i:]))

		if i < len(ext.instrumentPlugins) && ext.instrumentPlugins[i] != 0 || data[ptr+60] > 128 {
			return nil, fmt.Errorf("MPTM instrument %d: duplicate-instrument note-off with plugin routing is not supported", i+1)
		}
	}

	loaded.Assets.ITCompatibility.oplFlexibleNoteOff = additional.oplFlexibleNoteOff
	loaded.Assets.ITCompatibility.itRestoreChannelPan = additional.itRestoreChannelPan
	loaded.Assets.ITCompatibility.mptPitchPanSeparation = additional.itPitchPanSeparation &&
		mptBehaviourAvailable(*ext.lastSavedVersion, 0x01300053)
	loaded.Assets.ITCompatibility.mptResetFilterOnPorta = additional.itResetFilterOnPorta &&
		mptBehaviourAvailable(*ext.lastSavedVersion, 0x01300802)
	loaded.Assets.ITCompatibility.mptInitialNoteMemory = additional.itInitialNoteMemory &&
		mptBehaviourAvailable(*ext.lastSavedVersion, 0x01310025)
	loaded.Assets.ITCompatibility.mptNoSustainOnPortamento = additional.itNoSustainOnPortamento &&
		mptBehaviourAvailable(*ext.lastSavedVersion, 0x01320013)
	loaded.Assets.ITCompatibility.mptEmptyNoteMapIgnoreCell = additional.itEmptyNoteMapIgnoreCell &&
		mptBehaviourAvailable(*ext.lastSavedVersion, 0x01320013)
	loaded.Assets.ITCompatibility.mptOffsetWithInstrument = additional.itOffsetWithInstrument &&
		mptBehaviourAvailable(*ext.lastSavedVersion, 0x01320015)
	loaded.Assets.ITCompatibility.mptDoublePortamentoSlides = additional.itDoublePortamentoSlides &&
		mptBehaviourAvailable(*ext.lastSavedVersion, 0x01320027)
	loaded.Assets.ITCompatibility.mptCarryAfterNoteOff = additional.itCarryAfterNoteOff &&
		mptBehaviourAvailable(*ext.lastSavedVersion, 0x01320040)
	loaded.Assets.ITCompatibility.mptNoteCutWithPorta = additional.itNoteCutWithPorta &&
		mptBehaviourAvailable(*ext.lastSavedVersion, 0x01320102)
	loaded.Assets.ITCompatibility.mptStoppedFilterEnvAtStart = additional.itStoppedFilterEnvAtStart &&
		mptBehaviourAvailable(*ext.lastSavedVersion, 0x01320304)
	loaded.Assets.ITCompatibility.mptVolumeSlidePropagation = !additional.itVolColNoSlidePropagation ||
		!mptBehaviourAvailable(*ext.lastSavedVersion, 0x01320203)

	if err := applyMPTInstrumentFadeouts(loaded.Assets.ITInstruments, ext.instrumentFadeout); err != nil {
		return nil, err
	}

	if err := applyMPTInstrumentPanning(loaded.Assets.ITInstruments, ext.instrumentPanning); err != nil {
		return nil, err
	}

	if err := applyMPTInstrumentGlobalVolume(loaded.Assets.ITInstruments, ext.instrumentGlobalVolume); err != nil {
		return nil, err
	}

	if err := applyMPTCutoffSwing(loaded.Assets.ITInstruments, ext.instrumentCutSwing); err != nil {
		return nil, err
	}

	if err := applyMPTResonanceSwing(loaded.Assets.ITInstruments, ext.instrumentResSwing); err != nil {
		return nil, err
	}

	for i, mode := range ext.instrumentFilterMode {
		if i >= len(loaded.Assets.ITInstruments) {
			break
		}
		if mode != 0xff {
			loaded.Assets.ITInstruments[i].mptHasFilterMode = true
			loaded.Assets.ITInstruments[i].mptFilterMode = mode
		}
	}

	instrumentResampling := ext.instrumentResampling
	if len(loaded.Assets.ITInstruments) == 0 {
		instrumentResampling = nil
	}
	if err := applyMPTSongResampling(loaded.Assets.ITInstruments, instrumentResampling, ext.songResampling); err != nil {
		return nil, err
	}

	if ext.songResampling != nil {
		loaded.Assets.mptNearestResampling = *ext.songResampling == 0
		loaded.Assets.mptCubicResampling = *ext.songResampling == 2
		loaded.Assets.mptSincResampling = *ext.songResampling == 3
		if loaded.Assets.mptSincResampling {
			mptSincOnce.Do(initMPTSincTables)
		}
	}

	if len(loaded.Assets.ITInstruments) == 0 && (loaded.Assets.mptSincResampling || loaded.Assets.mptCubicResampling) {
		for i, sample := range loaded.Assets.Samples {
			if sample.LoopEnable && sample.LoopPingPong {
				mode := "sinc"
				if loaded.Assets.mptCubicResampling {
					mode = "cubic"
				}
				return nil, fmt.Errorf("MPTM sample %d uses %s resampling with an unsupported ping-pong loop", i+1, mode)
			}
		}
	}

	if err := validateMPTInterpolatedLoops(loaded.Assets.ITInstruments, loaded.Assets.Samples); err != nil {
		return nil, err
	}

	if !loaded.Assets.ITCompatibility.mptPingPongMode {
		for i, instrument := range loaded.Assets.ITInstruments {
			if instrument.mptNearestResampling {
				continue
			}
			for _, sample := range instrument.Samples {
				if sample != 0 && int(sample) <= len(loaded.Assets.Samples) &&
					(loaded.Assets.Samples[sample-1].itSustainPingPong ||
						loaded.Assets.Samples[sample-1].mptSustainLoop && loaded.Assets.Samples[sample-1].LoopPingPong) {
					return nil, fmt.Errorf("MPTM instrument %d: ping-pong sustain with linear resampling and non-IT ping-pong mode is not supported", i+1)
				}
			}
		}
	}

	assets := &loaded.Assets
	applyMPTExtraChannelSettings(assets, channels, ext.extraChannelSettings)

	legacyLinearPeriods := ext.playbackFlags == nil && assets.ITCompatibility.LinearSlides && *ext.lastSavedVersion < 0x01240000
	if legacyLinearPeriods {
		for i, sample := range assets.Samples {
			if sample.ITVibratoDepth != 0 && sample.ITVibratoRate != 0 {
				return nil, fmt.Errorf("MPTM sample %d: auto-vibrato with legacy linear periods is not supported", i+1)
			}
		}
		for i, instrument := range assets.ITInstruments {
			if instrument.PitchEnvelope.Enabled && !instrument.FilterEnvelope {
				return nil, fmt.Errorf("MPTM instrument %d: pitch envelope with legacy linear periods is not supported", i+1)
			}
		}
	}

	assets.NumTracks = channels
	if sequences.patterns != nil {
		bypassed, err := applyMPTPatternOverrides(sequences.patterns, assets.Patterns, assets.NumTracks)
		if err != nil {
			return nil, err
		}
		if bypassed {
			loaded.Warnings = append(loaded.Warnings, "MPTM plug-in parameter automation is bypassed; playback may sound different")
		}
	}

	if err := validateMPTPendingInstrumentAndNoteCut(assets, hasPluginRouting); err != nil {
		return nil, err
	}

	pitchProfile, err := projectMPTBehaviours(flags, *ext.lastSavedVersion, assets.ITCompatibility.LinearSlides, false, false)
	if err != nil {
		return nil, err
	}

	var macroConfig *mptMacroConfig

	embeddedMacros := u16(44)&0x80 != 0 || u16(46)&8 != 0
	hasRetrigger, checkedSmoothSwing, hasCueOffsets := false, false, false

	var cueOffsetTracks, effectOffsetTracks [MaxTracks]bool
	for patternIndex, pattern := range assets.Patterns {
		for rowIndex, row := range pattern.Rows {
			for channel := range row {
				cmd := &row[channel]

				if isMPTCueOffset(*cmd) {
					if *ext.lastSavedVersion < 0x01260000 || cmd.Note < 1 || cmd.Note > 120 || cmd.EffectCmd != 0 {
						return nil, fmt.Errorf("MPTM pattern %d row %d channel %d: cue offset requires an ordinary note without an effect on a 1.26+ profile", patternIndex, rowIndex, channel+1)
					}
					hasCueOffsets, cueOffsetTracks[channel] = true, true
				}

				if cmd.EffectCmd == 0x12 {
					effectOffsetTracks[channel] = true
				}

				if cmd.EffectCmd == 0x1a || cmd.EffectCmd == 0x1b {
					hasRetrigger = true
				}

				if legacyLinearPeriods {
					switch cmd.EffectCmd {
					case 0x01, 0x02, 0x03, 0x04, 0x10, 0x11, 0x1f, 0x21:
						return nil, fmt.Errorf("MPTM pattern %d row %d channel %d: pitch effect with legacy linear periods is not supported", patternIndex, rowIndex, channel+1)
					}
					if cmd.HasVolume && (cmd.Volume >= 105 && cmd.Volume <= 124 || cmd.Volume >= 193 && cmd.Volume <= 212) {
						return nil, fmt.Errorf("MPTM pattern %d row %d channel %d: volume-column pitch effect with legacy linear periods is not supported", patternIndex, rowIndex, channel+1)
					}
				}

				if cmd.EffectCmd == 0x19 || cmd.EffectCmd == 0x26 {
					smooth := cmd.EffectCmd == 0x26
					if smooth {
						if *ext.lastSavedVersion < 0x01260000 || !pitchProfile.known.enabled(mptITFilterBehaviour) {
							return nil, fmt.Errorf("MPTM pattern %d row %d channel %d: smooth MIDI macros with a legacy playback profile are not supported", patternIndex, rowIndex, channel+1)
						}

						if flags.enabled(mptOldSwingBehaviour) && !checkedSmoothSwing {
							for i, instrument := range assets.ITInstruments {
								if instrument.mptCutoffSwing != 0 || instrument.mptResonanceSwing != 0 {
									return nil, fmt.Errorf("MPTM instrument %d: smooth MIDI macros with old filter swing behavior are not supported", i+1)
								}
							}
							checkedSmoothSwing = true
						}
					}

					if macroConfig == nil {
						config, err := readMPTMacroConfig(data, bounds.propertiesStart)
						if err != nil {
							return nil, err
						}
						macroConfig = &config
					}

					if embeddedMacros && *ext.lastSavedVersion < 0x01260000 {
						return nil, fmt.Errorf("MPTM pattern %d row %d: embedded MIDI macros with a pre-1.26 playback profile are not supported", patternIndex, rowIndex)
					}

					if err := macroConfig.validateCommand(smooth, cmd.EffectArg, *ext.lastSavedVersion,
						pitchProfile.known.enabled(mptITFilterBehaviour)); err != nil {
						return nil, fmt.Errorf("MPTM pattern %d row %d channel %d: %w", patternIndex, rowIndex, channel+1, err)
					}
				}

				if cmd.EffectCmd == 0x15 && cmd.EffectArg&0xf0 == 0x10 {
					if *ext.lastSavedVersion < 0x01260000 {
						return nil, fmt.Errorf("MPTM pattern %d row %d channel %d: glissando with an unsupported pre-1.26 playback profile", patternIndex, rowIndex, channel+1)
					}

					if !pitchProfile.known.enabled(mptPeriodsAreHertz) {
						return nil, fmt.Errorf("MPTM pattern %d row %d channel %d: glissando with period-based pitch is not supported", patternIndex, rowIndex, channel+1)
					}
				}

				if cmd.EffectCmd == 0x15 && cmd.EffectArg >= 0x30 && cmd.EffectArg <= 0x5f {
					if *ext.lastSavedVersion < 0x01260000 {
						return nil, fmt.Errorf("MPTM pattern %d row %d channel %d: waveform selection with an unsupported pre-1.26 playback profile", patternIndex, rowIndex, channel+1)
					}

					if !pan.preciseWaveform && cmd.EffectArg&3 == 3 {
						return nil, fmt.Errorf("MPTM pattern %d row %d channel %d: legacy random S%02X waveform is not supported", patternIndex, rowIndex, channel+1, cmd.EffectArg)
					}
				}

				if *ext.lastSavedVersion < 0x01260000 {
					switch cmd.EffectCmd {
					case 0x11, 0x1f, 0x1e, 0x1d, 0x21, 0x1c, 0x22, 0x23:
						return nil, fmt.Errorf("MPTM pattern %d row %d uses a shared effect with an unsupported pre-1.26 playback profile", patternIndex, rowIndex)
					case 0x20:
						if cmd.EffectArg < 0x20 || flags.enabled(mptMODVBlankTiming) {
							return nil, fmt.Errorf("MPTM pattern %d row %d uses a tempo command with an unsupported pre-1.26 playback profile", patternIndex, rowIndex)
						}
					case 0x19:
						if cmd.EffectArg >= 0x80 {
							return nil, fmt.Errorf("MPTM pattern %d row %d uses a fixed macro with an unsupported pre-1.26 playback profile", patternIndex, rowIndex)
						}
					case 0x15:
						if cmd.EffectArg >= 0xf0 {
							return nil, fmt.Errorf("MPTM pattern %d row %d uses macro selection with an unsupported pre-1.26 playback profile", patternIndex, rowIndex)
						}
					}
				}

				if cmd.EffectCmd == 0x22 && cmd.EffectArg == 0x11 &&
					*ext.lastSavedVersion < 0x01291202 && flags.enabled(mptITTremor) && !assets.ITCompatibility.OldEffects {
					cmd.EffectArg = 0
				}

				if additional.itNoteOffOldEffects && assets.ITCompatibility.OldEffects && cmd.Note == 255 && cmd.Instrument != 0 {
					return nil, errors.New("MPTM note-off with instrument and old effects is not supported")
				}
			}
			for channel := assets.NumTracks; channel < len(row); channel++ {
				if row[channel] != (ModCommand{}) {
					return nil, fmt.Errorf("MPTM pattern %d row %d uses channel %d beyond the declared channel count",
						patternIndex, rowIndex, channel+1)
				}
			}
		}
	}

	for channel := range cueOffsetTracks {
		if cueOffsetTracks[channel] && effectOffsetTracks[channel] {
			return nil, fmt.Errorf("MPTM channel %d: cue offset with Oxx memory is not supported", channel+1)
		}
	}

	if hasCueOffsets {
		for i := range assets.Samples {
			for cue := range assets.Samples[i].mptCues {
				assets.Samples[i].mptCues[cue] = uint32(cue+1) << 11
			}

			if cues, ok := ext.sampleCues[uint16(i+1)]; ok {
				for cue := range assets.Samples[i].mptCues {
					assets.Samples[i].mptCues[cue] = 1 << 28
				}
				copy(assets.Samples[i].mptCues[:], cues)
			}
		}

		assets.ITCompatibility.mptCueOffsetITBehaviour = flags.enabled(mptITOffset)
	}

	if hasRetrigger && !flags.enabled(mptITShortSampleRetrig) {
		for _, sample := range assets.Samples {
			if len(sample.Data) != 0 && !sample.LoopEnable {
				loaded.Warnings = append(loaded.Warnings, "MPTM Qxx cannot restart a PCM sample after it has ended")
				break
			}
		}
	}

	assets.OrderList = nil
	for _, order := range seq.orders {
		if order == 0xffff {
			break
		}

		if order == 0xfffe {
			assets.OrderList = append(assets.OrderList, itOrderSkip)
		} else {
			assets.OrderList = append(assets.OrderList, int(order))
		}
	}

	if nextITOrder(assets.OrderList, 0) == len(assets.OrderList) {
		return nil, errors.New("MPTM has no playable orders")
	}

	if int(defaults.restart) >= len(assets.OrderList) {
		return nil, errors.New("MPTM restart after end marker is not supported")
	}

	assets.mptRestartOrder = int(defaults.restart)
	assets.mptSequenceLength = len(seq.orders)
	assets.mptRowsPerBeat = rowsPerBeat

	if rowsPerBeat != 0 {
		assets.mptTempoSwing = mptSongTempoSwing(ext.tempoSwing, rowsPerBeat)
	}

	assets.ITCompatibility.mptTempoUnclamped = !pitchProfile.known.enabled(mptTempoClamp)
	assets.ITCompatibility.mptTempoAsSpeed = pitchProfile.known.enabled(mptMODVBlankTiming)
	assets.ITCompatibility.itPatternLoopProfile = &loop
	assets.ITCompatibility.itPanbrelloProfile = &pan
	assets.ITCompatibility.itDCTSampleAcrossInstruments = duplicate
	assets.ITCompatibility.mptMixProfile = &mix
	assets.ITCompatibility.mptLegacySwing = *ext.lastSavedVersion >= 0x01200000 && *ext.lastSavedVersion < 0x01260000 &&
		!flags.enabled(mptITSwingBehaviour) && !flags.enabled(mptOldSwingBehaviour)
	assets.ITCompatibility.mptOldSwingBehaviour = flags.enabled(mptOldSwingBehaviour)

	if *ext.lastSavedVersion < 0x01170250 {
		for _, instrument := range assets.ITInstruments {
			if instrument.VolumeSwing != 0 || instrument.PanSwing != 0 || instrument.mptCutoffSwing != 0 || instrument.mptResonanceSwing != 0 {
				assets.ITCompatibility.mptOldSwingBehaviour = true
				break
			}
		}
	}

	if !flags.enabled(mptITFilterBehaviour) {
		for i, instrument := range assets.ITInstruments {
			if instrument.mptResonanceSwing != 0 && (!instrument.HasFilterCutoff || instrument.FilterCutoff == 127 ||
				!instrument.HasFilterResonance || instrument.FilterResonance == 0) {
				return nil, fmt.Errorf("MPTM instrument %d: resonance variation without compatible filter behavior is not supported", i+1)
			}
		}
	}

	assets.ITCompatibility.mptMacroConfig = macroConfig
	assets.ITCompatibility.oplEnvelopeEndCut = flags.enabled(mptITEnvelopePositionHandling)
	assets.ITCompatibility.mptEnvelopePositionHandling = flags.enabled(mptITEnvelopePositionHandling)
	assets.ITCompatibility.mptEnvelopeReset = flags.enabled(mptITEnvelopeReset)
	assets.ITCompatibility.mptVolumePitchSlides = true
	assets.ITCompatibility.mptRealNoteMapping = flags.enabled(mptITRealNoteMapping)
	assets.ITCompatibility.mptPanningReset = flags.enabled(mptITPanningReset)
	assets.ITCompatibility.mptITRetrigger = flags.enabled(mptITRetrigger)
	assets.ITCompatibility.mptLegacyTremor = !flags.enabled(mptITTremor)
	assets.ITCompatibility.mptGlobalVolumeSlideShared = !flags.enabled(mptPerChannelGlobalVolSlide)
	assets.ITCompatibility.mptSeparatePortamentoMemory = !flags.enabled(mptITPortaMemoryShare)
	assets.ITCompatibility.mptSharedVolumeColumnMemory = !flags.enabled(mptITVolColMemory)
	assets.ITCompatibility.mptPortamentoSwapResetsPos = flags.enabled(mptITPortamentoSwapResetsPos)
	assets.ITCompatibility.mptSlidesAtSpeedOne = flags.enabled(mptSlidesAtSpeed1)
	assets.ITCompatibility.mptMultiSampleInstrument = additional.itMultiSampleInstrument
	assets.ITCompatibility.mptInstrumentWithoutNote = flags.enabled(mptITInstrWithoutNote)
	assets.ITCompatibility.mptNNAReset = flags.enabled(mptITNNAReset)
	assets.ITCompatibility.mptCutStopsSample = flags.enabled(mptITSCxStopsSample)
	assets.ITCompatibility.mptClearOldNoteAfterCut = flags.enabled(mptITClearOldNoteAfterCut)
	assets.ITCompatibility.mptPortamentoInstrument = flags.enabled(mptITPortamentoInstrument)
	assets.ITCompatibility.itVolColFinePortamento = flags.enabled(mptITVolColFinePortamento)

	hasOPL := false
	for _, sample := range assets.Samples {
		if sample.opl != nil {
			hasOPL = true
			break
		}
	}

	assets.ITCompatibility.oplNoResetAtEnvelopeEnd = projectMPTOPLNoResetAtEnvelopeEnd(
		additional.oplNoResetAtEnvelopeEnd, *ext.lastSavedVersion, hasOPL, len(assets.ITInstruments) != 0)
	assets.ITCompatibility.oplPortaSampleRetrigger = true
	assets.ITCompatibility.oplInstrumentSwitch = true
	loaded.InitialSpeed, loaded.InitialTempo = int(defaults.speed), float64(defaults.tempo)/mptTempoScale

	if hasPluginRouting {
		loaded.Warnings = append(loaded.Warnings, "MPTM instrument plug-ins are bypassed; playback may sound different")
	}

	return loaded, nil
}

func validateMPTVolumeRamp(values []uint16) error {
	for i, value := range values {
		if value != 0 {
			return fmt.Errorf("MPTM instrument %d uses unsupported volume ramp-up %d", i+1, value)
		}
	}
	return nil
}
