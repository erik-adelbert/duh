package mod

import (
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
)

func TestMPTVolumeRampExtension(t *testing.T) {
	data := append([]byte("XTPM"), mptExtensionField("..RV", 2, []byte{0, 0, 0x34, 0x12})...)
	ext, err := parseMPTExtensions(data, 2)
	if err != nil {
		t.Fatal(err)
	}
	clear(data)
	if !reflect.DeepEqual(ext.instrumentVolumeRamp, []uint16{0, 0x1234}) {
		t.Fatalf("ramp values: %v", ext.instrumentVolumeRamp)
	}
	if err := validateMPTVolumeRamp(ext.instrumentVolumeRamp); err == nil ||
		!strings.Contains(err.Error(), "instrument 2 uses unsupported volume ramp-up 4660") {
		t.Fatalf("nonzero ramp: %v", err)
	}
	for _, width := range []uint16{1, 4} {
		data := append([]byte("XTPM"), mptExtensionField("..RV", width, make([]byte, int(width)*2))...)
		if _, err := parseMPTExtensions(data, 2); err == nil || !strings.Contains(err.Error(), "invalid size") {
			t.Fatalf("width %d: %v", width, err)
		}
	}
}

func TestMPTVolumeRampLoader(t *testing.T) {
	b := mptResamplingFixture(t, 1, 60, false)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, ramp := range []uint16{0, 256} {
		values := make([]byte, 4)
		binary.LittleEndian.PutUint16(values[2:], ramp)
		data := append([]byte(nil), b[:bounds.propertiesStart+4]...)
		data = append(data, mptExtensionField("..RV", 2, values)...)
		data = append(data, b[bounds.propertiesStart+4:bounds.serializedStart]...)
		tail := len(data)
		data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
		data = binary.LittleEndian.AppendUint32(data, uint32(tail))
		_, err := loadMPTM(data)
		if ramp == 0 && err != nil || ramp != 0 && (err == nil || !strings.Contains(err.Error(), "instrument 2 uses unsupported volume ramp-up 256")) {
			t.Fatalf("ramp %d: %v", ramp, err)
		}
	}
}
