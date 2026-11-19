package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func mptFilterModeFixture(t *testing.T, mode byte) []byte {
	t.Helper()
	b := mptResamplingFixture(t, 1, 60, true)
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	b[instrument+58], b[instrument+59] = 128|70, 128|60
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	data := append([]byte(nil), b[:bounds.propertiesStart+4]...)
	data = append(data, mptExtensionField("..MF", 1, []byte{255, mode})...)
	data = append(data, b[bounds.propertiesStart+4:bounds.serializedStart]...)
	tail := len(data)
	data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(data, uint32(tail))
}

func TestOpenMPTMInstrumentFilterModes(t *testing.T) {
	var outputs [2][]float32
	for _, mode := range []byte{0, 1, 255} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			ref, actual := mptSharedEffectRenderForDuration(t, mptFilterModeFixture(t, mode), 100)
			var cross, r2, a2 float64
			for i := 10 * 96; i < 80*96; i++ {
				r, a := float64(ref[i]), float64(actual[i])
				cross, r2, a2 = cross+r*a, r2+r*r, a2+a*a
			}
			correlation, gain := cross/math.Sqrt(r2*a2), math.Sqrt(a2/r2)
			if r2 < .001 || a2 < .001 || math.IsNaN(correlation) || correlation < .98 || math.Abs(gain-1) > .05 {
				t.Fatalf("correlation %g gain %g energy %g/%g", correlation, gain, r2, a2)
			}
			if mode < 2 {
				outputs[mode] = actual
			}
		})
	}
	var diff float64
	for i := 10 * 96; i < 80*96; i++ {
		d := float64(outputs[0][i] - outputs[1][i])
		diff += d * d
	}
	if diff < .01 {
		t.Fatalf("high-pass and low-pass outputs did not differ: %g", diff)
	}
}

func TestMPTFilterModeMemory(t *testing.T) {
	loaded, err := loadMPTM(mptFilterModeFixture(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	unchanged := w.Assets.ITInstruments[1]
	unchanged.mptHasFilterMode = false
	low := unchanged
	low.mptHasFilterMode = true
	low.mptFilterMode = 0
	w.Assets.ITInstruments = append(w.Assets.ITInstruments, unchanged, low)
	for _, tt := range []struct {
		instrument uint8
		high       bool
	}{{2, true}, {3, true}, {4, false}, {3, false}, {2, true}} {
		w.PendingCmds[0] = ModCommand{Note: 61, Instrument: tt.instrument}
		VoiceAllocationSystem(w)
		v := w.TrackCursors[0].ActiveVoice
		if v < 0 || !w.VoiceFilters[v].Enabled || w.VoiceFilters[v].HighPass != tt.high || w.VoiceLives[v].mptHighPass != tt.high {
			t.Fatalf("instrument %d: voice %d cursor %+v", tt.instrument, v, w.TrackCursors[0])
		}
	}
}

func TestMPTFilterModeUpdates(t *testing.T) {
	loaded, err := loadMPTM(mptFilterModeFixture(t, 1))
	if err != nil {
		t.Fatal(err)
	}

	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 2}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	w.VoiceFilters[v].Y1, w.VoiceRightFilters[v].Y1 = .1, -.2
	updateITFilterEnvelope(w, int(v), -10)
	if !w.VoiceFilters[v].HighPass || !w.VoiceRightFilters[v].HighPass ||
		w.VoiceFilters[v].Y1 != .1 || w.VoiceRightFilters[v].Y1 != -.2 {
		t.Fatal("filter envelope lost mode or stereo history")
	}
	w.Clock.Tick = 0
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x19, EffectArg: 40}
	MPTCutoffMacroSystem(w)
	if !w.VoiceFilters[v].HighPass || !w.VoiceRightFilters[v].HighPass ||
		w.VoiceFilters[v].Y1 != .1 || w.VoiceRightFilters[v].Y1 != -.2 {
		t.Fatal("filter macro lost mode or stereo history")
	}
}

func TestMPTFilterModeExtension(t *testing.T) {
	data := append([]byte("XTPM"), mptExtensionField("..MF", 1, []byte{255, 1})...)
	ext, err := parseMPTExtensions(data, 2)
	if err != nil {
		t.Fatal(err)
	}
	clear(data)
	if !reflect.DeepEqual(ext.instrumentFilterMode, []uint8{255, 1}) {
		t.Fatalf("filter modes: %v", ext.instrumentFilterMode)
	}
	for _, tt := range []struct {
		name string
		data []byte
		want string
	}{
		{"width", append([]byte("XTPM"), mptExtensionField("..MF", 2, make([]byte, 4))...), "invalid size"},
		{"truncated", append([]byte("XTPM"), mptExtensionField("..MF", 1, []byte{255})...), "truncated"},
		{"duplicate", append(append([]byte("XTPM"), mptExtensionField("..MF", 1, []byte{255, 255})...), mptExtensionField("..MF", 1, []byte{255, 255})...), "duplicate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseMPTExtensions(tt.data, 2); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
}

func TestMPTFilterModeLoader(t *testing.T) {
	b := mptResamplingFixture(t, 1, 60, false)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []byte{255, 0, 1, 2} {
		data := append([]byte(nil), b[:bounds.propertiesStart+4]...)
		data = append(data, mptExtensionField("..MF", 1, []byte{255, mode})...)
		data = append(data, b[bounds.propertiesStart+4:bounds.serializedStart]...)
		tail := len(data)
		data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
		data = binary.LittleEndian.AppendUint32(data, uint32(tail))
		_, err := loadMPTM(data)
		if mode != 2 && err != nil || mode == 2 && (err == nil || !strings.Contains(err.Error(), "instrument 2 uses unsupported filter mode")) {
			t.Fatalf("mode %d: %v", mode, err)
		}
	}
}

func TestOpenMPTFilterModeInheritance(t *testing.T) {
	b := mptFilterModeFixture(t, 255)
	first := int(binary.LittleEndian.Uint32(b[194:]))
	second := int(binary.LittleEndian.Uint32(b[198:]))
	copy(b[first:first+554], b[second:second+554])
	mode := bytesIndex(t, b, "..MF")
	b[mode+6] = 1
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	rows := []byte{0x81, 3, 60, 1, 0, 0x81, 3, 60, 2, 0}
	rows = append(rows, make([]byte, 62)...)
	oldLength := int(binary.LittleEndian.Uint16(b[pattern:]))
	oldTail := binary.LittleEndian.Uint32(b[len(b)-4:])
	data := append([]byte(nil), b[:pattern+8]...)
	binary.LittleEndian.PutUint16(data[pattern:], uint16(len(rows)))
	data = append(data, rows...)
	data = append(data, b[pattern+8+oldLength:len(b)-4]...)
	b = binary.LittleEndian.AppendUint32(data, oldTail+uint32(len(rows)-oldLength))
	ref, actual := mptSharedEffectRenderForDuration(t, b, 120)
	for _, window := range [][2]int{{10, 30}, {60, 100}} {
		var cross, r2, a2 float64
		for i := window[0] * 96; i < window[1]*96; i++ {
			r, a := float64(ref[i]), float64(actual[i])
			cross, r2, a2 = cross+r*a, r2+r*r, a2+a*a
		}
		correlation, gain := cross/math.Sqrt(r2*a2), math.Sqrt(a2/r2)
		if r2 < .001 || a2 < .001 || math.IsNaN(correlation) || correlation < .98 || math.Abs(gain-1) > .05 {
			t.Fatalf("%v ms: correlation %g gain %g energy %g/%g", window, correlation, gain, r2, a2)
		}
	}
}
