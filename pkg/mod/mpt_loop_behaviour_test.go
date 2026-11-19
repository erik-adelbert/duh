package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestMPTLoopProfileProjection(t *testing.T) {
	flags := mptSavedPlayBehaviours(1<<mptITPatternLoopTargetReset |
		1<<mptITFT2PatternLoop | 1<<mptITPatternLoopBreak | 1<<mptITPatternLoopWithJumpsOld)
	for _, tt := range []struct {
		name    string
		flags   mptSavedPlayBehaviours
		version uint32
		want    itPatternLoopProfile
		fail    bool
	}{
		{"target", flags, 0x01280100, itPatternLoopProfile{true, true, true, itJumpAlwaysWins}, false},
		{"disabled", 0, 0x01260000, itPatternLoopProfile{}, false},
		{"mixed", 1 << mptITPatternLoopBreak, 0x01289999, itPatternLoopProfile{preserveOnBreak: true}, false},
		{"old version", 0, 0x01259999, itPatternLoopProfile{}, false},
		{"new version", 0, 0x01290000, itPatternLoopProfile{}, false},
		{"unknown flags", 1 << mptKnownPlayBehaviourCount, 0x01280100, itPatternLoopProfile{}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := projectMPTLoopProfile(tt.flags, tt.version, false)
			if (err != nil) != tt.fail || err == nil && got != tt.want {
				t.Fatalf("profile %+v, error %v, want %+v, fail %v", got, err, tt.want, tt.fail)
			}
		})
	}
}

func TestMPTPositionSensitiveLoopProjection(t *testing.T) {
	for _, tt := range []struct {
		version uint32
		old     bool
		modern  bool
		want    itLoopJumpPriority
	}{
		{0x01280000, true, true, itJumpAlwaysWins},
		{0x01290000, true, true, itJumpRightWins},
		{0x01290031, true, true, itJumpAlwaysWins},
		{0x01290032, true, true, itJumpRightWins},
		{0x01310400, false, true, itJumpRightWins},
		{0x01310400, false, false, itLoopWins},
	} {
		var flags mptSavedPlayBehaviours
		if tt.old {
			flags |= 1 << mptITPatternLoopWithJumpsOld
		}
		got, err := projectMPTLoopProfile(flags, tt.version, tt.modern)
		if err != nil || got.jumpPriority != tt.want {
			t.Errorf("version %#x old=%v modern=%v: got %+v, %v; want %v", tt.version, tt.old, tt.modern, got, err, tt.want)
		}
	}
	data := make([]byte, 13)
	data[mptITPatternLoopWithJumpsBit/8] = 1 << (mptITPatternLoopWithJumpsBit % 8)
	_, additional, err := parseMPTPlaybackProfileFlags(data)
	if err != nil || !additional.itPatternLoopWithJumps {
		t.Fatalf("bit 103: %+v, %v", additional, err)
	}
}

func mptLoopReferenceLevels(t *testing.T, patterns [][][]loopTestEffect, flags mptSavedPlayBehaviours, version uint32, positionSensitive bool) []float64 {
	t.Helper()
	profile, err := projectMPTLoopProfile(flags, version, positionSensitive)
	if err != nil {
		t.Fatal(err)
	}
	b := itLoopReferenceFixture(patterns, 2, false)
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint16(b[40:], 0x891)
	b = append(b, []byte("STPM")...)
	b = append(b, mptExtensionField("VWSL", 4, binary.LittleEndian.AppendUint32(nil, version))...)
	flagBytes := make([]byte, 7)
	if positionSensitive {
		flagBytes = make([]byte, mptITPatternLoopWithJumpsBit/8+1)
		flagBytes[mptITPatternLoopWithJumpsBit/8] |= 1 << (mptITPatternLoopWithJumpsBit % 8)
	}
	for i := range 7 {
		flagBytes[i] = byte(uint64(flags) >> (8 * i))
	}
	b = append(b, mptExtensionField(".FSM", uint16(len(flagBytes)), flagBytes)...)
	tail := len(b)
	orders := make([]byte, 0, len(patterns)*2)
	for i := range patterns {
		orders = binary.LittleEndian.AppendUint16(orders, uint16(i))
	}
	b = append(b, mptTestSequences([]mptTestEntry{
		{"l", binary.LittleEndian.AppendUint16(nil, uint16(len(patterns)))},
		{"a", orders},
	})...)
	b = binary.LittleEndian.AppendUint32(b, uint32(tail))
	ref := renderMPTSequenceReference(t, b, "--repeat", "-1", "--end-time", "0.280")
	actual := make([]float32, 280*referenceRate/1000*2)
	if len(ref) < len(actual) {
		t.Fatalf("short OpenMPT render: %d samples", len(ref))
	}
	loaded.Assets.ITCompatibility.itPatternLoopProfile = &profile
	w := NewTracker(loaded.Assets, referenceRate, 960)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	for i := 0; i < len(actual)/2; i += 960 {
		if got := w.Read(actual[i*2:(i+960)*2], 960); got != 960 {
			t.Fatalf("short render at frame %d: %d", i, got)
		}
	}
	refBase, actualBase := channelRMS(ref, 8, 13, 0), channelRMS(actual, 8, 13, 0)
	var levels []float64
	for ms := 8; ms < 275; ms += 40 {
		want := channelRMS(ref, ms, ms+5, 0) / refBase
		got := channelRMS(actual, ms, ms+5, 0) / actualBase
		levels = append(levels, want)
		if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .20 {
			t.Errorf("%dms: OpenMPT %.2f, tracker %.2f", ms, want, got)
		}
	}
	return levels
}

