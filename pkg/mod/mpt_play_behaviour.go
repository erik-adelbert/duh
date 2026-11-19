// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import (
	"errors"
	"fmt"
)

// The low-bit projection covers stable serialized PlayBehaviour indices
// through OpenMPT's kITInstrWithNoteOff.
type mptPlayBehaviour uint8

const (
	mptCompatiblePlay mptPlayBehaviour = iota
	mptOldSwingBehaviour
	mptMIDICCBugEmulation
	mptOldMIDIPitchBends
	mptFT2VolumeRamping
	mptMODVBlankTiming
	mptSlidesAtSpeed1
	mptPeriodsAreHertz
	mptTempoClamp
	mptPerChannelGlobalVolSlide
	mptPanOverride
	mptITInstrWithoutNote
	mptITVolColFinePortamento
	mptITArpeggio
	mptITOutOfRangeDelay
	mptITPortaMemoryShare
	mptITPatternLoopTargetReset
	mptITFT2PatternLoop
	mptITPingPongNoReset
	mptITEnvelopeReset
	mptITClearOldNoteAfterCut
	mptITVibratoTremoloPanbrello
	mptITTremor
	mptITRetrigger
	mptITMultiSampleBehaviour
	mptITPortaTargetReached
	mptITPatternLoopBreak
	mptITOffset
	mptITSwingBehaviour
	mptITNNAReset
	mptITSCxStopsSample
	mptITEnvelopePositionHandling
	mptITPortamentoInstrument
	mptITPingPongMode
	mptITRealNoteMapping
	mptITHighOffsetNoRetrig
	mptITFilterBehaviour
	mptITNoSurroundPan
	mptITShortSampleRetrig
	mptITPortaNoNote
	mptITFT2DontResetNoteOffOnPorta
	mptITVolColMemory
	mptITPortamentoSwapResetsPos
	mptITEmptyNoteMapSlot
	mptITFirstTickHandling
	mptITSampleAndHoldPanbrello
	mptITClearPortaTarget
	mptITPanbrelloHold
	mptITPanningReset
	mptITPatternLoopWithJumpsOld
	mptITInstrWithNoteOff
	mptKnownPlayBehaviourCount
)

// mptSavedPlayBehaviours contains serialized flags, not the effective flags
// after OpenMPT's version- and song-dependent upgrades.
type mptSavedPlayBehaviours uint64

func (flags mptSavedPlayBehaviours) enabled(behaviour mptPlayBehaviour) bool {
	return behaviour < mptKnownPlayBehaviourCount && flags&(1<<behaviour) != 0
}

func parseMPTSavedPlayBehaviours(data []byte) (mptSavedPlayBehaviours, error) {
	flags, _, err := parseMPTPlayFlags(data, false)
	return flags, err
}

type mptAdditionalPlayBehaviours struct {
	itMultiSampleInstrument    bool
	rowDelayWithNoteDelay      bool
	itNoteOffOldEffects        bool
	oplFlexibleNoteOff         bool
	itRestoreChannelPan        bool
	itDuplicateCheck           bool
	oplWithNNA                 bool
	oplRealRetrig              bool
	oplNoResetAtEnvelopeEnd    bool
	itPitchPanSeparation       bool
	itPatternLoopWithJumps     bool
	itResetFilterOnPorta       bool
	itInitialNoteMemory        bool
	itNoSustainOnPortamento    bool
	itEmptyNoteMapIgnoreCell   bool
	itOffsetWithInstrument     bool
	itDoublePortamentoSlides   bool
	itCarryAfterNoteOff        bool
	itNoteCutWithPorta         bool
	itVolColNoSlidePropagation bool
	itStoppedFilterEnvAtStart  bool
}

const (
	mptITPatternLoopWithJumpsBit     = 103
	mptOPLRealRetrigBit              = 108
	mptITResetFilterOnPortaBit       = 119
	mptITInitialNoteMemoryBit        = 120
	mptITNoSustainOnPortamentoBit    = 122
	mptITEmptyNoteMapIgnoreCellBit   = 123
	mptITOffsetWithInstrumentBit     = 124
	mptITDoublePortamentoSlidesBit   = 127
	mptITCarryAfterNoteOffBit        = 132
	mptITNoteCutWithPortaBit         = 134
	mptITVolColNoSlidePropagationBit = 135
	mptITStoppedFilterEnvAtStartBit  = 136
)

