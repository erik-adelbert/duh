package mod

import (
	"encoding/binary"
	"math"
	"reflect"
	"strings"
	"testing"
)

func mptInstrumentPanFixture(t *testing.T, pan uint32, enabled bool) []byte {
	t.Helper()
	b := mptResamplingFixture(t, 1, 60, false)
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	b[instrument+25] = 16
	if !enabled {
		b[instrument+25] |= 0x80
	}
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	values := make([]byte, 8)
	binary.LittleEndian.PutUint32(values, 128)
	binary.LittleEndian.PutUint32(values[4:], pan)
	data := append([]byte(nil), b[:bounds.propertiesStart+4]...)
	data = append(data, mptExtensionField("...P", 4, values)...)
	data = append(data, b[bounds.propertiesStart+4:bounds.serializedStart]...)
	tail := len(data)
	data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(data, uint32(tail))
}

func TestMPTInstrumentPanning(t *testing.T) {
	data := append([]byte("XTPM"), mptExtensionField("...P", 4, []byte{128, 0, 0, 0, 0, 1, 0, 0})...)
	ext, err := parseMPTExtensions(data, 2)
	if err != nil {
		t.Fatal(err)
	}
	clear(data)
	if !reflect.DeepEqual(ext.instrumentPanning, []uint32{128, 256}) {
		t.Fatalf("panning values: %v", ext.instrumentPanning)
	}
	for _, tt := range []struct {
		name string
		data []byte
		want string
	}{
		{"width", append([]byte("XTPM"), mptExtensionField("...P", 2, make([]byte, 4))...), "invalid size"},
		{"truncated", append([]byte("XTPM"), mptExtensionField("...P", 4, make([]byte, 4))...), "truncated"},
		{"duplicate", append(append([]byte("XTPM"), mptExtensionField("...P", 4, make([]byte, 8))...), mptExtensionField("...P", 4, make([]byte, 8))...), "duplicate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseMPTExtensions(tt.data, 2); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
	instruments := []ITInstrument{{Panning: 1}, {Panning: 2}}
	original := append([]ITInstrument(nil), instruments...)
	for _, values := range [][]uint32{{0}, {0, 257}} {
		if err := applyMPTInstrumentPanning(instruments, values); err == nil || !reflect.DeepEqual(instruments, original) {
			t.Fatalf("invalid values %v: error %v, instruments %+v", values, err, instruments)
		}
	}
	for _, enabled := range []bool{false, true} {
		loaded, err := loadMPTM(mptInstrumentPanFixture(t, 192, enabled))
		if err != nil {
			t.Fatal(err)
		}
		got := loaded.Assets.ITInstruments[1]
		if got.Panning != 48 || got.HasPanning != enabled {
			t.Fatalf("enabled %t: %+v", enabled, got)
		}
	}
	for _, pan := range []uint32{0, 1, 103, 127, 128, 129, 193, 255, 256} {
		loaded, err := loadMPTM(mptInstrumentPanFixture(t, pan, true))
		if err != nil {
			t.Fatalf("pan %d: %v", pan, err)
		}
		instrument := &loaded.Assets.ITInstruments[1]
		if !instrument.hasMPTPanning || instrument.normalizedPanning() != float32(pan)/128-1 {
			t.Fatalf("pan %d: %+v", pan, instrument)
		}
	}
}

func TestOpenMPTMInstrumentPanning(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		ref, actual := mptSharedEffectRenderForDuration(t, mptInstrumentPanFixture(t, 192, enabled), 100)
		for channel := range 2 {
			want := channelRMS(ref, 30, 70, channel)
			got := channelRMS(actual, 30, 70, channel)
			if want < .001 || got < .001 || math.Abs(got/want-1) > .08 {
				t.Fatalf("enabled %t channel %d: OpenMPT %g tracker %g", enabled, channel, want, got)
			}

		}
		if enabled && channelRMS(ref, 30, 70, 1) < 1.5*channelRMS(ref, 30, 70, 0) {
			t.Fatal("reference did not apply extended panning")
		}
	}
}
