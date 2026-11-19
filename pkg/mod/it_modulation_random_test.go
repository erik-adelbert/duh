package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITRandomModulation(t *testing.T) {
	for _, family := range []byte{0x40, 0x50} {
		for _, old := range []bool{false, true} {
			t.Run(fmt.Sprintf("S%02X/old=%t", family|3, old), func(t *testing.T) {
				effect := byte(18)
				if family == 0x50 {
					effect = 25
				}
				rows := []byte{0x81, 15, 60, 1, 32, 19, family | 3, 0}
				for range 10 {
					rows = append(rows, 0x81, 8, effect, 0x4f, 0)
				}
				b := testITPitchFixture(rows, 6, true, false)
				base := testITPitchFixture([]byte{0x81, 7, 60, 1, 32, 0}, 6, true, false)
				if old {
					b[44] |= 0x10
					base[44] |= 0x10
				}
				ref, actual := referencePCMWithChunks(t, b, "2", 840, 256)
				refBase, actualBase := referencePCMWithChunks(t, base, "2", 840, 256)
				for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
					baseline := refBase
					if name == "tracker" {
						baseline = actualBase
					}
					seenUp, seenDown := false, false
					var held float64
					for tick := 6; tick < 42; tick++ {
						ms := tick*20 + 19
						l := channelRMS(pcm, ms, ms+1, 0)
						var value float64
						if family == 0x40 {
							value = l/channelRMS(baseline, ms, ms+1, 0) - 1
						} else {
							// Measure near the ramp endpoint, where the held pan has settled.
							end := (tick + 1) * referenceRate / 50
							var left, right float64
							for frame := end - 6; frame < end; frame++ {
								left += float64(pcm[frame*2]) * float64(pcm[frame*2])
								right += float64(pcm[frame*2+1]) * float64(pcm[frame*2+1])
							}
							l, r := math.Sqrt(left), math.Sqrt(right)
							value = (r - l) / (r + l)
							if (tick-6)%4 == 0 {
								held = value
							} else if math.Abs(value-held) > .04 {
								t.Fatalf("%s tick %d: random panbrello hold changed from %.3f to %.3f", name, tick, held, value)
							}
						}
						if math.IsNaN(value) || value < -1.01 || value > 1.01 {
							t.Fatalf("%s tick %d: out-of-range value %v", name, tick, value)
						}
						seenUp = seenUp || value > .05
						seenDown = seenDown || value < -.05
					}
					if !seenUp || !seenDown {
						t.Fatalf("%s random waveform did not vary in both directions", name)
					}
				}
			})
		}
	}
}

func TestOpenMPTITWaveformSelectionWithVolumeVibrato(t *testing.T) {
	for _, arg := range []byte{0x41, 0x52} {
		for _, legacy := range []bool{false, true} {
			for _, old := range []bool{false, true} {
				t.Run(fmt.Sprintf("S%02X/legacy=%t/old=%t", arg, legacy, old), func(t *testing.T) {
					rows := []byte{0x81, 11, 60, 1, 8, 0x8f, 0,
						0x81, 12, 208, 19, arg, 0, 0x81, 4, 208, 0}
					b := testITPitchFixture(rows, 3, true, legacy)
					if old {
						b[44] |= 0x10
					}
					ref, actual := referencePCMWithChunks(t, b, "2", 180, 256)
					for tick := range 9 {
						want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
						if math.Abs(got/want-1) > .0015 {
							t.Errorf("tick %d: OpenMPT %.3f Hz, tracker %.3f Hz", tick, want, got)
						}
					}
				})
			}
		}
	}
}
