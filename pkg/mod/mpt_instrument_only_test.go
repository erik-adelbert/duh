package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTInstrumentOnlyAfterPortamento(t *testing.T) {
	b, assets := sameFormatInstrumentFixture(false)
	b = sameFormatInstrumentMode(b, &assets)
	b[50] = 2
	instrument := int(binary.LittleEndian.Uint32(b[194:]))
	b[instrument+65+72*2] = 2
	assets.ITInstruments[0].Samples[72] = 2
	rows := []byte{0x81, 3, 60, 1, 0, 0x81, 9, 72, 7, 0, 0, 0x81, 2, 1, 0}
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	packed := append(rows, make([]byte, 61)...)
	binary.LittleEndian.PutUint16(b[pattern:], uint16(len(packed)))
	copy(b[pattern+8:], packed)
	ref := referenceTransition(t, b)

	patternRows := make([][]ModCommand, 64)
	for i := range patternRows {
		patternRows[i] = make([]ModCommand, 1)
	}
	patternRows[0][0] = ModCommand{Note: 61, Instrument: 1}
	patternRows[1][0] = ModCommand{Note: 73, EffectCmd: 3}
	patternRows[2][0] = ModCommand{Instrument: 1}
	assets.Patterns, assets.OrderList = []PatternAsset{{Rows: patternRows}}, []int{0}
	assets.ITCompatibility.oplInstrumentSwitch = true
	assets.ITCompatibility.mptMultiSampleInstrument = true
	w := NewTracker(assets, referenceRate, 5760)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 2, 125, -1
	actual := make([]float32, 5760*2)
	if w.Read(actual, 5760) != 5760 {
		t.Fatal("short render")
	}
	for _, pcm := range [][]float32{ref, actual} {
		before, after := rmsWindow(pcm, 50, 70), rmsWindow(pcm, 90, 110)
		if before < .001 || math.Abs(after/before-.5) > .08 {
			t.Fatalf("instrument-only volume ratio %g, expected 0.5", after/before)
		}
	}
	voice := w.TrackCursors[0].ActiveVoice
	if voice < 0 || w.VoiceSamples[voice].SampleID != 0 {
		t.Fatal("instrument-only row changed the playing sample")
	}
}

func TestMPTInstrumentOnlyParsing(t *testing.T) {
	b := oplEnvelopeEndFixture(t, 0x01280100, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}, []byte{0})
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	copy(b[pattern+8:], []byte{0x81, 2, 1, 0})
	if _, err := loadITData(b, true); err != nil {
		t.Fatalf("MPTM instrument-only row: %v", err)
	}
}
