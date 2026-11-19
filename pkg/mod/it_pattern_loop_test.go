package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

type loopTestEffect struct {
	channel byte
	effect  byte
	arg     byte
}

func itLoopReferenceFixture(patterns [][][]loopTestEffect, speed byte, legacy bool) []byte {
	b := testITPitchFixture(nil, speed, true, legacy)
	b[256+19] = 64
	binary.LittleEndian.PutUint16(b[32:], uint16(len(patterns)+1))
	binary.LittleEndian.PutUint16(b[38:], uint16(len(patterns)))
	for i := range patterns {
		b[192+i] = byte(i)
	}
	b[192+len(patterns)] = 255
	table := 192 + len(patterns) + 1
	binary.LittleEndian.PutUint32(b[table:], 256)
	for p, rows := range patterns {
		binary.LittleEndian.PutUint32(b[table+4+p*4:], uint32(len(b)))
		packed := make([]byte, 0)
		rowCount := len(rows)
		if p == len(patterns)-1 {
			rowCount = max(16, rowCount)
		}
		for row := 0; row < rowCount; row++ {
			var effects []loopTestEffect
			if row < len(rows) {
				effects = rows[row]
			}
			volume := byte(8 + (row+p*2)%7*8)
			packed = append(packed, 0x81, 7, 60, 1, volume)
			for _, effect := range effects {
				packed = append(packed, 0x80|effect.channel+1, 8, effect.effect, effect.arg)
			}
			packed = append(packed, 0)
		}
		header := make([]byte, 8)
		binary.LittleEndian.PutUint16(header, uint16(len(packed)))
		binary.LittleEndian.PutUint16(header[2:], uint16(rowCount))
		b = append(b, header...)
		b = append(b, packed...)
	}
	return b
}

func TestOpenMPTITPatternLoops(t *testing.T) {
	sb := func(channel, arg byte) loopTestEffect { return loopTestEffect{channel, 19, arg} }
	jump := func(channel, order byte) loopTestEffect { return loopTestEffect{channel, 2, order} }
	patternBreak := func(channel, row byte) loopTestEffect { return loopTestEffect{channel, 3, row} }
	tests := []struct {
		name     string
		patterns [][][]loopTestEffect
	}{
		{"basic", [][][]loopTestEffect{
			{nil, {sb(1, 0xb0)}, nil, {sb(1, 0xb2)}, nil, nil, nil, nil},
		}},
		{"implicit and reset", [][][]loopTestEffect{
			{nil, {sb(1, 0xb1)}, nil, {sb(1, 0xb1)}, nil, nil, nil, nil},
		}},
		{"reuse S00", [][][]loopTestEffect{
			{nil, {sb(1, 0xb1)}, nil, {sb(1, 0)}, nil, nil, nil, nil},
		}},
		{"two channels", [][][]loopTestEffect{
			{{sb(1, 0xb0)}, {sb(2, 0xb0)}, nil, {sb(1, 0xb1), sb(2, 0xb1)}, nil, nil, nil, nil},
		}},
		{"break on loop", [][][]loopTestEffect{
			{{sb(1, 0xb0)}, nil, {sb(1, 0xb1), patternBreak(2, 2)}, nil},
			{nil, nil, nil, nil, nil, nil, nil, nil},
		}},
		{"jump before loop", [][][]loopTestEffect{
			{{sb(2, 0xb0)}, nil, {jump(1, 1), sb(2, 0xb1)}, nil},
			{nil, nil, nil, nil, nil, nil, nil, nil},
		}},
		{"jump after loop", [][][]loopTestEffect{
			{{sb(1, 0xb0)}, nil, {sb(1, 0xb1), jump(2, 1)}, nil},
			{nil, nil, nil, nil, nil, nil, nil, nil},
		}},
		{"jump break after loop", [][][]loopTestEffect{
			{{sb(1, 0xb0)}, nil, {sb(1, 0xb1), jump(2, 1), patternBreak(3, 2)}, nil},
			{nil, nil, nil, nil, nil, nil, nil, nil},
		}},
		{"loop counter across jump", [][][]loopTestEffect{
			{nil, {sb(1, 0xb0)}, {sb(1, 0xb2), jump(2, 1)}, nil},
			{{sb(1, 0xb1)}, nil, nil, nil, nil, nil, nil, nil},
		}},
		{"loop target past pattern", [][][]loopTestEffect{
			{nil, nil, nil, nil, nil, {sb(1, 0xb0)}, {patternBreak(2, 0)}},
			{{sb(1, 0xb1)}, nil, nil},
			{nil, nil, nil, nil, nil, nil, nil, nil},
		}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			for _, speed := range []byte{1, 2} {
				t.Run(fmt.Sprintf("%s/legacy=%v/speed=%d", tt.name, legacy, speed), func(t *testing.T) {
					b := itLoopReferenceFixture(tt.patterns, speed, legacy)
					duration := 200
					if legacy && speed == 1 && (tt.name == "implicit and reset" || tt.name == "reuse S00") {
						// Legacy target retention makes these fixtures infinite; OpenMPT fades on loop detection.
						duration = 120
					}
					ref, actual := referencePCMForDuration(t, b, "2", duration)
					refBase, actualBase := channelRMS(ref, 8, 13, 0), channelRMS(actual, 8, 13, 0)
					if refBase < .0001 || actualBase < .0001 {
						t.Fatal("silent reference baseline")
					}
					for ms := 8; ms+5 <= duration-5; ms += int(speed) * 20 {
						want := channelRMS(ref, ms, ms+5, 0) / refBase
						got := channelRMS(actual, ms, ms+5, 0) / actualBase
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .20 {
							t.Errorf("%dms: OpenMPT row level %.2f, tracker %.2f", ms, want, got)
						}
					}
				})
			}
		}
	}
}

