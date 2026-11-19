package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTMQxxStandalone(t *testing.T) {
	for mode := range byte(16) {
		for interval := range byte(16) {
			arg := mode<<4 | interval
			if arg == 0 {
				continue
			}
			for _, compatible := range []bool{false, true} {
				t.Run(fmt.Sprintf("Q%02X/compatible=%t", arg, compatible), func(t *testing.T) {
					rows := []byte{0x81, 7, 60, 2, 32, 0,
						0x81, 8, 17, arg, 0, 0x81, 8, 17, 0, 0, 0x81, 8, 17, arg, 0}
					b := mptS70Fixture(t, rows)
					b[50] = 4
					flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITRetrigger)/8
					mask := byte(1 << (mptITRetrigger % 8))
					b[flag] &^= mask
					if compatible {
						b[flag] |= mask
					}
					sample := binary.LittleEndian.Uint32(b[202:])
					binary.LittleEndian.PutUint32(b[sample+60:], 48000)
					data := binary.LittleEndian.Uint32(b[sample+72:])
					for i := range 4096 {
						var value uint16
						if i < 700 {
							value = 12000
						}
						binary.LittleEndian.PutUint16(b[data+uint32(i)*2:], value)
					}
					ref, actual := mptSharedEffectRenderForDuration(t, b, 320)
					baseRef, baseActual := rmsWindow(ref, 8, 10), rmsWindow(actual, 8, 10)
					if baseRef < .001 || baseActual < .001 {
						t.Fatal("silent comparison baseline")
					}
					peakRef, peakActual := 0.0, 0.0
					for tick := 4; tick < 16; tick++ {
						want := rmsWindow(ref, tick*20+8, tick*20+10) / baseRef
						got := rmsWindow(actual, tick*20+8, tick*20+10) / baseActual
						peakRef, peakActual = math.Max(peakRef, want), math.Max(peakActual, got)
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .10 {
							t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
						}
						if interval == 2 && mode < 6 && (peakRef < .1 || peakActual < .1) {
							t.Fatal("retrigger marker is unexpectedly silent")
						}
					}
				})
			}
		}
	}
}

func TestOpenMPTMQxxCellInteractions(t *testing.T) {
	tests := []struct {
		name string
		cell []byte
	}{
		{"note", []byte{0x81, 9, 60, 17, 0x92, 0}},
		{"instrument-same", []byte{0x81, 10, 2, 17, 0x92, 0}},
		{"instrument-change", []byte{0x81, 10, 1, 17, 0x92, 0}},
		{"volume", []byte{0x81, 12, 24, 17, 0x92, 0}},
		{"note-instrument", []byte{0x81, 11, 60, 2, 17, 0x92, 0}},
		{"note-volume", []byte{0x81, 13, 60, 24, 17, 0x92, 0}},
	}
	for _, tt := range tests {
		for _, compatible := range []bool{false, true} {
			for _, instrFlag := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/compatible=%t/instrFlag=%t", tt.name, compatible, instrFlag), func(t *testing.T) {
					rows := append([]byte{0x81, 7, 60, 2, 32, 0}, tt.cell...)
					rows = append(rows, 0x81, 8, 17, 0, 0)
					b := mptInstrumentWithoutNoteFixture(t, rows, instrFlag, 0)
					b[50] = 4
					flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITRetrigger)/8
					mask := byte(1 << (mptITRetrigger % 8))
					b[flag] &^= mask
					if compatible {
						b[flag] |= mask
					}
					sample := binary.LittleEndian.Uint32(b[202:])
					b[sample+18] |= 0x10
					binary.LittleEndian.PutUint32(b[sample+52:], 128)
					binary.LittleEndian.PutUint32(b[sample+56:], 2048)
					binary.LittleEndian.PutUint32(b[sample+60:], 48000)
					data := binary.LittleEndian.Uint32(b[sample+72:])
					for i := range 4096 {
						var value uint16
						if i < 700 {
							value = 12000
						}
						binary.LittleEndian.PutUint16(b[data+uint32(i)*2:], value)
					}
					ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
					baseRef, baseActual := rmsWindow(ref, 8, 10), rmsWindow(actual, 8, 10)
					if baseRef < .001 || baseActual < .001 {
						t.Fatal("silent comparison baseline")
					}
					for tick := 4; tick < 12; tick++ {
						want := rmsWindow(ref, tick*20+8, tick*20+10) / baseRef
						got := rmsWindow(actual, tick*20+8, tick*20+10) / baseActual
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .10 {
							t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestOpenMPTMQxxNoteVolume(t *testing.T) {
	for mode := range byte(16) {
		for _, interval := range []byte{0, 1, 2, 3, 5} {
			arg := mode<<4 | interval
			if arg == 0 {
				continue
			}
			for _, compatible := range []bool{false, true} {
				for _, withNote := range []bool{false, true} {
					t.Run(fmt.Sprintf("Q%02X/compatible=%t/note=%t", arg, compatible, withNote), func(t *testing.T) {
						rows := []byte{0x81, 7, 60, 2, 32, 0}
						if withNote {
							rows = append(rows, 0x81, 15, 60, 2, 32, 17, arg, 0)
						} else {
							rows = append(rows, 0x81, 12, 32, 17, arg, 0)
						}
						rows = append(rows, 0x81, 8, 17, 0, 0)
						b := mptS70Fixture(t, rows)
						b[50] = 4
						flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITRetrigger)/8
						mask := byte(1 << (mptITRetrigger % 8))
						b[flag] &^= mask
						if compatible {
							b[flag] |= mask
						}
						sample := binary.LittleEndian.Uint32(b[202:])
						binary.LittleEndian.PutUint32(b[sample+60:], 48000)
						data := binary.LittleEndian.Uint32(b[sample+72:])
						for i := range 4096 {
							var value uint16
							if i < 700 {
								value = 12000
							}
							binary.LittleEndian.PutUint16(b[data+uint32(i)*2:], value)
						}
						ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
						baseRef, baseActual := rmsWindow(ref, 8, 10), rmsWindow(actual, 8, 10)
						if baseRef < .001 || baseActual < .001 {
							t.Fatal("silent comparison baseline")
						}
						for tick := 4; tick < 12; tick++ {
							want := rmsWindow(ref, tick*20+8, tick*20+10) / baseRef
							got := rmsWindow(actual, tick*20+8, tick*20+10) / baseActual
							if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .10 {
								t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
							}
						}
					})
				}
			}
		}
	}
}

func TestMPTQxxShortSampleWarning(t *testing.T) {
	for _, arg := range []byte{0, 0x02, 0xc2} {
		for _, shortSampleRetrig := range []bool{false, true} {
			t.Run(fmt.Sprintf("Q%02X/short=%t", arg, shortSampleRetrig), func(t *testing.T) {
				b := mptS70Fixture(t, []byte{0x81, 3, 60, 2, 0, 0x81, 8, 17, arg, 0})
				sample := binary.LittleEndian.Uint32(b[202:])
				b[sample+18] &^= 0x10
				flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITShortSampleRetrig)/8
				mask := byte(1 << (mptITShortSampleRetrig % 8))
				b[flag] &^= mask
				if shortSampleRetrig {
					b[flag] |= mask
				}
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
			})
		}
	}
}
