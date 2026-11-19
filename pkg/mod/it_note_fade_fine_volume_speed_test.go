package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTITFadeFineVolumeSpeed(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, volume := range []byte{66, 75, 76} {
			for _, speed := range []byte{0, 1, 3, 0xe0} {
				t.Run(fmt.Sprintf("legacy=%t/volume=%d/A%02X", legacy, volume, speed), func(t *testing.T) {
					fixture := func(target byte, fade uint16) []byte {
						rows := []byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 77, 0,
							0x81, 13, 246, target, 1, speed, 0, 0x81, 3, 60, 1, 0}
						if target == 32 {
							rows = []byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 77, 0,
								0x81, 8, 1, speed, 0, 0x81, 3, 60, 1, 0}
						}
						b := testITInstrumentWithRows(rows)
						b[50], b[44] = 3, b[44]|8
						if legacy {
							binary.LittleEndian.PutUint16(b[40:], 0x217)
							binary.LittleEndian.PutUint16(b[42:], 0x200)
						}
						h := b[len(b)-554:]
						h[17], h[304] = 0, 0
						binary.LittleEndian.PutUint16(h[20:], fade)
						copy(b[512:512+8192], testITPitchFixture(nil, 3, true, legacy)[512:512+8192])
						binary.LittleEndian.PutUint32(b[256+60:], 24000)
						binary.LittleEndian.PutUint32(b[256+56:], 3840)
						return b
					}
					effectiveSpeed := int(speed)
					if effectiveSpeed == 0 {
						effectiveSpeed = 3
					}
					onset := 120 + effectiveSpeed*20
					ref, actual := referencePCMWithChunks(t, fixture(volume, 32), "2", onset+40, 97)
					refNoFade, actualNoFade := referencePCMWithChunks(t, fixture(volume, 0), "2", 240, 97)
					refNoSlide, _ := referencePCMWithChunks(t, fixture(32, 0), "2", 240, 97)
					_, whole := referencePCMWithChunks(t, fixture(volume, 32), "2", onset+40, 0)
					for i := range actual {
						if actual[i] != whole[i] {
							t.Fatalf("chunk-dependent PCM at sample %d", i)
						}
					}
					for _, ms := range []int{135, 155} {
						if ms+5 >= onset {
							continue
						}
						rn, an := rmsWindow(refNoFade, ms, ms+5), rmsWindow(actualNoFade, ms, ms+5)
						if rn < .001 || an < .001 {
							t.Fatal("silent comparison baseline")
						}
						want, got := rmsWindow(ref, ms, ms+5)/rn, rmsWindow(actual, ms, ms+5)/an
						if math.IsNaN(got) || math.Abs(got-want) > .05 {
							t.Errorf("%dms fade: OpenMPT %.3f, tracker %.3f", ms, want, got)
						}
						if ms == 135 && math.Abs(rn/rmsWindow(refNoSlide, ms, ms+5)-1) < .02 {
							t.Error("reference fine slide had no measurable effect")
						}
					}
					if speed == 0xe0 {
						for _, renderer := range []struct {
							name string
							pcm  []float32
						}{{"OpenMPT", ref}, {"tracker", actual}} {
							if rmsWindow(renderer.pcm, onset-100, onset-20) > .001 ||
								rmsWindow(renderer.pcm, onset+10, onset+30) < .005 {
								t.Errorf("%s: wrong timing for A%02X row boundary at %dms", renderer.name, speed, onset)
							}
						}
					}
				})
			}
		}
	}
}

func TestITFadeFineVolumeSpeedValidation(t *testing.T) {
	for _, tt := range []struct {
		name string
		row  []byte
		ok   bool
	}{
		{"AE0", []byte{0x81, 13, 246, 76, 1, 0xe0, 0}, true},
		{"A00", []byte{0x81, 13, 246, 65, 1, 0, 0}, true},
		{"instrument", []byte{0x81, 15, 246, 1, 76, 1, 0xe0, 0}, false},
		{"normal slide", []byte{0x81, 13, 246, 99, 1, 0xe0, 0}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadIT(testITInstrumentWithRows(tt.row))
			if tt.ok && err != nil || !tt.ok && (err == nil || !strings.Contains(err.Error(), "note fade")) {
				t.Fatalf("got %v", err)
			}
		})
	}
	if _, err := loadMPTM(mptS70Fixture(t, []byte{0x81, 13, 253, 76, 1, 0xe0, 0})); err == nil || !strings.Contains(err.Error(), "note fade") {
		t.Errorf("MPTM combination: %v", err)
	}
}