func TestITPatternLoopStateAndFlow(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%v", legacy), func(t *testing.T) {
			rows := func(n int) [][]ModCommand {
				r := make([][]ModCommand, n)
				for i := range r {
					r[i] = make([]ModCommand, 2)
				}
				return r
			}
			w := NewTracker(SongAssets{
				IT: true, NumTracks: 2, ITCompatibility: ITCompatibility{LegacyModPlug: legacy},
				OrderList: []int{0, 1}, Patterns: []PatternAsset{{Rows: rows(8)}, {Rows: rows(3)}},
			}, 48000, 1)
			w.Clock.Speed = 2
			w.Clock.Row = 2
			w.PendingCmds[0] = ModCommand{EffectCmd: 0x15, EffectArg: 0xb0}
			GlobalFlowSystem(w)
			VolumeEffectSystem(w)
			if w.TrackMemories[0].itLoopStart != 2 || w.TrackMemories[0].ITLastSpecial != 0xb0 {
				t.Fatal("SB0 did not store loop start and special-command memory")
			}
			w.Clock.Row, w.Clock.Tick = 4, 0
			w.PendingCmds[0].EffectArg = 0xb2
			GlobalFlowSystem(w)
			VolumeEffectSystem(w)
			if w.TrackMemories[0].itLoopCount != 2 || w.itFlowPending {
				t.Fatal("SB2 did not initialize exactly once on the first tick")
			}
			w.Clock.Tick = 1
			GlobalFlowSystem(w)
			if w.TrackMemories[0].itLoopCount != 2 || !w.itFlowPending {
				t.Fatal("SB2 changed its counter on a later tick")
			}
			w.Clock.SamplesRemaining = 0
			if !TickSchedulerSystem(w) || w.Clock.Row != 2 || w.Clock.Order != 0 {
				t.Fatal("SB2 did not return to SB0")
			}
			for want := uint8(1); ; want-- {
				w.Clock.Row, w.Clock.Tick = 4, 0
				w.PendingCmds[0].EffectArg = 0
				GlobalFlowSystem(w)
				VolumeEffectSystem(w)
				if got := w.TrackMemories[0].itLoopCount; got != want {
					t.Fatalf("S00 recalled loop count = %d, want %d", got, want)
				}
				if want == 0 {
					break
				}
			}
			wantStart := 5
			if legacy {
				wantStart = 2
			}
			if got := w.TrackMemories[0].itLoopStart; got != wantStart {
				t.Fatalf("completed loop start = %d, want %d", got, wantStart)
			}
			if w.TrackMemories[1].itLoopCount != 0 || w.TrackMemories[1].itLoopStart != 0 {
				t.Fatal("loop state leaked to another channel")
			}
		})
	}
}

func TestITPatternLoopBreakPreservesOrResetsCounter(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		w := NewTracker(SongAssets{
			IT: true, NumTracks: 1, ITCompatibility: ITCompatibility{LegacyModPlug: legacy},
			OrderList: []int{0, 1}, Patterns: []PatternAsset{{Rows: make([][]ModCommand, 4)}, {Rows: make([][]ModCommand, 4)}},
		}, 48000, 1)
		w.Clock.Speed = 1
		w.TrackMemories[0].itLoopStart, w.TrackMemories[0].itLoopCount = 3, 2
		w.PendingCmds[0] = ModCommand{EffectCmd: 0x0d}
		GlobalFlowSystem(w)
		w.Clock.SamplesRemaining = 0
		if !TickSchedulerSystem(w) || w.Clock.Order != 1 {
			t.Fatal("pattern break did not advance")
		}
		want := uint8(2)
		if legacy {
			want = 0
		}
		if got := w.TrackMemories[0].itLoopCount; got != want || w.TrackMemories[0].itLoopStart != 3 {
			t.Fatalf("legacy=%v: loop count=%d start=%d", legacy, got, w.TrackMemories[0].itLoopStart)
		}
	}
}

func TestITPatternLoopPackedReuseAndValidation(t *testing.T) {
	pattern, err := unpackITPattern([]byte{
		0x81, 8, 19, 0xb0, 0,
		0x81, 8, 19, 0xb3, 0,
		0x81, 0x80, 0,
		0x81, 8, 19, 0, 0,
	}, 4)
	if err != nil {
		t.Fatal(err)
	}
	for row, want := range []uint8{0xb0, 0xb3, 0xb3, 0} {
		cmd := pattern.Rows[row][0]
		if cmd.EffectCmd != 0x15 || cmd.EffectArg != want {
			t.Fatalf("row %d: effect %02x%02x, want S%02x", row, cmd.EffectCmd, cmd.EffectArg, want)
		}
	}
}
