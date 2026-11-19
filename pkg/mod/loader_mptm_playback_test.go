package mod

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mptmPlaybackFixture() []byte {
	b, _ := transitionFixture(false, false, false)
	b = b[:len(b)-11]
	b[30], b[44], b[46] = 8, b[44]|8, b[46]|4
	b = append(b, []byte("STPM")...)
	b = append(b, mptExtensionField("...C", 2, []byte{1, 0})...)
	b = append(b, mptExtensionField("..MT", 1, []byte{2})...)
	b = append(b, mptExtensionField(".MMP", 4, []byte{4, 0, 0, 0})...)
	b = append(b, mptExtensionField("VWSL", 4, []byte{0, 1, 0x28, 1})...)
	b = append(b, mptExtensionField(".APS", 4, []byte{64, 0, 0, 0})...)
	b = append(b, mptExtensionField("VTSV", 4, []byte{64, 0, 0, 0})...)
	b = append(b, mptExtensionField(".FSM", 7, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7})...)
	tail := len(b)
	b = append(b, mptTestSequences([]mptTestEntry{{"l", []byte{1, 0}}, {"a", []byte{0, 0}}})...)
	return binary.LittleEndian.AppendUint32(b, uint32(tail))
}

func TestLoadMPTMSynthetic(t *testing.T) {
	data := mptmPlaybackFixture()
	loaded, err := loadMPTM(data)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Assets.NumTracks != 1 || loaded.Assets.Samples[1].opl == nil || loaded.Assets.mptRowsPerBeat != 8 {
		t.Fatal("synthetic MPTM configuration was not loaded")
	}
	path := filepath.Join(t.TempDir(), "synthetic.mptm")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMPTM(path); err != nil {
		t.Fatalf("loading synthetic MPTM from file: %v", err)
	}
	w := NewTracker(loaded.Assets, 48000, 4096)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	buf := make([]float32, 2048)
	nonzero := false
	for range 50 {
		n := w.Read(buf, len(buf)/2)
		for _, sample := range buf[:n*2] {
			nonzero = nonzero || sample != 0
		}
	}
	if !nonzero {
		t.Fatal("synthetic MPTM produced silence")
	}
}

func TestLoadMPTMRejectsUnsupported(t *testing.T) {
	for _, tt := range []struct {
		name string
		edit func([]byte)
		want string
	}{
		{"unknown flags", func(b []byte) { b[bytesIndex(t, b, ".FSM")+12] |= 8 }, "playback behaviour bit 51"},
		{"unknown song property", func(b []byte) { copy(b[bytesIndex(t, b, ".APS"):], "????") }, "unsupported MPTM song field"},
		{"alternative tempo", func(b []byte) { b[bytesIndex(t, b, "..MT")+6] = 1 }, "tempo"},
		{"unsupported OPL flags", func(b []byte) { b[int(binary.LittleEndian.Uint32(b[198:]))+18] |= 0x20 }, "invalid MPTM OPL sample"},
		{"invalid OPL panning", func(b []byte) { b[int(binary.LittleEndian.Uint32(b[198:]))+47] = 0xff }, "invalid MPTM OPL sample"},
		{"too few channels", func(b []byte) { b[bytesIndex(t, b, "...C")+6] = 0 }, "missing required"},
		{"command on undeclared channel", func(b []byte) { b[int(binary.LittleEndian.Uint32(b[202:]))+8] = 0x82 }, "beyond the declared channel count"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := mptmPlaybackFixture()
			tt.edit(data)
			if _, err := loadMPTM(data); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want error containing %q", err, tt.want)
			}
		})
	}
	for _, data := range [][]byte{nil, []byte("IMPM"), make([]byte, 192)} {
		if _, err := loadMPTM(data); err == nil {
			t.Fatal("accepted malformed MPTM")
		}
	}
}

