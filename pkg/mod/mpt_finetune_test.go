package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTFineTune(t *testing.T) {
	tests := []struct {
		name    string
		rows    []byte
		c5Speed uint32
	}{
		{"note and finetune", []byte{0x81, 11, 60, 1, 19, 0x25, 0}, 8363},
		{"lowest finetune", []byte{0x81, 11, 60, 1, 19, 0x20, 0}, 8363},
		{"highest finetune", []byte{0x81, 11, 60, 1, 19, 0x2f, 0}, 8363},
		{"different sample speed", []byte{0x81, 11, 60, 1, 19, 0x25, 0}, 11025},
		{"finetune after note", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 19, 0x25, 0}, 8363},
		{"finetune after slide", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 6, 8, 0, 0x81, 8, 19, 0x25, 0}, 8363},
		{"special memory", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 19, 0x25, 0, 0x81, 8, 19, 0, 0}, 8363},
		{"next note", []byte{0x81, 11, 60, 1, 19, 0x25, 0, 0x81, 1, 72, 0}, 8363},
		{"next note and instrument", []byte{0x81, 11, 60, 1, 19, 0x25, 0, 0x81, 3, 72, 1, 0}, 8363},
		{"volume portamento", []byte{0x81, 3, 60, 1, 0, 0x81, 13, 72, 193, 19, 0x25, 0}, 8363},
		{"moving volume portamento", []byte{0x81, 3, 60, 1, 0, 0x81, 13, 72, 195, 19, 0x25, 0}, 8363},
		{"volume pitch slide", []byte{0x81, 3, 60, 1, 0, 0x81, 12, 116, 19, 0x25, 0}, 8363},
	}
	for _, tt := range tests {
		for _, older := range []bool{false, true} {
			for _, opl := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/older=%t/opl=%t", tt.name, older, opl), func(t *testing.T) {
					fixture := func(rows []byte) ([]byte, []float32) {
						b := mptmPlaybackFixture()
						b[50] = 2
						b[bytesIndex(t, b, "..MT")+6] = 0
						binary.LittleEndian.PutUint32(b[256+60:], tt.c5Speed)
						oplHeader := int(binary.LittleEndian.Uint32(b[198:]))
						binary.LittleEndian.PutUint32(b[oplHeader+60:], tt.c5Speed)
						pattern := int(binary.LittleEndian.Uint32(b[202:]))
						packed := append(append([]byte(nil), rows...), make([]byte, 64)...)
						if opl {
							packed[3] = 2
						}
						binary.LittleEndian.PutUint16(b[pattern:], uint16(len(packed)))
						copy(b[pattern+8:], packed)
						loaded, err := loadMPTM(b)
						if err != nil {
							t.Fatal(err)
						}
						if !older {
							binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01300600)
							copy(b[bytesIndex(t, b, ".FSM")+6:], []byte{0x80, 0xa6, 0x37, 0xd7, 0x86, 0xf2, 0x05})
						}
						w := NewTracker(loaded.Assets, referenceRate, 5760)
						w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 2, 125, -1
						actual := make([]float32, 5760*2)
						if w.Read(actual, 5760) != 5760 {
							t.Fatal("short render")
						}
						return b, actual
					}
					b, actual := fixture(tt.rows)
					base, actualBase := fixture([]byte{0x81, 3, 60, 1, 0})
					ref, refBase := referenceTransition(t, b), referenceTransition(t, base)
					wantBase, gotBase := referenceTickFrequency(t, refBase, 0), referenceTickFrequency(t, actualBase, 0)
					for tick := range 6 {
						want := referenceTickFrequency(t, ref, tick) / wantBase
						got := referenceTickFrequency(t, actual, tick) / gotBase
						if math.Abs(got-want) > .01 {
							t.Errorf("tick %d pitch ratio: OpenMPT %.5f tracker %.5f", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestMPTFineTuneValidation(t *testing.T) {
	for arg := byte(0x20); arg <= 0x2f; arg++ {
		row := []byte{0x81, 8, 19, arg, 0}
		if _, err := unpackITPatternMode(row, 1, true); err != nil {
			t.Fatalf("MPTM S%02X: %v", arg, err)
		}
		if _, err := unpackITPattern(row, 1); err != nil {
			t.Fatalf("ordinary IT S%02X: %v", arg, err)
		}
	}
}

func TestOpenMPTFineTuneInstrument(t *testing.T) {
	for _, opl := range []bool{false, true} {
		t.Run(fmt.Sprintf("opl=%t", opl), func(t *testing.T) {
			fixture := func(finetune bool) ([]byte, []float32) {
				b := oplEnvelopeEndFixture(t, 0x01280100, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}, []byte{0})
				b[30], b[44], b[50] = 8, b[44]|8, 2
				instrument := byte(2)
				if opl {
					instrument = 1
				}
				ptr := int(binary.LittleEndian.Uint32(b[194+4*(instrument-1):]))
				b[ptr+304] = 0
				b[ptr+64+72*2] = 67
				pattern := int(binary.LittleEndian.Uint32(b[210:]))
				rows := []byte{0x81, 3, 60, instrument, 0, 0x81, 7, 72, instrument, 0, 0, 0x81, 4, 64, 0}
				if finetune {
					rows = []byte{0x81, 3, 60, instrument, 0, 0x81, 15, 72, instrument, 0, 19, 0x25, 0, 0x81, 4, 64, 0}
				}
				length := int(binary.LittleEndian.Uint16(b[pattern:]))
				binary.LittleEndian.PutUint16(b[pattern+2:], 32)
				clear(b[pattern+8 : pattern+8+length])
				copy(b[pattern+8:pattern+8+length], rows)
				bounds, err := locateMPTExtensions(b)
				if err != nil {
					t.Fatal(err)
				}
				properties := append([]byte("XTPM"), b[bounds.propertiesStart:bounds.serializedStart]...)
				for _, field := range []struct {
					code string
					data []byte
				}{
					{"...C", []byte{1, 0}}, {"..MT", []byte{0}}, {".MMP", []byte{4, 0, 0, 0}},
					{".APS", []byte{64, 0, 0, 0}}, {"VTSV", []byte{64, 0, 0, 0}},
				} {
					properties = append(properties, mptExtensionField(field.code, uint16(len(field.data)), field.data)...)
				}
				data := append([]byte(nil), b[:bounds.propertiesStart]...)
				data = append(data, properties...)
				tail := len(data)
				data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
				data = binary.LittleEndian.AppendUint32(data, uint32(tail))
				loaded, err := loadMPTM(data)
				if err != nil {
					t.Fatal(err)
				}
				w := NewTracker(loaded.Assets, referenceRate, 5760)
				w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 2, 125, -1
				actual := make([]float32, 5760*2)
				if w.Read(actual, 5760) != 5760 {
					t.Fatal("short render")
				}
				return data, actual
			}
			data, actual := fixture(true)
			base, actualBase := fixture(false)
			ref, refBase := referenceTransition(t, data), referenceTransition(t, base)
			for tick := 4; tick < 6; tick++ {
				want := referenceTickFrequency(t, ref, tick) / referenceTickFrequency(t, refBase, tick)
				got := referenceTickFrequency(t, actual, tick) / referenceTickFrequency(t, actualBase, tick)
				if math.Abs(got-want) > .01 {
					t.Errorf("tick %d pitch ratio: OpenMPT %.5f tracker %.5f", tick, want, got)
				}
			}
		})
	}
}
