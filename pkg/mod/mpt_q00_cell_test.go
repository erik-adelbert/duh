package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMQ00CellInteractions(t *testing.T) {
	cells := []struct {
		name string
		data []byte
	}{
		{"note", []byte{0x81, 9, 60, 17, 0, 0}},
		{"instrument-same", []byte{0x81, 10, 2, 17, 0, 0}},
		{"instrument-change", []byte{0x81, 10, 1, 17, 0, 0}},
		{"volume", []byte{0x81, 12, 24, 17, 0, 0}},
		{"fine-up", []byte{0x81, 12, 67, 17, 0, 0}},
		{"fine-down", []byte{0x81, 12, 77, 17, 0, 0}},
		{"slide-up", []byte{0x81, 12, 87, 17, 0, 0}},
		{"slide-down", []byte{0x81, 12, 97, 17, 0, 0}},
		{"note-instrument", []byte{0x81, 11, 64, 2, 17, 0, 0}},
		{"note-instrument-volume", []byte{0x81, 15, 60, 2, 24, 17, 0, 0}},
	}
	for _, tt := range cells {
		for _, memory := range []byte{0, 0x02, 0x92, 0xc2} {
			for _, compatible := range []bool{false, true} {
				for _, instrFlag := range []bool{false, true} {
					for _, tremor := range []bool{false, true} {
						t.Run(fmt.Sprintf("%s/memory=%02X/compatible=%t/instrFlag=%t/tremor=%t", tt.name, memory, compatible, instrFlag, tremor), func(t *testing.T) {
							rows := []byte{0x81, 7, 60, 2, 32, 0}
							if memory != 0 {
								rows = append(rows, 0x81, 8, 17, memory, 0)
							} else {
								rows = append(rows, 0)
							}
							rows = append(rows, tt.data...)
							rows = append(rows, 0x81, 8, 17, 0, 0)
							b := mptInstrumentWithoutNoteFixture(t, rows, instrFlag, 0)
							b[50] = 4
							flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITRetrigger)/8
							mask := byte(1 << (mptITRetrigger % 8))
							b[flag] &^= mask
							if compatible {
								b[flag] |= mask
							}
							flag = bytesIndex(t, b, ".FSM") + 6 + int(mptITTremor)/8
							mask = byte(1 << (mptITTremor % 8))
							b[flag] &^= mask
							if tremor {
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
							ref, actual := mptSharedEffectRenderForDuration(t, b, 320)
							baseRef, baseActual := rmsWindow(ref, 8, 10), rmsWindow(actual, 8, 10)
							if baseRef < .001 || baseActual < .001 {
								t.Fatal("silent comparison baseline")
							}
							for tick := 8; tick < 16; tick++ {
								want := rmsWindow(ref, tick*20+8, tick*20+10) / baseRef
								got := rmsWindow(actual, tick*20+8, tick*20+10) / baseActual
								if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .10 {
									t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
								}
							}
							assertMPTPatternDelayChunking(t, b, 14000)
						})
					}
				}
			}
		}
	}
}

func TestOpenMPTMQ00SampleModeAndSpeed(t *testing.T) {
	for _, speed := range []byte{1, 2, 4, 6} {
		for _, memory := range []byte{0, 0x02, 0x92} {
			for _, compatible := range []bool{false, true} {
				for _, withNote := range []bool{false, true} {
					t.Run(fmt.Sprintf("speed=%d/memory=%02X/compatible=%t/note=%t", speed, memory, compatible, withNote), func(t *testing.T) {
						rows := []byte{0x81, 7, 60, 1, 32, 0}
						if memory == 0 {
							rows = append(rows, 0)
						} else {
							rows = append(rows, 0x81, 8, 17, memory, 0)
						}

						if withNote {
							rows = append(rows, 0x81, 15, 60, 1, 24, 17, 0, 0)
						} else {
							rows = append(rows, 0x81, 12, 87, 17, 0, 0)
						}
						rows = append(rows, 0x81, 8, 17, 0, 0)
						b := mptTwoPCMSamplesFixture(t, rows, true, true)
						b[50] = speed
						flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITRetrigger)/8
						mask := byte(1 << (mptITRetrigger % 8))
						b[flag] &^= mask
						if compatible {
							b[flag] |= mask
						}
						sample := binary.LittleEndian.Uint32(b[194:])
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
						duration := int(speed)*80 + 20
						ref, actual := mptSharedEffectRenderForDuration(t, b, duration)
						baseRef, baseActual := rmsWindow(ref, 8, 10), rmsWindow(actual, 8, 10)
						if baseRef < .001 || baseActual < .001 {
							t.Fatal("silent comparison baseline")
						}
						for tick := int(speed) * 2; tick < int(speed)*4; tick++ {
							want := rmsWindow(ref, tick*20+8, tick*20+10) / baseRef
							got := rmsWindow(actual, tick*20+8, tick*20+10) / baseActual
							if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .10 {
								t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
							}
						}
						assertMPTPatternDelayChunking(t, b, int(speed)*4*960)
					})
				}
			}
		}
	}
}
