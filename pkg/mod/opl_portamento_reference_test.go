package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTOPLPortamentoPatchSwap(t *testing.T) {
	b, assets := transitionFixture(false, true, false)
	first := testOPLPatch
	first[3] = 15
	start := len(b) - 4
	extension := binary.LittleEndian.Uint32(b[start:])
	b = b[:start]
	b[256+18], b[256+46] = 1, 0x40
	binary.LittleEndian.PutUint32(b[256+48:], 12)
	binary.LittleEndian.PutUint32(b[256+72:], uint32(len(b)))
	b = append(b, first[:]...)
	b = binary.LittleEndian.AppendUint32(b, extension)
	assets.Samples[0] = SampleAsset{opl: &first, C5Speed: 8363, Volume: 64, ITGlobalVolume: 64}
	rows := make([][]ModCommand, 64)
	for i := range rows {
		rows[i] = make([]ModCommand, 1)
	}
	rows[0][0] = ModCommand{Note: 61, Instrument: 1}
	rows[1][0] = ModCommand{Note: 73, Instrument: 2, EffectCmd: 3, EffectArg: 16}
	assets.OrderList, assets.Patterns = []int{0}, []PatternAsset{{Rows: rows}}
	w := NewTracker(assets, referenceRate, 5760)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 3, 125, -1
	actual := make([]float32, 5760*2)
	if got := w.Read(actual, 5760); got != 5760 {
		t.Fatalf("short render: %d", got)
	}
	ref := referenceTransition(t, b)
	for _, tick := range []int{1, 3, 4, 5} {
		want := referenceTickFrequency(t, ref, tick)
		got := referenceTickFrequency(t, actual, tick)
		if math.Abs(got/want-1) > .015 {
			t.Errorf("tick %d: tracker %.2f Hz, OpenMPT %.2f Hz", tick, got, want)
		}
	}
}