func TestLoadMPTMOtherPlaybackProfiles(t *testing.T) {
	for _, tt := range []struct {
		name string
		edit func([]byte)
	}{
		{"changed flags", func(b []byte) { b[bytesIndex(t, b, ".FSM")+6] ^= 2 }},
		{"nonlinear slides", func(b []byte) { b[44] &^= 8 }},
		{"newer version", func(b []byte) { binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01300600) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := mptmPlaybackFixture()
			tt.edit(b)
			if _, err := loadMPTM(b); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLoadMPTMInstrumentPlugin(t *testing.T) {
	for _, tt := range []struct {
		plugin, legacy byte
		warn           bool
	}{{0, 0, false}, {3, 0, true}, {0, 129, true}} {
		b := oplEnvelopeEndFixture(t, 0x01280100, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}, []byte{0})
		b[30], b[44] = 8, b[44]|8
		instrument := int(binary.LittleEndian.Uint32(b[194:]))
		b[instrument+60] = tt.legacy
		bounds, err := locateMPTExtensions(b)
		if err != nil {
			t.Fatal(err)
		}
		field := append([]byte("XTPM"), mptExtensionField(".PiM", 1, []byte{tt.plugin, 0})...)
		properties := append(field, b[bounds.propertiesStart:bounds.serializedStart]...)
		for _, extra := range []struct {
			code string
			data []byte
		}{
			{"...C", []byte{1, 0}}, {"..MT", []byte{2}}, {".MMP", []byte{4, 0, 0, 0}},
			{".APS", []byte{64, 0, 0, 0}}, {"VTSV", []byte{64, 0, 0, 0}},
		} {
			properties = append(properties, mptExtensionField(extra.code, uint16(len(extra.data)), extra.data)...)
		}
		data := append([]byte(nil), b[:bounds.propertiesStart]...)
		data = append(data, properties...)
		tail := len(data)
		data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
		data = binary.LittleEndian.AppendUint32(data, uint32(tail))
		loaded, err := loadMPTM(data)
		if err != nil {
			t.Fatalf("plugin %d legacy %d: %v", tt.plugin, tt.legacy, err)
		}
		if (len(loaded.Warnings) != 0) != tt.warn {
			t.Fatalf("plugin %d legacy %d: warnings %v", tt.plugin, tt.legacy, loaded.Warnings)
		}
	}
}

func bytesIndex(t *testing.T, data []byte, code string) int {
	t.Helper()
	bounds, err := locateMPTExtensions(data)
	if err != nil {
		t.Fatal(err)
	}
	i := strings.Index(string(data[bounds.propertiesStart:bounds.serializedStart]), code)
	if i < 0 {
		t.Fatalf("field %s not found", code)
	}
	return bounds.propertiesStart + i
}

func TestLoadMPTMTarget(t *testing.T) {
	path := os.Getenv("MPTECS_MPTM_TARGET")
	if path == "" {
		t.Skip("set MPTECS_MPTM_TARGET to the audited Triangel Island file")
	}
	if _, err := LoadIT(path); err == nil || !strings.Contains(err.Error(), "MPTM modules are not yet supported") {
		t.Fatalf("ordinary IT loader must reject MPTM: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[4] ^= 1 // Song title changes do not change the supported playback profile.
	if _, err := loadMPTM(data); err != nil {
		t.Fatalf("target with changed title: %v", err)
	}
	instrumentTable := 192 + int(binary.LittleEndian.Uint16(data[32:]))
	instrumentOffset := int(binary.LittleEndian.Uint32(data[instrumentTable:]))
	data[instrumentOffset+60] = 1
	if _, err := loadMPTM(data); err == nil || !strings.Contains(err.Error(), "unsupported MIDI output") {
		t.Fatalf("target with MIDI output: %v", err)
	}
	loaded, err := LoadMPTM(path)
	if err != nil {
		t.Fatal(err)
	}
	a := loaded.Assets
	expectedOrders := []int{4, 5, 6, 0, 1, 2, 3, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 13, 21, 22, 23, 24, 25, 26, 27, 28}
	if !reflect.DeepEqual(a.OrderList, expectedOrders) || a.NumTracks != 16 || len(a.Patterns) != 29 ||
		len(a.Samples) != 18 || len(a.ITInstruments) != 19 || loaded.InitialSpeed != 4 ||
		loaded.InitialTempo != 115 || a.mptRowsPerBeat != 8 {
		t.Fatalf("unexpected target configuration: tracks=%d orders=%v patterns=%d samples=%d instruments=%d speed=%d tempo=%v rows/beat=%d",
			a.NumTracks, a.OrderList, len(a.Patterns), len(a.Samples), len(a.ITInstruments), loaded.InitialSpeed, loaded.InitialTempo, a.mptRowsPerBeat)
	}
	opl, stereo := 0, 0
	for _, sample := range a.Samples {
		if sample.opl != nil {
			opl++
		}
		if len(sample.RightData) > 0 && len(sample.Data) == len(sample.RightData) {
			stereo++
		}
	}
	if opl != 16 || stereo != 2 || !a.ITInstruments[9].hasMPTFadeout || a.ITInstruments[9].mptFadeout != 3333 ||
		a.ITCompatibility.itPatternLoopProfile == nil || a.ITCompatibility.itPanbrelloProfile == nil ||
		a.ITCompatibility.mptMixProfile == nil || !a.ITCompatibility.itDCTSampleAcrossInstruments ||
		!a.ITCompatibility.oplEnvelopeEndCut || !a.ITCompatibility.oplNoResetAtEnvelopeEnd {
		t.Fatalf("unexpected target sample or compatibility profile: opl=%d stereo=%d", opl, stereo)
	}

	const rate = 48000
	w := NewTracker(a, rate, 4096)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	buf := make([]float32, 2048)
	var frames int
	var windowEnergy [5]float64
	starts := [...]int{0, 60, 120, 180, 250}
	for {
		n := w.Read(buf, len(buf)/2)
		if n == 0 {
			break
		}
		for _, value := range buf[:n*2] {
			v := float64(value)
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatal("MPTM produced non-finite audio")
			}
		}
		for j, start := range starts {
			begin := max(frames, start*rate)
			end := min(frames+n, (start+5)*rate)
			for i := (begin - frames) * 2; i < (end-frames)*2; i++ {
				v := float64(buf[i])
				windowEnergy[j] += v * v
			}
		}
		frames += n
		if frames > 260*rate {
			t.Fatal("MPTM did not terminate near the expected duration")
		}
	}
	if math.Abs(float64(frames)/rate-255.991) > 0.1 {
		t.Fatalf("MPTM duration %.3fs, want approximately 255.991s", float64(frames)/rate)
	}
	for i, energy := range windowEnergy {
		if energy < 0.01 {
			t.Errorf("MPTM is silent near %ds: energy %g", starts[i], energy)
		}
	}
}
