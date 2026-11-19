package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMPatternDelayNotes(t *testing.T) {
	tests := []struct {
		name        string
		before, row []byte
	}{
		{"new-note-same-channel", nil, []byte{0x81, 11, 60, 2, 19, 0xe2}},
		{"new-note-other-channel", nil, []byte{0x81, 8, 19, 0xe2, 0x83, 3, 60, 2}},
		{"note-memory", []byte{0x83, 3, 60, 2, 0}, []byte{0x81, 8, 19, 0xe2, 0x83, 1, 64}},
		{"new-note-and-global-slide", nil, []byte{0x81, 8, 19, 0xe2, 0x82, 8, 23, 0x01, 0x83, 3, 60, 2}},
	}
	for _, tt := range tests {
		for _, delays := range [][]byte{{0x63}, {0xe2}, {0xe2, 0x63}} {
			for _, speed := range []byte{1, 2, 3} {
				for _, firstTick := range []bool{false, true} {
					for _, nna := range []byte{0, 1} {
						t.Run(fmt.Sprintf("%s/S%X/speed=%d/first=%t/nna=%d", tt.name, delays, speed, firstTick, nna), func(t *testing.T) {
							b := mptResamplingFixture(t, 1, 60, false)
							b[50] = speed
							b[bytesIndex(t, b, "...C")+6] = 4
							flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITFirstTickHandling)/8
							mask := byte(1 << (mptITFirstTickHandling % 8))
							b[flag] &^= mask
							if firstTick {
								b[flag] |= mask
							}
							instrument := int(binary.LittleEndian.Uint32(b[198:]))
							b[instrument+17], b[instrument+304] = nna, 0
							sample := int(binary.LittleEndian.Uint32(b[202:]))
							data := int(binary.LittleEndian.Uint32(b[sample+72:]))
							for i := range 4096 {
								value := 15000 * (1 - float64(i)/4096) * math.Sin(2*math.Pi*float64(i)/32)
								binary.LittleEndian.PutUint16(b[data+i*2:], uint16(int16(value)))
							}
							row := append([]byte(nil), tt.row...)
							for i := range row {
								if i > 0 && row[i-1] == 19 && row[i] == 0xe2 {
									row[i] = delays[0]
									break
								}
							}
							if len(delays) == 2 {
								row = append(row, 0x84, 8, 19, delays[1])
							}
							rows := append(append([]byte(nil), tt.before...), row...)
							rows = append(rows, 0, 0)
							b = mptPatternRowsFixture(t, b, rows)
							ref, actual := mptSharedEffectRenderForDuration(t, b, 400)
							if rmsWindow(ref, 30, 35) < .001 {
								t.Fatal("reference fixture is unexpectedly silent")
							}
							for ms := 20; ms < 390; ms += 5 {
								// The initial note transition has different short ramping in OpenMPT.
								if len(tt.before) != 0 && ms == int(speed)*20 {
									continue
								}
								want, got := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
								if math.Abs(got-want) > .006 {
									t.Errorf("%dms: OpenMPT RMS %g tracker %g", ms, want, got)
								}
							}
						})
					}
				}
			}
		}
	}
}

func TestOpenMPTMPatternDelaySampleModeNote(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	flags &^= 1 << mptMODVBlankTiming
	b := mptSharedEffectFixture(t, []byte{
		0x81, 11, 60, 1, 19, 0xe2, 0x82, 8, 19, 0x63, 0,
		0x81, 1, 254, 0,
	}, flags)
	b[bytesIndex(t, b, "...C")+6] = 2
	ref, actual := mptSharedEffectRenderForDuration(t, b, 400)
	for tick := range 15 {
		want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
		if math.Abs(got/want-1) > .005 {
			t.Errorf("tick %d: OpenMPT %.2f Hz tracker %.2f Hz", tick, want, got)
		}
	}
	assertMPTPatternDelayChunking(t, b, referenceRate/3)
}
