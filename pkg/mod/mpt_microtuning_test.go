package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTMMicrotuning(t *testing.T) {
	for _, tt := range []struct {
		arg   byte
		depth int8
	}{
		{0, 2}, {64, 2}, {128, 2}, {192, 2}, {255, 2}, {0, 0}, {0, -2}, {192, 4},
	} {
		t.Run(fmt.Sprintf("arg=%02x/depth=%d", tt.arg, tt.depth), func(t *testing.T) {
			b := mptResamplingFixture(t, 1, 60, true)
			b[53] = byte(tt.depth)
			instrument := int(binary.LittleEndian.Uint32(b[198:]))
			b[instrument+304] = 0
			sample := int(binary.LittleEndian.Uint32(b[202:]))
			data := int(binary.LittleEndian.Uint32(b[sample+72:]))
			for i := range 4096 {
				value := int16(18000 * math.Sin(2*math.Pi*float64(i)/32))
				binary.LittleEndian.PutUint16(b[data+i*2:], uint16(value))
			}
			b = mptPatternRowsFixture(t, b, []byte{
				0x81, 3, 60, 2, 0,
				0x81, 8, 30, tt.arg, 0,
				0x81, 3, 60, 2, 0,
			})
			ref, actual := mptSharedEffectRenderForDuration(t, b, 120)
			for tick := range 6 {
				want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
				if math.Abs(got/want-1) > .003 {
					t.Errorf("tick %d: OpenMPT %.2f tracker %.2f", tick, want, got)
				}
			}
		})
	}
}

func TestMPTMMicrotuningExtensionAndUnsupported(t *testing.T) {
	b := mptResamplingFixture(t, 1, 60, true)
	b = mptPatternRowsFixture(t, b, []byte{0x81, 11, 60, 2, 30, 0, 0})
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	field := mptExtensionField("DWPM", 1, []byte{0, 0xfe})
	out := append([]byte(nil), b[:bounds.propertiesStart+4]...)
	out = append(out, field...)
	out = append(out, b[bounds.propertiesStart+4:len(b)-4]...)
	out = binary.LittleEndian.AppendUint32(out, uint32(bounds.serializedStart+len(field)))
	loaded, err := loadMPTM(out)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Assets.ITInstruments[1].mptPitchWheelDepth != -2 {
		t.Fatalf("pitch wheel depth: %d", loaded.Assets.ITInstruments[1].mptPitchWheelDepth)
	}
	if _, err := unpackITPatternMode([]byte{0x81, 8, 27, 0, 0}, 1, true); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("extended parameter unexpectedly accepted: %v", err)
	}
}

func TestOpenMPTMMicrotuningTransitions(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"note only", []byte{0x81, 1, 60, 0}},
		{"same instrument only", []byte{0x81, 2, 2, 0}},
		{"same instrument porta", []byte{0x81, 11, 64, 2, 7, 4, 0}},
		{"same row note", []byte{0x81, 11, 60, 2, 30, 0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := mptResamplingFixture(t, 1, 60, true)
			b[53] = 2
			instrument := int(binary.LittleEndian.Uint32(b[198:]))
			b[instrument+304] = 0
			sample := int(binary.LittleEndian.Uint32(b[202:]))
			data := int(binary.LittleEndian.Uint32(b[sample+72:]))
			for i := range 4096 {
				binary.LittleEndian.PutUint16(b[data+i*2:], uint16(int16(18000*math.Sin(2*math.Pi*float64(i)/32))))
			}
			rows := []byte{0x81, 3, 60, 2, 0, 0x81, 8, 30, 0, 0}
			rows = append(rows, tt.rows...)
			b = mptPatternRowsFixture(t, b, rows)
			ref, actual := mptSharedEffectRenderForDuration(t, b, 120)
			for tick := 2; tick < 6; tick++ {
				want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
				if math.Abs(got/want-1) > .003 {
					t.Errorf("tick %d: OpenMPT %.2f tracker %.2f", tick, want, got)
				}
			}
		})
	}
}

func TestOpenMPTMMicrotuningWithoutActiveVoice(t *testing.T) {
	b := mptResamplingFixture(t, 1, 60, true)
	b[53] = 2
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	b[instrument+304] = 0
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	data := int(binary.LittleEndian.Uint32(b[sample+72:]))
	for i := range 4096 {
		binary.LittleEndian.PutUint16(b[data+i*2:], uint16(int16(18000*math.Sin(2*math.Pi*float64(i)/32))))
	}
	b = mptPatternRowsFixture(t, b, []byte{
		0x81, 3, 60, 2, 0,
		0x81, 1, 254, 0,
		0x81, 8, 30, 0, 0,
		0x81, 1, 60, 0,
	})
	ref, actual := mptSharedEffectRenderForDuration(t, b, 160)
	for tick := 6; tick < 8; tick++ {
		want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
		if math.Abs(got/want-1) > .003 {
			t.Errorf("tick %d: OpenMPT %.2f tracker %.2f", tick, want, got)
		}
	}
}
