package mod

import (
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
)

func itOrderFixture(orders []byte) []byte {
	b := testIT()
	pointers := append([]byte(nil), b[194:202]...)
	binary.LittleEndian.PutUint16(b[32:], uint16(len(orders)))
	copy(b[192:], orders)
	copy(b[192+len(orders):], pointers)
	return b
}

func TestLoadITOrderSkips(t *testing.T) {
	b := itOrderFixture([]byte{254, 0, 254, 254, 0, 254, 255})
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{itOrderSkip, 0, itOrderSkip, itOrderSkip, 0, itOrderSkip}; !reflect.DeepEqual(loaded.Assets.OrderList, want) {
		t.Fatalf("orders = %v, want %v", loaded.Assets.OrderList, want)
	}
	for _, orders := range [][]byte{{254, 254}, {254, 255}, {255}} {
		if _, err := loadIT(itOrderFixture(orders)); err == nil || !strings.Contains(err.Error(), "no playable orders") {
			t.Errorf("orders %v: got %v, want no playable orders", orders, err)
		}
	}
	if _, err := loadIT(itOrderFixture([]byte{254, 1, 255})); err == nil || !strings.Contains(err.Error(), "unsupported IT order 1") {
		t.Fatalf("invalid pattern order: %v", err)
	}
}

func TestITOrderSkipsPlaybackFlow(t *testing.T) {
	rows := func(n int) [][]ModCommand {
		result := make([][]ModCommand, n)
		for i := range result {
			result[i] = make([]ModCommand, 1)
		}
		return result
	}
	newWorld := func(orders []int) *Tracker {
		w := NewTracker(SongAssets{
			IT: true, NumTracks: 1, OrderList: orders,
			Patterns: []PatternAsset{{Rows: rows(1)}, {Rows: rows(4)}},
		}, 48000, 1)
		w.Clock.Speed, w.Clock.Tick = 1, -1
		return w
	}
	t.Run("initial and consecutive skips", func(t *testing.T) {
		w := newWorld([]int{itOrderSkip, 0, itOrderSkip, itOrderSkip, 1, itOrderSkip})
		if !TickSchedulerSystem(w) || w.Clock.Order != 1 || w.Clock.Row != 0 {
			t.Fatalf("initial position = %d:%d", w.Clock.Order, w.Clock.Row)
		}
		w.Clock.SamplesRemaining = 0
		if !TickSchedulerSystem(w) || w.Clock.Order != 4 || w.Clock.Row != 0 {
			t.Fatalf("next position = %d:%d", w.Clock.Order, w.Clock.Row)
		}
		w.Clock.Row = 3
		w.Clock.SamplesRemaining = 0
		if TickSchedulerSystem(w) || !w.Clock.SongFinished {
			t.Fatal("trailing skip did not end song")
		}
	})
	for _, tt := range []struct {
		name       string
		jump       int
		breakRow   int
		wantOrder  int
		wantRow    int
		wantFinish bool
	}{
		{"break over skips", -1, 2, 3, 2, false},
		{"jump to skip preserves order numbers", 2, -1, 3, 0, false},
		{"jump and break to skip", 1, 3, 3, 3, false},
		{"break past pattern end", 1, 8, 3, 0, false},
		{"jump to final skip", 4, 0, 5, 0, true},
		{"jump beyond end", 8, 0, 8, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld([]int{0, itOrderSkip, itOrderSkip, 1, itOrderSkip})
			w.Assets.NumTracks = 2
			w.Clock.Tick = 0
			if tt.jump >= 0 {
				w.PendingCmds[0] = ModCommand{EffectCmd: 0x0b, EffectArg: uint8(tt.jump)}
			}
			if tt.breakRow >= 0 {
				w.PendingCmds[1] = ModCommand{EffectCmd: 0x0d, EffectArg: uint8(tt.breakRow)}
			}
			GlobalFlowSystem(w)
			w.Clock.SamplesRemaining = 0
			ok := TickSchedulerSystem(w)
			if ok == tt.wantFinish || w.Clock.SongFinished != tt.wantFinish || w.Clock.Order != tt.wantOrder || w.Clock.Row != tt.wantRow {
				t.Fatalf("playing=%v finished=%v position=%d:%d, want finished=%v position=%d:%d",
					ok, w.Clock.SongFinished, w.Clock.Order, w.Clock.Row, tt.wantFinish, tt.wantOrder, tt.wantRow)
			}
		})
	}
}

func TestOpenMPTITOrderSkips(t *testing.T) {
	b := itOrderFixture([]byte{254, 0, 254, 254, 0, 254, 255})
	b[50] = 1
	b[256+18] |= 0x10
	binary.LittleEndian.PutUint32(b[256+56:], 3840)
	packed := []byte{0x81, 3, 60, 1, 0, 0x81, 1, 254, 0, 0, 0}
	binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
	binary.LittleEndian.PutUint16(b[338:], 4)
	copy(b[344:], packed)
	ref, actual := referencePCM(t, b)
	for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
		for _, window := range []struct {
			start, end int
			audible    bool
		}{{5, 15, true}, {45, 70, false}, {85, 95, true}, {105, 115, false}} {
			level := channelRMS(pcm, window.start, window.end, 0)
			if window.audible && level < .001 || !window.audible && level > .001 {
				t.Errorf("%s %d-%dms RMS %g, audible=%v", name, window.start, window.end, level, window.audible)
			}
		}
	}
}

func TestOpenMPTITJumpAndBreakAcrossSkips(t *testing.T) {
	b := itOrderFixture([]byte{0, 254, 254, 1, 255})
	b[50] = 1
	b[256+18] |= 0x10
	binary.LittleEndian.PutUint32(b[256+56:], 3840)
	binary.LittleEndian.PutUint16(b[38:], 2)
	binary.LittleEndian.PutUint32(b[192+5+8:], uint32(len(b)))
	first := []byte{0x81, 8, 2, 1, 0x82, 8, 3, 2, 0}
	binary.LittleEndian.PutUint16(b[336:], uint16(len(first)))
	binary.LittleEndian.PutUint16(b[338:], 1)
	copy(b[344:], first)
	second := []byte{0, 0, 0x81, 3, 60, 1, 0, 0x81, 1, 254, 0, 0, 0, 0, 0}
	header := make([]byte, 8)
	binary.LittleEndian.PutUint16(header, uint16(len(second)))
	binary.LittleEndian.PutUint16(header[2:], 8)
	b = append(b, header...)
	b = append(b, second...)
	ref, actual := referencePCM(t, b)
	for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
		for _, window := range []struct {
			start, end int
			audible    bool
		}{{5, 15, false}, {25, 35, true}, {45, 70, false}} {
			level := channelRMS(pcm, window.start, window.end, 0)
			if window.audible && level < .001 || !window.audible && level > .001 {
				t.Errorf("%s %d-%dms RMS %g, audible=%v", name, window.start, window.end, level, window.audible)
			}
		}
	}
}
