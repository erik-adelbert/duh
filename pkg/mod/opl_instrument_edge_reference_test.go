package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTInstrumentOnlyAfterCut(t *testing.T) {
	b, assets := transitionFixture(false, false, false)
	b[50] = 2
	packed := append([]byte{0x81, 3, 60, 1, 0, 0x81, 1, 254, 0, 0x81, 2, 2, 0}, make([]byte, 61)...)
	binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
	copy(b[344:], packed)
	ref := referenceTransition(t, b)
	rows := make([][]ModCommand, 64)
	for i := range rows {
		rows[i] = make([]ModCommand, 1)
	}
	rows[0][0] = ModCommand{Note: 61, Instrument: 1}
	rows[1][0] = ModCommand{Note: 254}
	rows[2][0] = ModCommand{Instrument: 2}
	assets.OrderList, assets.Patterns = []int{0}, []PatternAsset{{Rows: rows}}
	w := NewTracker(assets, referenceRate, 5760)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 2, 125, -1
	actual := make([]float32, 5760*2)
	if got := w.Read(actual, 5760); got != 5760 {
		t.Fatalf("short render: %d", got)
	}
	for _, pcm := range [][]float32{ref, actual} {
		if channelRMS(pcm, 60, 70, 0) > 1e-4 {
			t.Fatal("cut interval was not silent")
		}
		if channelRMS(pcm, 100, 110, 0) < .001 {
			t.Fatal("instrument-only switch after cut did not restart playback")
		}
		if got := referenceTickFrequency(t, pcm, 5); math.Abs(got/261.7-1) > .01 {
			t.Fatalf("frequency after switch = %.2f Hz", got)
		}
	}
}
