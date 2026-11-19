package mod

import (
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
)

func TestMPTPanReleaseNodeExtension(t *testing.T) {
	data := append([]byte("XTPM"), mptExtensionField("NREA", 1, []byte{255, 3})...)
	ext, err := parseMPTExtensions(data, 2)
	if err != nil {
		t.Fatal(err)
	}
	clear(data)
	if !reflect.DeepEqual(ext.instrumentPanReleaseNode, []uint8{255, 3}) {
		t.Fatalf("pan release nodes: %v", ext.instrumentPanReleaseNode)
	}
	for _, tt := range []struct {
		name string
		data []byte
		want string
	}{
		{"width", append([]byte("XTPM"), mptExtensionField("NREA", 2, make([]byte, 4))...), "invalid size"},
		{"truncated", append([]byte("XTPM"), mptExtensionField("NREA", 1, []byte{255})...), "truncated"},
		{"duplicate", append(append([]byte("XTPM"), mptExtensionField("NREA", 1, []byte{255, 255})...), mptExtensionField("NREA", 1, []byte{255, 255})...), "duplicate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseMPTExtensions(tt.data, 2); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
}

func TestMPTPanReleaseNodeLoader(t *testing.T) {
	b := mptResamplingFixture(t, 1, 60, false)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range []byte{255, 0, 3, 254} {
		data := append([]byte(nil), b[:bounds.propertiesStart+4]...)
		data = append(data, mptExtensionField("NREA", 1, []byte{255, node})...)
		data = append(data, b[bounds.propertiesStart+4:bounds.serializedStart]...)
		tail := len(data)
		data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
		data = binary.LittleEndian.AppendUint32(data, uint32(tail))
		_, err := loadMPTM(data)
		if node == 255 && err != nil || node != 255 && (err == nil || !strings.Contains(err.Error(), "instrument 2 uses unsupported pan-envelope release node")) {
			t.Fatalf("node %d: %v", node, err)
		}
	}
}

func TestMPTPitchReleaseNodeExtension(t *testing.T) {
	data := append([]byte("XTPM"), mptExtensionField("NREP", 1, []byte{255, 3})...)
	ext, err := parseMPTExtensions(data, 2)
	if err != nil {
		t.Fatal(err)
	}
	clear(data)
	if !reflect.DeepEqual(ext.instrumentPitchReleaseNode, []uint8{255, 3}) {
		t.Fatalf("pitch release nodes: %v", ext.instrumentPitchReleaseNode)
	}
	for _, tt := range []struct {
		name string
		data []byte
		want string
	}{
		{"width", append([]byte("XTPM"), mptExtensionField("NREP", 2, make([]byte, 4))...), "invalid size"},
		{"truncated", append([]byte("XTPM"), mptExtensionField("NREP", 1, []byte{255})...), "truncated"},
		{"duplicate", append(append([]byte("XTPM"), mptExtensionField("NREP", 1, []byte{255, 255})...), mptExtensionField("NREP", 1, []byte{255, 255})...), "duplicate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseMPTExtensions(tt.data, 2); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
}

func TestMPTPitchReleaseNodeLoader(t *testing.T) {
	b := mptResamplingFixture(t, 1, 60, false)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range []byte{255, 0, 3, 254} {
		data := append([]byte(nil), b[:bounds.propertiesStart+4]...)
		data = append(data, mptExtensionField("NREP", 1, []byte{255, node})...)
		data = append(data, b[bounds.propertiesStart+4:bounds.serializedStart]...)
		tail := len(data)
		data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
		data = binary.LittleEndian.AppendUint32(data, uint32(tail))
		_, err := loadMPTM(data)
		if node == 255 && err != nil || node != 255 && (err == nil || !strings.Contains(err.Error(), "instrument 2 uses unsupported pitch-envelope release node")) {
			t.Fatalf("node %d: %v", node, err)
		}
	}
}

func TestMPTVolumeReleaseNodeExtension(t *testing.T) {
	data := append([]byte("XTPM"), mptExtensionField("NREV", 1, []byte{255, 3})...)
	ext, err := parseMPTExtensions(data, 2)
	if err != nil {
		t.Fatal(err)
	}
	clear(data)
	if !reflect.DeepEqual(ext.instrumentVolumeReleaseNode, []uint8{255, 3}) {
		t.Fatalf("volume release nodes: %v", ext.instrumentVolumeReleaseNode)
	}
	for _, tt := range []struct {
		name string
		data []byte
		want string
	}{
		{"width", append([]byte("XTPM"), mptExtensionField("NREV", 2, make([]byte, 4))...), "invalid size"},
		{"truncated", append([]byte("XTPM"), mptExtensionField("NREV", 1, []byte{255})...), "truncated"},
		{"duplicate", append(append([]byte("XTPM"), mptExtensionField("NREV", 1, []byte{255, 255})...), mptExtensionField("NREV", 1, []byte{255, 255})...), "duplicate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseMPTExtensions(tt.data, 2); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
}

func TestMPTVolumeReleaseNodeLoader(t *testing.T) {
	b := mptResamplingFixture(t, 1, 60, false)
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	setTestEnvelope(b[instrument:instrument+554], 304, 1, []int8{64, 32, 0}, []uint16{0, 3, 8})
	binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01290000)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range []byte{255, 0, 1, 2, 3, 254} {
		data := append([]byte(nil), b[:bounds.propertiesStart+4]...)
		data = append(data, mptExtensionField("NREV", 1, []byte{255, node})...)
		data = append(data, b[bounds.propertiesStart+4:bounds.serializedStart]...)
		tail := len(data)
		data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
		data = binary.LittleEndian.AppendUint32(data, uint32(tail))
		_, err := loadMPTM(data)
		if (node == 255 || node < 3) && err != nil || node != 255 && node >= 3 && (err == nil || !strings.Contains(err.Error(), "invalid volume-envelope release node")) {
			t.Fatalf("node %d: %v", node, err)
		}
	}
}
