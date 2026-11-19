package mod

import (
	"encoding/binary"
	"math"
	"reflect"
	"strings"
	"testing"
)

func mptInstrumentGlobalVolumeFixture(t *testing.T, value uint32) []byte {
	t.Helper()
	b := mptResamplingFixture(t, 1, 60, true)
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	b[instrument+24] = 32
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	values := binary.LittleEndian.AppendUint32(nil, 64)
	values = binary.LittleEndian.AppendUint32(values, value)
	data := append([]byte(nil), b[:bounds.propertiesStart+4]...)
	data = append(data, mptExtensionField("..VG", 4, values)...)
	data = append(data, b[bounds.propertiesStart+4:bounds.serializedStart]...)
	tail := len(data)
	data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(data, uint32(tail))
}

func TestMPTInstrumentGlobalVolume(t *testing.T) {
	for _, width := range []int{1, 2, 4} {
		values := make([]byte, width*2)
		values[0], values[width] = 32, 64
		data := append([]byte("XTPM"), mptExtensionField("..VG", uint16(width), values)...)
		ext, err := parseMPTExtensions(data, 2)
		if err != nil || !reflect.DeepEqual(ext.instrumentGlobalVolume, []uint32{32, 64}) {
			t.Fatalf("width %d: %+v, %v", width, ext.instrumentGlobalVolume, err)
		}
	}
	for _, tt := range []struct {
		name string
		data []byte
		want string
	}{
		{"width", append([]byte("XTPM"), mptExtensionField("..VG", 3, make([]byte, 6))...), "invalid size"},
		{"truncated", append([]byte("XTPM"), mptExtensionField("..VG", 4, make([]byte, 4))...), "truncated"},
		{"duplicate", append(append([]byte("XTPM"), mptExtensionField("..VG", 1, []byte{1, 2})...), mptExtensionField("..VG", 1, []byte{1, 2})...), "duplicate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseMPTExtensions(tt.data, 2); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
	instruments := []ITInstrument{{GlobalVolume: 12}, {GlobalVolume: 34}}
	before := append([]ITInstrument(nil), instruments...)
	if err := applyMPTInstrumentGlobalVolume(instruments, []uint32{1}); err == nil || !reflect.DeepEqual(instruments, before) {
		t.Fatalf("invalid count: instruments %+v, error %v", instruments, err)
	}
	if err := applyMPTInstrumentGlobalVolume(instruments, []uint32{32, 65536}); err == nil || !reflect.DeepEqual(instruments, before) {
		t.Fatalf("invalid value: instruments %+v, error %v", instruments, err)
	}
	for _, tt := range []struct {
		value uint32
		want  uint8
	}{{0, 0}, {1, 2}, {32, 64}, {64, 128}, {65, 128}, {65535, 128}} {
		loaded, err := loadMPTM(mptInstrumentGlobalVolumeFixture(t, tt.value))
		if err != nil {
			t.Fatal(err)
		}
		if got := loaded.Assets.ITInstruments[1].GlobalVolume; got != tt.want {
			t.Errorf("value %d: got %d, want %d", tt.value, got, tt.want)
		}
		if _, err := loadMPTM(mptInstrumentGlobalVolumeFixture(t, 65536)); err == nil || !strings.Contains(err.Error(), "unsupported extended global volume") {
			t.Fatalf("oversized volume: %v", err)
		}
	}
}

func TestOpenMPTMInstrumentGlobalVolume(t *testing.T) {
	baseRef, baseActual := mptSharedEffectRenderForDuration(t, mptInstrumentGlobalVolumeFixture(t, 64), 100)
	for _, value := range []uint32{0, 1, 16, 32, 64, 65, 65535} {
		ref, actual := mptSharedEffectRenderForDuration(t, mptInstrumentGlobalVolumeFixture(t, value), 100)
		want := rmsWindow(ref, 30, 70) / rmsWindow(baseRef, 30, 70)
		got := rmsWindow(actual, 30, 70) / rmsWindow(baseActual, 30, 70)
		if math.IsNaN(want) || math.IsNaN(got) || math.Abs(want-got) > .02 {
			t.Errorf("value %d: OpenMPT ratio %g, tracker %g", value, want, got)
		}

	}
}

func TestOpenMPTMInstrumentGlobalVolumeSampleGain(t *testing.T) {
	for _, sampleVolume := range []byte{31, 63} {
		fixture := func(value uint32) []byte {
			b := mptInstrumentGlobalVolumeFixture(t, value)
			sample := int(binary.LittleEndian.Uint32(b[202:]))
			b[sample+17] = sampleVolume
			return b
		}

		baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture(64), 100)
		ref, actual := mptSharedEffectRenderForDuration(t, fixture(17), 100)
		want := rmsWindow(ref, 30, 70) / rmsWindow(baseRef, 30, 70)
		got := rmsWindow(actual, 30, 70) / rmsWindow(baseActual, 30, 70)
		if math.IsNaN(want) || math.IsNaN(got) || math.Abs(want-got) > .01 {
			t.Errorf("sample volume %d: OpenMPT ratio %g, tracker %g", sampleVolume, want, got)
		}

	}
}
