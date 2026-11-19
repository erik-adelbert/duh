package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTMQC2(t *testing.T) {
	for _, compatible := range []bool{false, true} {
		for _, initialVolume := range []byte{24, 59} {
			t.Run(fmt.Sprintf("compatible=%t/volume=%d", compatible, initialVolume), func(t *testing.T) {
				rows := []byte{0x81, 15, 60, 2, initialVolume, 17, 0xc2, 0, 0x81, 8, 17, 0, 0}
				b := mptS70Fixture(t, rows)
				b[50] = 6
				flags := bytesIndex(t, b, ".FSM") + 6
				if !compatible {
					b[flags+2] &^= 0x80
				}
				sample := int(binary.LittleEndian.Uint32(b[202:]))
				b[sample+18] &^= 0x10
				binary.LittleEndian.PutUint32(b[sample+60:], 48000)
				pcm := int(binary.LittleEndian.Uint32(b[sample+72:]))
				for i := range 4096 {
					var value uint16
					if i < 700 {
						value = 16000
					}
					binary.LittleEndian.PutUint16(b[pcm+i*2:], value)
				}
				ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
				refBase, actualBase := rmsWindow(ref, 8, 10), rmsWindow(actual, 8, 10)
				if refBase < .001 || actualBase < .001 {
					t.Fatal("silent comparison baseline")
				}
				for tick := range 12 {
					want := rmsWindow(ref, tick*20+8, tick*20+10) / refBase
					got := rmsWindow(actual, tick*20+8, tick*20+10) / actualBase
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .10 {
						t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
					}
					if tick == 2 {
						expected := math.Min(float64(initialVolume)+8, 64) / float64(initialVolume)
						if math.Abs(want-expected) > .05 {
							t.Fatalf("reference volume ratio %.3f, expected %.3f", want, expected)
						}
					}
				}
			})
		}
	}
}

func TestOpenMPTMQC2OPL(t *testing.T) {
	for _, compatible := range []bool{false, true} {
		t.Run(fmt.Sprintf("compatible=%t", compatible), func(t *testing.T) {
			b := mptS70Fixture(t, []byte{0x81, 15, 60, 2, 24, 17, 0xc2, 0, 0x81, 8, 17, 0, 0})
			b[50] = 6
			flags := bytesIndex(t, b, ".FSM") + 6
			if !compatible {
				b[flags+2] &^= 0x80
			}
			instrument := int(binary.LittleEndian.Uint32(b[198:]))
			for note := range 120 {
				b[instrument+65+note*2] = 2
			}
			sample := int(binary.LittleEndian.Uint32(b[206:]))
			patch := int(binary.LittleEndian.Uint32(b[sample+72:]))
			copy(b[patch:patch+12], percussiveOPLPatch[:])
			ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
			refBase, actualBase := rmsWindow(ref, 5, 12), rmsWindow(actual, 5, 12)
			if refBase < .00001 || actualBase < .00001 {
				t.Fatal("silent comparison baseline")
			}
			for tick := 2; tick < 10; tick += 2 {
				want, got := rmsWindow(ref, tick*20+5, tick*20+12)/refBase, rmsWindow(actual, tick*20+5, tick*20+12)/actualBase
				if math.IsNaN(want) || math.IsNaN(got) || want < .1 || math.Abs(got-want) > .25 {
					t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
				}
			}
		})
	}
}

func TestOpenMPTMQC2ShortSample(t *testing.T) {
	for _, shortSampleRetrig := range []bool{false, true} {
		t.Run(fmt.Sprintf("shortSampleRetrig=%t", shortSampleRetrig), func(t *testing.T) {
			b := mptS70Fixture(t, []byte{0x81, 15, 60, 2, 24, 17, 0xc2, 0})
			b[50] = 6
			flags := bytesIndex(t, b, ".FSM") + 6
			if shortSampleRetrig {
				b[flags+4] |= 1 << (uint(mptITShortSampleRetrig) % 8)
			} else {
				b[flags+4] &^= 1 << (uint(mptITShortSampleRetrig) % 8)
			}
			sample := int(binary.LittleEndian.Uint32(b[202:]))
			b[sample+18] &^= 0x10
			binary.LittleEndian.PutUint32(b[sample+48:], 400)
			binary.LittleEndian.PutUint32(b[sample+60:], 48000)
			loaded, err := loadMPTM(b)
			if err != nil {
				t.Fatal(err)
			}
			warned := false
			for _, warning := range loaded.Warnings {
				warned = warned || strings.Contains(warning, "Qxx cannot restart")
			}
			if warned == shortSampleRetrig {
				t.Fatalf("short-sample warning = %t", warned)
			}
			ref, actual := mptSharedEffectRenderForDuration(t, b, 115)
			baseRef, baseActual := rmsWindow(ref, 3, 5), rmsWindow(actual, 3, 5)
			if baseRef < .001 || baseActual < .001 {
				t.Fatal("silent comparison baseline")
			}
			want := rmsWindow(ref, 43, 45) / baseRef
			got := rmsWindow(actual, 43, 45) / baseActual
			if !shortSampleRetrig {
				if want < .1 {
					t.Fatalf("OpenMPT did not restart ended sample: %.3f", want)
				}
				return
			}
			if want > .01 {
				t.Fatalf("OpenMPT unexpectedly restarted ended sample: %.3f", want)
			}
			if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .15 {
				t.Fatalf("retrigger after sample end: OpenMPT %.3f tracker %.3f", want, got)
			}
		})
	}
}