func mptBehaviourAvailable(version, introduced uint32) bool {
	// Compatibility exports retain only the major and minor version.
	return version == introduced&0xffff0000 || version >= introduced
}

func parseMPTPlaybackProfileFlags(data []byte) (mptSavedPlayBehaviours, mptAdditionalPlayBehaviours, error) {
	return parseMPTPlayFlags(data, true)
}

func resolveMPTPlaybackProfileFlags(data []byte, version uint32, linearSlides bool) (mptSavedPlayBehaviours, mptAdditionalPlayBehaviours, error) {
	if data != nil {
		return parseMPTPlaybackProfileFlags(data)
	}
	if version < 0x01200000 || version >= 0x01260000 {
		return 0, mptAdditionalPlayBehaviours{}, fmt.Errorf("MPTM playback flags are missing for unsupported saved version %#08x", version)
	}
	// OpenMPT resets the serialized flags when it reads song extensions.
	// With no field, old non-compatible mode starts from zero.
	var flags mptSavedPlayBehaviours

	if version > 0x01200000 && version <= 0x01200103 {
		flags |= 1 << mptSlidesAtSpeed1
	}

	if linearSlides && version >= 0x01240000 {
		flags |= 1 << mptPeriodsAreHertz
	}

	return flags, mptAdditionalPlayBehaviours{}, nil
}

func parseMPTPlayFlags(data []byte, recognizeAdditional bool) (mptSavedPlayBehaviours, mptAdditionalPlayBehaviours, error) {
	if data == nil {
		return 0, mptAdditionalPlayBehaviours{}, errors.New("missing MPTM playback flags")
	}

	var result mptSavedPlayBehaviours
	var additional mptAdditionalPlayBehaviours

	for byteIndex, value := range data {
		for bit := range uint8(8) {
			if value&(1<<bit) == 0 {
				continue
			}

			index := byteIndex*8 + int(bit)
			if index >= int(mptKnownPlayBehaviourCount) {
				if recognizeAdditional {
					switch index {
					case 87:
						additional.itMultiSampleInstrument = true
						continue
					case 88:
						additional.rowDelayWithNoteDelay = true
						continue
					case 99:
						additional.oplFlexibleNoteOff = true
						continue
					case 100:
						additional.itNoteOffOldEffects = true
						continue
					case 102:
						additional.itRestoreChannelPan = true
						continue
					case mptITPatternLoopWithJumpsBit:
						additional.itPatternLoopWithJumps = true
						continue
					case 104:
						additional.itDuplicateCheck = true
						continue
					case 105:
						additional.oplWithNNA = true
						continue
					case mptOPLRealRetrigBit:
						additional.oplRealRetrig = true
						continue
					case 109:
						additional.oplNoResetAtEnvelopeEnd = true
						continue
					case 115:
						additional.itPitchPanSeparation = true
						continue
					case mptITResetFilterOnPortaBit:
						additional.itResetFilterOnPorta = true
						continue
					case mptITInitialNoteMemoryBit:
						additional.itInitialNoteMemory = true
						continue
					case mptITNoSustainOnPortamentoBit:
						additional.itNoSustainOnPortamento = true
						continue
					case mptITEmptyNoteMapIgnoreCellBit:
						additional.itEmptyNoteMapIgnoreCell = true
						continue
					case mptITOffsetWithInstrumentBit:
						additional.itOffsetWithInstrument = true
						continue
					case mptITDoublePortamentoSlidesBit:
						additional.itDoublePortamentoSlides = true
						continue
					case mptITCarryAfterNoteOffBit:
						additional.itCarryAfterNoteOff = true
						continue
					case mptITNoteCutWithPortaBit:
						additional.itNoteCutWithPorta = true
						continue
					case mptITVolColNoSlidePropagationBit:
						additional.itVolColNoSlidePropagation = true
						continue
					case mptITStoppedFilterEnvAtStartBit:
						additional.itStoppedFilterEnvAtStart = true
						continue
					}
				}

				err := fmt.Errorf("unsupported MPTM playback behaviour bit %d", index)
				return 0, mptAdditionalPlayBehaviours{}, err
			}
			result |= 1 << uint(index)
		}
	}

	return result, additional, nil
}
