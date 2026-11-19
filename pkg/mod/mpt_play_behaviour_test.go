package mod

import (
	"strings"
	"testing"
)

func TestMPTSavedPlayBehaviourIndices(t *testing.T) {
	ordered := []mptPlayBehaviour{
		mptCompatiblePlay, mptOldSwingBehaviour, mptMIDICCBugEmulation, mptOldMIDIPitchBends,
		mptFT2VolumeRamping, mptMODVBlankTiming, mptSlidesAtSpeed1, mptPeriodsAreHertz,
		mptTempoClamp, mptPerChannelGlobalVolSlide, mptPanOverride, mptITInstrWithoutNote,
		mptITVolColFinePortamento, mptITArpeggio, mptITOutOfRangeDelay, mptITPortaMemoryShare,
		mptITPatternLoopTargetReset, mptITFT2PatternLoop, mptITPingPongNoReset, mptITEnvelopeReset,
		mptITClearOldNoteAfterCut, mptITVibratoTremoloPanbrello, mptITTremor, mptITRetrigger,
		mptITMultiSampleBehaviour, mptITPortaTargetReached, mptITPatternLoopBreak, mptITOffset,
		mptITSwingBehaviour, mptITNNAReset, mptITSCxStopsSample, mptITEnvelopePositionHandling,
		mptITPortamentoInstrument, mptITPingPongMode, mptITRealNoteMapping, mptITHighOffsetNoRetrig,
		mptITFilterBehaviour, mptITNoSurroundPan, mptITShortSampleRetrig, mptITPortaNoNote,
		mptITFT2DontResetNoteOffOnPorta, mptITVolColMemory, mptITPortamentoSwapResetsPos,
		mptITEmptyNoteMapSlot, mptITFirstTickHandling, mptITSampleAndHoldPanbrello,
		mptITClearPortaTarget, mptITPanbrelloHold, mptITPanningReset,
		mptITPatternLoopWithJumpsOld, mptITInstrWithNoteOff,
	}
	if len(ordered) != int(mptKnownPlayBehaviourCount) {
		t.Fatalf("known behaviour count = %d, want %d", mptKnownPlayBehaviourCount, len(ordered))
	}
	for i, flag := range ordered {
		if int(flag) != i {
			t.Fatalf("behaviour at index %d has value %d", i, flag)
		}
	}
	tests := []struct {
		name string
		flag mptPlayBehaviour
		bit  int
	}{
		{"old compatibility", mptCompatiblePlay, 0},
		{"old swing", mptOldSwingBehaviour, 1},
		{"periods are hertz", mptPeriodsAreHertz, 7},
		{"tempo clamp", mptTempoClamp, 8},
		{"instrument without note", mptITInstrWithoutNote, 11},
		{"arpeggio", mptITArpeggio, 13},
		{"porta memory sharing", mptITPortaMemoryShare, 15},
		{"vibrato", mptITVibratoTremoloPanbrello, 21},
		{"note cut", mptITSCxStopsSample, 30},
		{"envelope position", mptITEnvelopePositionHandling, 31},
		{"portamento instrument", mptITPortamentoInstrument, 32},
		{"filter", mptITFilterBehaviour, 36},
		{"volume column memory", mptITVolColMemory, 41},
		{"panning reset", mptITPanningReset, 48},
		{"old pattern loop jumps", mptITPatternLoopWithJumpsOld, 49},
		{"instrument with note off", mptITInstrWithNoteOff, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.flag) != tt.bit {
				t.Fatalf("index = %d, want %d", tt.flag, tt.bit)
			}
			data := make([]byte, tt.bit/8+1)
			data[tt.bit/8] = 1 << uint(tt.bit%8)
			flags, err := parseMPTSavedPlayBehaviours(data)
			if err != nil {
				t.Fatal(err)
			}
			for i := range mptKnownPlayBehaviourCount {
				if got, want := flags.enabled(i), i == tt.flag; got != want {
					t.Fatalf("bit %d = %v, want %v", i, got, want)
				}
			}
		})
	}
}

func TestMPTSavedPlayBehavioursEmptyAndUnsupported(t *testing.T) {
	if _, err := parseMPTSavedPlayBehaviours(nil); err == nil {
		t.Fatal("missing field was accepted")
	}
	for _, data := range [][]byte{{}, {0}, {0, 0, 0, 0, 0, 0, 0, 0, 0}} {
		flags, err := parseMPTSavedPlayBehaviours(data)
		if err != nil || flags != 0 {
			t.Fatalf("flags %x: got %#x, error %v", data, flags, err)
		}
	}
	for _, data := range [][]byte{
		{0, 0, 0, 0, 0, 0, 8},
		{0, 0, 0, 0, 0, 0, 0, 1},
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 8}, // OPL flexible note-off
	} {
		if _, err := parseMPTSavedPlayBehaviours(data); err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("flags %x: expected unsupported error, got %v", data, err)
		}
	}
	if _, err := parseMPTSavedPlayBehaviours([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 8}); err == nil ||
		!strings.Contains(err.Error(), "bit 99") {
		t.Fatalf("expected named bit 99 rejection, got %v", err)
	}
	if mptSavedPlayBehaviours(^uint64(0)).enabled(mptKnownPlayBehaviourCount) {
		t.Fatal("out-of-range behaviour reported as enabled")
	}
}

func TestMPTPlaybackFlagsFieldPresence(t *testing.T) {
	for _, tt := range []struct {
		name    string
		data    []byte
		present bool
	}{
		{"absent", []byte("STPM"), false},
		{"empty", append([]byte("STPM"), mptExtensionField(".FSM", 0, nil)...), true},
		{"zero", append([]byte("STPM"), mptExtensionField(".FSM", 1, []byte{0})...), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			extensions, err := parseMPTExtensions(tt.data, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := extensions.playbackFlags != nil; got != tt.present {
				t.Fatalf("field present = %v, want %v", got, tt.present)
			}
			flags, err := parseMPTSavedPlayBehaviours(extensions.playbackFlags)
			if (err == nil) != tt.present || flags != 0 {
				t.Fatalf("flags = %#x, error = %v", flags, err)
			}
		})
	}
}

func FuzzMPTSavedPlayBehaviours(f *testing.F) {
	f.Add([]byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7})
	f.Add([]byte{0})
	f.Add([]byte{0, 0, 0, 0, 0, 0, 8})
	f.Fuzz(func(t *testing.T, data []byte) {
		flags, err := parseMPTSavedPlayBehaviours(data)
		if err != nil {
			return
		}

		for i := 0; i < len(data)*8; i++ {
			set := data[i/8]&(1<<uint(i%8)) != 0
			if i >= int(mptKnownPlayBehaviourCount) {
				if set {
					t.Fatalf("unsupported set bit %d", i)
				}
			} else if flags.enabled(mptPlayBehaviour(i)) != set {
				t.Fatalf("bit %d differs from serialized field", i)
			}
		}
	})
}
