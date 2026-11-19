package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestITRetriggerOpenMPT(t *testing.T) {
	for mode := range byte(16) {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("Q%X2/legacy=%t", mode, legacy), func(t *testing.T) {
				rows := []byte{0x81, 15, 60, 1, 32, 17, mode<<4 | 2, 0,
					0x81, 8, 17, 0, 0}
				b := testITPitchFixture(rows, 6, false, legacy)
				binary.LittleEndian.PutUint32(b[256+60:], 48000)
				for i := range 4096 {
					var sample int16
					if i < 700 {
						sample = 16000
					}
					binary.LittleEndian.PutUint16(b[512+i*2:], uint16(sample))
				}
				ref, actual := referencePCMWithChunks(t, b, "1", 240, 256)
				refBase, actualBase := rmsWindow(ref, 8, 10), rmsWindow(actual, 8, 10)
				for tick := range 12 {
					want := rmsWindow(ref, tick*20+8, tick*20+10) / refBase
					got := rmsWindow(actual, tick*20+8, tick*20+10) / actualBase
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .13 {
						t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
					}
				}
			})
		}
	}
}

func TestITRetriggerMemoryAndTiming(t *testing.T) {
	for _, arg := range []byte{0x00, 0x04, 0x80, 0x83, 0xff} {
		p, err := unpackITPattern([]byte{0x81, 8, 17, arg, 0, 0x81, 0x80, 0}, 2)
		if err != nil {
			t.Fatalf("Q%02X: %v", arg, err)
		}

		for row := range p.Rows {
			if p.Rows[row][0].EffectCmd != 0x1b || p.Rows[row][0].EffectArg != arg {
				t.Fatalf("Q%02X row %d: %+v", arg, row, p.Rows[row][0])
			}
		}
	}
	w := NewTracker(SongAssets{IT: true, NumTracks: 2}, referenceRate, 1)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x1b, EffectArg: 4}
	PitchEffectSystem(w)
	if w.TrackMemories[0].LastRetrig != 4 || w.TrackMemories[1].LastRetrig != 0 {
		t.Fatal("Qxx memory is not isolated per track")
	}
	w.PendingCmds[0].EffectArg = 0
	PitchEffectSystem(w)
	if w.TrackMemories[0].LastRetrig != 4 || w.TrackMemories[0].RetrigCount != 3 {
		t.Fatal("Q00 did not recall interval")
	}
}

func TestITRetriggerInstrumentEnvelopeOpenMPT(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, action := range []byte{0, 1, 2, 3} {
			t.Run(fmt.Sprintf("legacy=%t/nna=%d", legacy, action), func(t *testing.T) {
				b := testITInstrument()
				b[50] = 6
				h := b[len(b)-554:]
				h[17] = action
				setTestEnvelope(h, 304, 1, []int8{64, 32}, []uint16{0, 20})
				for i := range 4096 {
					binary.LittleEndian.PutUint16(b[512+i*2:], 12000)
				}
				if legacy {
					binary.LittleEndian.PutUint16(b[40:], 0x217)
					binary.LittleEndian.PutUint16(b[42:], 0x200)
				}
				rows := []byte{0x81, 11, 60, 1, 17, 0x92, 0, 0x81, 8, 17, 0, 0}
				packed := append(rows, make([]byte, 64)...)
				binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
				copy(b[344:], packed)
				ref, actual := referencePCMWithChunks(t, b, "1", 240, 256)
				refBase, actualBase := channelRMS(ref, 19, 20, 0), channelRMS(actual, 19, 20, 0)
				for tick := range 12 {
					start := tick*20 + 19
					want := channelRMS(ref, start, start+1, 0) / refBase
					got := channelRMS(actual, start, start+1, 0) / actualBase
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .12 {
						t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
					}
				}
			})
		}
	}
}

func TestITRetriggerIntervalsOpenMPT(t *testing.T) {
	tests := []struct {
		name  string
		speed byte
		rows  []byte
	}{
		{"Q04 then memory", 6, []byte{0x81, 11, 60, 1, 17, 4, 0, 0x81, 8, 17, 0, 0}},
		{"initial Q00", 6, []byte{0x81, 11, 60, 1, 17, 0, 0, 0x81, 8, 17, 0, 0}},
		{"Q80 zero interval", 6, []byte{0x81, 11, 60, 1, 17, 0x80, 0, 0x81, 8, 17, 0, 0}},
		{"interrupted", 2, []byte{0x81, 11, 60, 1, 17, 4, 0, 0, 0x81, 8, 17, 0, 0}},
		{"speed one", 1, []byte{0x81, 11, 60, 1, 17, 2, 0, 0x81, 8, 17, 0, 0, 0x81, 8, 17, 0, 0}},
		{"volume column", 6, []byte{0x81, 15, 60, 1, 40, 17, 0x92, 0, 0x81, 12, 32, 17, 0, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%t", tt.name, legacy), func(t *testing.T) {
				b := testITPitchFixture(tt.rows, tt.speed, false, legacy)
				binary.LittleEndian.PutUint32(b[256+60:], 48000)
				for i := range 4096 {
					var sample int16
					if i < 700 {
						sample = 16000
					}
					binary.LittleEndian.PutUint16(b[512+i*2:], uint16(sample))
				}
				ref, actual := referencePCMWithChunks(t, b, "1", 240, 256)
				refBase, actualBase := rmsWindow(ref, 8, 10), rmsWindow(actual, 8, 10)
				for tick := range 12 {
					want := rmsWindow(ref, tick*20+8, tick*20+10) / refBase
					got := rmsWindow(actual, tick*20+8, tick*20+10) / actualBase
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .13 {
						t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
					}
				}
			})
		}
	}
}