func TestOpenMPTPositionSensitiveLoopJumps(t *testing.T) {
	sb := func(ch, param byte) loopTestEffect { return loopTestEffect{ch, 19, param} }
	jump := func(ch byte) loopTestEffect { return loopTestEffect{ch, 2, 1} }
	brk := func(ch byte) loopTestEffect { return loopTestEffect{ch, 3, 1} }
	for _, old := range []bool{false, true} {
		for _, scenario := range []struct {
			name string
			row  []loopTestEffect
			ch   byte
		}{
			{"jump left", []loopTestEffect{jump(1), sb(2, 0xb1)}, 2},
			{"jump right", []loopTestEffect{sb(1, 0xb1), jump(2)}, 1},
			{"break", []loopTestEffect{sb(1, 0xb1), brk(2)}, 1},
		} {
			t.Run(fmt.Sprintf("%s/old=%v", scenario.name, old), func(t *testing.T) {
				patterns := [][][]loopTestEffect{
					{{sb(scenario.ch, 0xb0)}, nil, scenario.row, nil},
					{nil, nil, nil, nil, nil, nil, nil, nil},
				}
				flags := mptSavedPlayBehaviours(1<<mptITPatternLoopTargetReset | 1<<mptITFT2PatternLoop | 1<<mptITPatternLoopBreak)
				if old {
					flags |= 1 << mptITPatternLoopWithJumpsOld
				}
				mptLoopReferenceLevels(t, patterns, flags, 0x01310400, true)
			})
		}
	}
}

func TestOpenMPTLoopProfileProjection(t *testing.T) {
	sb := func(ch, param byte) loopTestEffect { return loopTestEffect{ch, 19, param} }
	jump := func(ch, order byte) loopTestEffect { return loopTestEffect{ch, 2, order} }
	brk := func(ch, row byte) loopTestEffect { return loopTestEffect{ch, 3, row} }
	all := mptSavedPlayBehaviours(1<<mptITPatternLoopTargetReset | 1<<mptITFT2PatternLoop |
		1<<mptITPatternLoopBreak | 1<<mptITPatternLoopWithJumpsOld)
	tests := []struct {
		name     string
		flag     mptPlayBehaviour
		patterns [][][]loopTestEffect
	}{
		{"old jump left", mptITPatternLoopWithJumpsOld, [][][]loopTestEffect{
			{{sb(2, 0xb0)}, nil, {jump(1, 1), sb(2, 0xb1)}, nil},
			{nil, nil, nil, nil, nil, nil, nil, nil},
		}},
		{"old jump right", mptITPatternLoopWithJumpsOld, [][][]loopTestEffect{
			{{sb(1, 0xb0)}, nil, {sb(1, 0xb1), jump(2, 1)}, nil},
			{nil, nil, nil, nil, nil, nil, nil, nil},
		}},
		{"target reset", mptITPatternLoopTargetReset, [][][]loopTestEffect{
			{nil, {sb(1, 0xb1)}, nil, {sb(1, 0xb1)}, nil, nil, nil, nil},
		}},
		{"nested", mptITFT2PatternLoop, [][][]loopTestEffect{
			{{sb(1, 0xb0)}, {sb(2, 0xb0)}, nil, {sb(1, 0xb1), sb(2, 0xb1)}, nil, nil, nil, nil},
		}},
		{"break memory", mptITPatternLoopBreak, [][][]loopTestEffect{
			{nil, {sb(1, 0xb0)}, {sb(1, 0xb3), jump(2, 1)}, nil},
			{{sb(1, 0xb1)}, {sb(1, 0xb1)}, {brk(2, 0)}, nil},
			{nil, nil, nil, nil, nil, nil, nil, nil},
		}},
	}
	for _, tt := range tests {
		var referenceLevels [2][]float64
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/enabled=%v", tt.name, enabled), func(t *testing.T) {
				flags := all &^ (1 << tt.flag)
				if enabled {
					flags |= 1 << tt.flag
				}
				index := 0
				if enabled {
					index = 1
				}
				referenceLevels[index] = mptLoopReferenceLevels(t, tt.patterns, flags, 0x01280100, false)
			})
		}
		distinct := false
		if len(referenceLevels[0]) == 0 || len(referenceLevels[1]) == 0 {
			continue
		}
		for i := range referenceLevels[0] {
			if math.Abs(referenceLevels[0][i]-referenceLevels[1][i]) > .3 {
				distinct = true
			}
		}
		if !distinct {
			t.Errorf("%s: reference fixtures did not distinguish enabled and disabled flags", tt.name)
		}
	}
}
