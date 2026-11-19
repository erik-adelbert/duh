package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func sameFormatInstrumentFixture(opl bool) ([]byte, SongAssets) {
	b, assets := transitionFixture(false, false, false)
	first, second := 256, 8704
	if opl {
		b[first+18], b[first+46] = 1, 0x40
		binary.LittleEndian.PutUint32(b[first+48:], 12)
		copy(b[512:], testOPLPatch[:])
		firstPatch := testOPLPatch
		assets.Samples[0] = SampleAsset{opl: &firstPatch, C5Speed: 8363, Volume: 64, ITGlobalVolume: 64}
		secondPatch := testOPLPatch
		secondPatch[3] = 8
		copy(b[8784:], secondPatch[:])
		assets.Samples[1].opl = &secondPatch
		b[second+19] = 32
		assets.Samples[1].Volume = 32
	} else {
		b[second+18], b[second+46], b[second+19] = 0x13, 1, 32
		binary.LittleEndian.PutUint32(b[second+48:], 4096)
		binary.LittleEndian.PutUint32(b[second+56:], 3840)
		end := len(b) - 4
		extension := binary.LittleEndian.Uint32(b[end:])
		b = b[:end]
		binary.LittleEndian.PutUint32(b[second+72:], uint32(len(b)))
		pcm := make([]float32, 4096)
		for i := range pcm {
			v := int16(math.Sin(float64(i)*2*math.Pi/8) * 16000)
			pcm[i] = float32(v) / 32768
			b = binary.LittleEndian.AppendUint16(b, uint16(v))
		}
		b = binary.LittleEndian.AppendUint32(b, extension)
		assets.Samples[1] = SampleAsset{Data: pcm, LoopEnable: true, LoopEnd: 3840, C5Speed: 8363, Volume: 32, ITGlobalVolume: 64}
	}
	packed := append([]byte{0x81, 3, 60, 1, 0, 0x81, 2, 2, 0}, make([]byte, 62)...)
	binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
	copy(b[344:], packed)
	return b, assets
}

func sameFormatInstrumentMode(b []byte, assets *SongAssets) []byte {
	b = b[:len(b)-4]
	b[44] |= 4
	binary.LittleEndian.PutUint16(b[34:], 2)
	binary.LittleEndian.PutUint32(b[202:], 256)
	binary.LittleEndian.PutUint32(b[206:], 8704)
	binary.LittleEndian.PutUint32(b[210:], 336)
	assets.ITInstruments = make([]ITInstrument, 2)
	for i := range 2 {
		binary.LittleEndian.PutUint32(b[194+i*4:], uint32(len(b)))
		h := make([]byte, 554)
		copy(h, "IMPI")
		h[17], h[24], h[25] = 1, 128, 128
		instrument := &assets.ITInstruments[i]
		instrument.NewNoteAction, instrument.GlobalVolume = 1, 128
		for j := range 120 {
			h[64+j*2], h[65+j*2] = byte(j), byte(i+1)
			instrument.Notes[j], instrument.Samples[j] = uint8(j), uint8(i+1)
		}
		b = append(b, h...)
	}
	offset := len(b)
	b = append(b, []byte("228mptm")...)
	return binary.LittleEndian.AppendUint32(b, uint32(offset))
}

