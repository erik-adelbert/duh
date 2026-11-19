package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITResonanceMacros(t *testing.T) {
	for _, instrument := range []bool{false, true} {
		for _, extended := range []bool{false, true} {
			for _, timing := range []string{"before", "same", "after"} {
				for resonance := byte(0x80); resonance <= 0x8f; resonance++ {
					for _, cutoff := range []byte{64, 127} {
						t.Run(fmt.Sprintf("instrument=%t/extended=%t/%s/Z%02X/cutoff=%d", instrument, extended, timing, resonance, cutoff), func(t *testing.T) {
							fixture := func(arg byte) []byte {
								var b []byte
								if instrument {
									b = testITInstrument()
									b[len(b)-554+304] = 0
								} else {
									b = testIT()
								}
								b[50] = 2
								if extended {
									b[45] |= 0x10
								}
								b[256+18] |= 0x10
								binary.LittleEndian.PutUint32(b[256+56:], 3840)
								for i := range 4096 {
									v := int16(math.Sin(float64(i)*2*math.Pi*2000/48000) * 16000)
									binary.LittleEndian.PutUint16(b[512+i*2:], uint16(v))
								}
								rows := []byte{0x81, 8, 26, cutoff, 0, 0x81, 8, 26, arg, 0, 0x81, 3, 60, 1, 0}
								switch timing {
								case "same":
									rows = []byte{0x81, 8, 26, cutoff, 0, 0x81, 11, 60, 1, 26, arg, 0}
								case "after":
									rows = []byte{0x81, 11, 60, 1, 26, cutoff, 0, 0x81, 8, 26, arg, 0}
								}
								packed := append(rows, make([]byte, 64)...)
								binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
								copy(b[344:], packed)
								return b
							}
							ref, actual := referencePCMWithChunks(t, fixture(resonance), "1", 180, 256)
							refBase, actualBase := referencePCMWithChunks(t, fixture(0x80), "1", 180, 256)
							assertITResonanceRatios(t, ref, actual, refBase, actualBase)
						})
					}
				}
			}
		}
	}
}

func TestOpenMPTITEmbeddedResonanceMacros(t *testing.T) {
	for _, parametered := range []bool{false, true} {
		for _, envelope := range []bool{false, true} {
			t.Run(fmt.Sprintf("parametered=%t/envelope=%t", parametered, envelope), func(t *testing.T) {
				fixture := func(value byte) []byte {
					b := testITInstrument()
					b[50] = 2
					h := b[len(b)-554:]
					h[304], h[58] = 0, 0x80|64
					if envelope {
						setTestEnvelope(h, 468, 0x81, []int8{-8, 8}, []uint16{0, 4})
					}
					b[256+18] |= 0x10
					binary.LittleEndian.PutUint32(b[256+56:], 3840)
					for i := range 4096 {
						v := int16(math.Sin(float64(i)*2*math.Pi*2000/48000) * 16000)
						binary.LittleEndian.PutUint16(b[512+i*2:], uint16(v))
					}
					rows := []byte{0x81, 11, 60, 1, 26, 0x84, 0}
					fixed := map[int]string{4: fmt.Sprintf("F0F001%02X", value)}
					params := map[int]string{}
					if parametered {
						rows = []byte{0x81, 8, 19, 0xf2, 0, 0x81, 11, 60, 1, 26, value, 0}
						fixed = nil
						params[2] = "F0F001z"
					}
					packed := append(rows, make([]byte, 64)...)
					binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
					copy(b[344:], packed)
					return embeddedMacroFixture(t, b, -1, params, fixed)
				}
				ref, actual := referencePCMWithChunks(t, fixture(96), "1", 180, 256)
				refBase, actualBase := referencePCMWithChunks(t, fixture(0), "1", 180, 256)
				if ratio := rmsWindow(ref, 140, 160) / rmsWindow(refBase, 140, 160); math.Abs(ratio-1) < .05 {
					t.Fatalf("reference resonance had no measurable effect: ratio %g", ratio)
				}
				assertITResonanceRatios(t, ref, actual, refBase, actualBase)
			})
		}
	}
}

func assertITResonanceRatios(t *testing.T, ref, actual, refBase, actualBase []float32) {
	t.Helper()
	for _, win := range [][2]int{{100, 120}, {140, 160}} {
		rb, ab := rmsWindow(refBase, win[0], win[1]), rmsWindow(actualBase, win[0], win[1])
		if rb < 1e-5 || ab < 1e-5 {
			t.Fatalf("silent baseline %g %g", rb, ab)
		}
		want, got := rmsWindow(ref, win[0], win[1])/rb, rmsWindow(actual, win[0], win[1])/ab
		if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .05 {
			t.Errorf("%dms: OpenMPT %g tracker %g", win[0], want, got)
		}
	}
}
