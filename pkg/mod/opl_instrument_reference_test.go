package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTInstrumentOnlyTransition(t *testing.T) {
	for _, mode := range []bool{false, true} {
		for _, firstOPL := range []bool{false, true} {
			t.Run(fmt.Sprintf("instrument=%v/opl=%v", mode, firstOPL), func(t *testing.T) {
				b, assets := transitionFixture(firstOPL, false, false)
				first, second := byte(1), byte(2)
				if firstOPL {
					first, second = second, first
				}
				packed := append([]byte{0x81, 3, 60, first, 0, 0x81, 2, second, 0}, make([]byte, 62)...)
				binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
				copy(b[344:], packed)
				if mode {
					b = b[:len(b)-4]
					b[44] |= 4
					binary.LittleEndian.PutUint16(b[34:], 2)
					binary.LittleEndian.PutUint32(b[202:], 256)
					binary.LittleEndian.PutUint32(b[206:], uint32(8704))
					binary.LittleEndian.PutUint32(b[210:], 336)
					for i := range 2 {
						binary.LittleEndian.PutUint32(b[194+i*4:], uint32(len(b)))
						h := make([]byte, 554)
						copy(h, "IMPI")
						h[17], h[24], h[25] = 1, 128, 128
						for j := range 120 {
							h[64+j*2], h[65+j*2] = byte(j), byte(i+1)
						}
						b = append(b, h...)
					}
					off := len(b)
					b = append(b, []byte("228mptm")...)
					b = binary.LittleEndian.AppendUint32(b, uint32(off))
					assets.ITInstruments = make([]ITInstrument, 2)
					for i := range assets.ITInstruments {
						assets.ITInstruments[i].GlobalVolume = 128
						assets.ITInstruments[i].NewNoteAction = 1
						for j := range 120 {
							assets.ITInstruments[i].Notes[j] = uint8(j)
							assets.ITInstruments[i].Samples[j] = uint8(i + 1)
						}
					}
				}
				ref := referenceTransition(t, b)
				rows := make([][]ModCommand, 64)
				for i := range rows {
					rows[i] = make([]ModCommand, 1)
				}
				rows[0][0] = ModCommand{Note: 61, Instrument: first}
				rows[1][0] = ModCommand{Instrument: second}
				assets.OrderList, assets.Patterns = []int{0}, []PatternAsset{{Rows: rows}}
				w := NewTracker(assets, referenceRate, 5760)
				w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 3, 125, -1
				actual := make([]float32, 5760*2)
				if got := w.Read(actual, 5760); got != 5760 {
					t.Fatalf("short render: %d", got)
				}
				want := 261.7
				if firstOPL {
					want = 522.7
				}
				if got := referenceTickFrequency(t, actual, 4); math.Abs(got/want-1) > .01 {
					t.Fatalf("after switch frequency %.2f, want %.2f", got, want)
				}
				if !firstOPL {
					if got := referenceTickFrequency(t, ref, 4); math.Abs(got/want-1) > .01 {
						t.Fatalf("OpenMPT frequency %.2f, want %.2f", got, want)
					}
				} else if transitionTone(ref, 261.7) < .01 || transitionTone(ref, 522.7) < .01 {
					t.Fatal("OpenMPT reference did not retain the old OPL note")
				}
				voice := w.TrackCursors[0].ActiveVoice
				if voice < 0 || w.VoiceSamples[voice].SampleID != int(second)-1 {
					t.Fatal("instrument-only switch did not select the new sample")
				}
				if firstOPL && w.oplVoices[0] != -1 {
					t.Fatal("old OPL voice was not released")
				}
			})
		}
	}
}