func TestOpenMPTSameFormatInstrumentSwitch(t *testing.T) {
	for _, opl := range []bool{false, true} {
		for _, differentSpeed := range []bool{false, true} {
			for _, instrumentMode := range []bool{false, true} {
				t.Run(fmt.Sprintf("opl=%v/different-speed=%v/instruments=%v", opl, differentSpeed, instrumentMode), func(t *testing.T) {
					b, assets := sameFormatInstrumentFixture(opl)
					if instrumentMode {
						b = sameFormatInstrumentMode(b, &assets)
					}
					if differentSpeed {
						binary.LittleEndian.PutUint32(b[8704+60:], 16726)
						assets.Samples[1].C5Speed = 16726
					}
					ref := referenceTransition(t, b)
					rows := make([][]ModCommand, 64)
					for i := range rows {
						rows[i] = make([]ModCommand, 1)
					}
					rows[0][0] = ModCommand{Note: 61, Instrument: 1}
					rows[1][0] = ModCommand{Instrument: 2}
					assets.OrderList, assets.Patterns = []int{0}, []PatternAsset{{Rows: rows}}
					w := NewTracker(assets, referenceRate, 5760)
					w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 3, 125, -1
					actual := make([]float32, 5760*2)
					if got := w.Read(actual, 5760); got != 5760 {
						t.Fatalf("short render: %d", got)
					}
					wantFreq, gotFreq := referenceTickFrequency(t, ref, 4), referenceTickFrequency(t, actual, 4)
					if math.Abs(gotFreq/wantFreq-1) > .015 {
						t.Errorf("frequency OpenMPT %.2f Hz, tracker %.2f Hz", wantFreq, gotFreq)
					}
					wantRatio := channelRMS(ref, 90, 110, 0) / channelRMS(ref, 30, 50, 0)
					gotRatio := channelRMS(actual, 90, 110, 0) / channelRMS(actual, 30, 50, 0)
					if !opl {
						var dot, p, q float64
						for i := 80 * 96; i < 100*96; i += 2 {
							x, y := float64(ref[i]), float64(actual[i])
							dot, p, q = dot+x*y, p+x*x, q+y*y
						}
						if correlation := dot / math.Sqrt(p*q); correlation < .99 {
							t.Fatalf("PCM waveform correlation after switch = %.5f", correlation)
						}
					}
					if math.Abs(gotRatio-wantRatio) > .08 {
						t.Errorf("volume ratio OpenMPT %.3f, tracker %.3f", wantRatio, gotRatio)
					}
					v := w.TrackCursors[0].ActiveVoice
					if v < 0 || w.VoiceSamples[v].SampleID != 1 || (w.VoiceSamples[v].oplChannel != 0) != opl {
						t.Fatal("instrument-only switch did not select the new sample")
					}
					if opl && w.opl.patches[w.VoiceSamples[v].oplChannel-1] != *assets.Samples[1].opl {
						t.Fatal("instrument-only switch did not apply the new OPL patch")
					}
				})
			}
		}
	}
}

func TestOpenMPTSameFormatInstrumentAfterCut(t *testing.T) {
	for _, opl := range []bool{false, true} {
		for _, instrument := range []byte{1, 2} {
			t.Run(fmt.Sprintf("opl=%v/instrument=%d", opl, instrument), func(t *testing.T) {
				b, assets := sameFormatInstrumentFixture(opl)
				b[50] = 2
				packed := append([]byte{0x81, 3, 60, 1, 0, 0x81, 1, 254, 0, 0x81, 2, instrument, 0}, make([]byte, 61)...)
				binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
				copy(b[344:], packed)
				ref := referenceTransition(t, b)
				rows := make([][]ModCommand, 64)
				for i := range rows {
					rows[i] = make([]ModCommand, 1)
				}
				rows[0][0] = ModCommand{Note: 61, Instrument: 1}
				rows[1][0] = ModCommand{Note: 254}
				rows[2][0] = ModCommand{Instrument: instrument}
				assets.OrderList, assets.Patterns = []int{0}, []PatternAsset{{Rows: rows}}
				w := NewTracker(assets, referenceRate, 5760)
				w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 2, 125, -1
				actual := make([]float32, 5760*2)
				if w.Read(actual, 5760) != 5760 {
					t.Fatal("short render")
				}
				for _, pcm := range [][]float32{ref, actual} {
					if channelRMS(pcm, 60, 70, 0) > .0001 || channelRMS(pcm, 100, 110, 0) < .001 {
						t.Fatal("cut interval or restarted instrument incorrect")
					}
				}
				wantFreq, gotFreq := referenceTickFrequency(t, ref, 5), referenceTickFrequency(t, actual, 5)
				if math.Abs(gotFreq/wantFreq-1) > .015 {
					t.Fatalf("frequency OpenMPT %.2f, tracker %.2f", wantFreq, gotFreq)
				}
			})
		}
	}
}
