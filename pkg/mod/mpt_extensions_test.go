package mod

import (
	"encoding/binary"
	"os"
	"reflect"
	"strings"
	"testing"
)

func mptExtensionField(code string, size uint16, value []byte) []byte {
	b := append([]byte(code), byte(size), byte(size>>8))
	return append(b, value...)
}

func mptExtensionFixture() []byte {
	b := []byte("XTPM")
	b = append(b, mptExtensionField("..OF", 4, []byte{0, 32, 0, 0, 5, 13, 0, 0})...)
	b = append(b, mptExtensionField("DWPM", 1, []byte{0, 2})...)
	b = append(b, []byte("STPM")...)
	b = append(b, mptExtensionField("...C", 2, []byte{16, 0})...)
	b = append(b, mptExtensionField("..MT", 1, []byte{2})...)
	b = append(b, mptExtensionField(".MMP", 4, []byte{4, 0, 0, 0})...)
	b = append(b, mptExtensionField("VWSL", 4, []byte{0, 1, 0x28, 1})...)
	b = append(b, mptExtensionField(".APS", 4, []byte{64, 0, 0, 0})...)
	b = append(b, mptExtensionField("VTSV", 4, []byte{64, 0, 0, 0})...)
	b = append(b, mptExtensionField("SWNG", 10, []byte{2, 0, 0, 0, 0, 1, 0, 0, 0, 1})...)
	b = append(b, mptExtensionField(".FSM", 3, []byte{0x81, 0xff, 7})...)
	b = append(b, mptExtensionField("AUTH", 4, []byte("Pigu"))...)
	return b
}

func TestParseMPTExtensions(t *testing.T) {
	data := mptExtensionFixture()
	got, err := parseMPTExtensions(data, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.instrumentFadeout, []uint32{8192, 3333}) ||
		!reflect.DeepEqual(got.instrumentMIDIPWD, []int8{0, 2}) ||
		got.channels == nil || *got.channels != 16 ||
		got.tempoMode == nil || *got.tempoMode != 2 ||
		got.mixLevels == nil || *got.mixLevels != 4 ||
		got.lastSavedVersion == nil || *got.lastSavedVersion != 0x01280100 ||
		got.samplePreamp == nil || *got.samplePreamp != 64 ||
		got.pluginVolume == nil || *got.pluginVolume != 64 ||
		!reflect.DeepEqual(got.tempoSwing, []uint32{1 << 24, 1 << 24}) ||
		!reflect.DeepEqual(got.playbackFlags, []byte{0x81, 0xff, 7}) ||
		got.artist == nil || *got.artist != "Pigu" {
		t.Fatalf("unexpected extensions: %+v", got)
	}
	for i := range data {
		data[i] = 0
	}
	if got.playbackFlags[0] != 0x81 || *got.artist != "Pigu" {
		t.Fatal("parsed values unexpectedly alias input")
	}
}

func TestMPTFilterSwingExtensions(t *testing.T) {
	for _, tt := range []struct {
		name, code, want string
		values           []byte
	}{
		{"neutral cutoff", "..SC", "", []byte{0, 0}},
		{"neutral resonance", "..SR", "", []byte{0, 0}},
		{"cutoff variation", "..SC", "", []byte{0, 18}},
		{"resonance variation", "..SR", "", []byte{0, 26}},
		{"invalid resonance variation", "..SR", "instrument 2 resonance variation 65", []byte{0, 65}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := append([]byte("XTPM"), mptExtensionField(tt.code, 1, tt.values)...)
			ext, err := parseMPTExtensions(data, 2)
			if err != nil {
				t.Fatal(err)
			}
			data[len(data)-1] ^= 0xff
			got := ext.instrumentCutSwing
			if tt.code == "..SR" {
				got = ext.instrumentResSwing
			}
			if !reflect.DeepEqual(got, tt.values) {
				t.Fatalf("variation values = %v, want %v", got, tt.values)
			}
			if tt.code == "..SC" {
				err = applyMPTCutoffSwing(make([]ITInstrument, 2), ext.instrumentCutSwing)
			} else {
				err = applyMPTResonanceSwing(make([]ITInstrument, 2), ext.instrumentResSwing)
			}
			if tt.want == "" && err != nil || tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("validation error = %v, want %q", err, tt.want)
			}
		})
	}
	for _, code := range []string{"..SC", "..SR"} {
		data := append([]byte("XTPM"), mptExtensionField(code, 2, []byte{0, 0, 0, 0})...)
		if _, err := parseMPTExtensions(data, 2); err == nil || !strings.Contains(err.Error(), "invalid size") {
			t.Errorf("%s: expected invalid size, got %v", code, err)
		}
	}
}

func TestMPTInstrumentPluginExtension(t *testing.T) {
	data := append([]byte("XTPM"), mptExtensionField(".PiM", 1, []byte{0, 3})...)
	ext, err := parseMPTExtensions(data, 2)
	if err != nil || !reflect.DeepEqual(ext.instrumentPlugins, []uint8{0, 3}) {
		t.Fatalf("plugin assignments %v, error %v", ext.instrumentPlugins, err)
	}
	data[len(data)-1] = 0
	if ext.instrumentPlugins[1] != 3 {
		t.Fatal("plugin assignments alias input")
	}
	invalid := append([]byte("XTPM"), mptExtensionField(".PiM", 2, []byte{0, 0, 0, 0})...)
	if _, err := parseMPTExtensions(invalid, 2); err == nil || !strings.Contains(err.Error(), "invalid size") {
		t.Fatalf("expected invalid size, got %v", err)
	}
}

func TestMPTExtendedTempoAndCreatedVersion(t *testing.T) {
	data := append([]byte("STPM"), mptExtensionField("..TD", 4, []byte{0xd4, 3, 0, 0})...)
	data = append(data, mptExtensionField(".VWC", 4, []byte{0, 7, 0x29, 1})...)
	got, err := parseMPTExtensions(data, 0)
	if err != nil || got.defaultTempo == nil || *got.defaultTempo != 980 ||
		got.createdVersion == nil || *got.createdVersion != 0x01290700 {
		t.Fatalf("extensions = %+v, error %v", got, err)
	}
	for _, code := range []string{"..TD", ".VWC"} {
		invalid := append([]byte("STPM"), mptExtensionField(code, 2, []byte{0, 0})...)
		if _, err := parseMPTExtensions(invalid, 0); err == nil || !strings.Contains(err.Error(), "invalid size") {
			t.Errorf("%s: expected invalid size, got %v", code, err)
		}
	}
}

func TestParseMPTExtensionsRejectsInvalid(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		n    int
		want string
	}{
		{"missing header", []byte("invalid"), 1, "missing"},
		{"truncated header", []byte("STPM..."), 1, "truncated"},
		{"truncated instrument payload", append([]byte("XTPM"), mptExtensionField("..OF", 4, []byte{1, 2, 3, 4})...), 2, "truncated"},
		{"truncated song payload", append([]byte("STPM"), mptExtensionField("..MT", 2, []byte{2})...), 0, "truncated"},
		{"invalid instrument size", append([]byte("XTPM"), mptExtensionField("..OF", 2, []byte{0, 0})...), 1, "invalid size"},
		{"zero instrument count", append([]byte("XTPM"), mptExtensionField("DWPM", 1, []byte{0})...), 0, "invalid"},
		{"invalid tempo size", append([]byte("STPM"), mptExtensionField("..MT", 2, []byte{2, 0})...), 0, "invalid size"},
		{"invalid swing count", append([]byte("STPM"), mptExtensionField("SWNG", 2, []byte{1, 0})...), 0, "invalid size"},
		{"unknown instrument field", append([]byte("XTPM"), mptExtensionField("????", 1, []byte{1})...), 1, "unsupported"},
		{"unknown song field", append([]byte("STPM"), mptExtensionField("????", 1, []byte{1})...), 0, "unsupported"},
		{"duplicate field", append(append([]byte("STPM"), mptExtensionField("..MT", 1, []byte{2})...), mptExtensionField("..MT", 1, []byte{2})...), 0, "duplicate"},
		{"duplicate song header", []byte("STPMSTPM"), 0, "duplicate"},
		{"instrument header after song", []byte("STPMXTPM"), 1, "unexpected"},
		{"negative instruments", []byte("STPM"), -1, "instrument count"},
		{"too many instruments", []byte("STPM"), 256, "instrument count"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseMPTExtensions(tt.data, tt.n)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestParseTargetMPTExtensions(t *testing.T) {
	path := os.Getenv("MPTECS_MPTM_TARGET")
	if path == "" {
		t.Skip("set MPTECS_MPTM_TARGET to the Triangel Island fixture")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 192 || !isMPTM(b) {
		t.Fatal("not a supported MPTM fixture header")
	}
	u16 := func(i int) int { return int(binary.LittleEndian.Uint16(b[i:])) }
	instruments, samples := u16(34), u16(36)
	if instruments != 19 || samples != 18 {
		t.Fatal("unexpected target pointer table")
	}
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	if bounds.propertiesStart != 259310 || bounds.serializedStart != 259543 || bounds.serializedEnd != len(b)-4 {
		t.Fatalf("unexpected extension bounds: %+v", bounds)
	}
	got, err := parseMPTExtensions(b[bounds.propertiesStart:bounds.serializedStart], instruments)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.instrumentFadeout) != 19 || got.instrumentFadeout[9] != 3333 ||
		len(got.instrumentMIDIPWD) != 19 ||
		got.channels == nil || *got.channels != 16 ||
		got.tempoMode == nil || *got.tempoMode != 2 ||
		got.mixLevels == nil || *got.mixLevels != 4 ||
		got.lastSavedVersion == nil || *got.lastSavedVersion != 0x01280100 ||
		got.samplePreamp == nil || *got.samplePreamp != 64 ||
		got.pluginVolume == nil || *got.pluginVolume != 64 ||
		!reflect.DeepEqual(got.tempoSwing, []uint32{1 << 24, 1 << 24, 1 << 24, 1 << 24, 1 << 24, 1 << 24, 1 << 24, 1 << 24}) ||
		!reflect.DeepEqual(got.playbackFlags, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}) ||
		got.artist == nil || *got.artist != "Pigu" {
		t.Fatalf("unexpected target extension values: %+v", got)
	}
	targetInstruments := make([]ITInstrument, instruments)
	if err := applyMPTInstrumentFadeouts(targetInstruments, got.instrumentFadeout); err != nil {
		t.Fatal(err)
	}
	if !targetInstruments[9].hasMPTFadeout || targetInstruments[9].fadeoutStep() != float32(3333)/32768 {
		t.Fatal("target instrument 10 extended fadeout lost precision")
	}
	mix, err := projectMPTMixProfile(*got.mixLevels, *got.samplePreamp, *got.pluginVolume)
	if err != nil || mix != (mptMixProfile{pcmGain: .5, oplGain: 8225.0 / 4096}) {
		t.Fatalf("target relative mix profile: %+v, error %v", mix, err)
	}
	rowsPerBeat, err := projectMPTModernRowsPerBeat(got.tempoMode, uint16(b[30]))
	if err != nil || rowsPerBeat != 8 {
		t.Fatalf("target modern tempo rows per beat = %d, error %v", rowsPerBeat, err)
	}
	flags, err := parseMPTSavedPlayBehaviours(got.playbackFlags)
	if err != nil {
		t.Fatal(err)
	}
	for i := range mptKnownPlayBehaviourCount {
		if want := i == mptCompatiblePlay || i >= mptPeriodsAreHertz; flags.enabled(i) != want {
			t.Fatalf("target saved behaviour bit %d = %v, want %v", i, flags.enabled(i), want)
		}
	}
	if *got.lastSavedVersion != 0x01280100 || u16(44)&8 == 0 {
		t.Fatal("unexpected target version or pitch-slide mode")
	}
	sampleTable := 192 + u16(32) + instruments*4
	hasOPL := false
	for i := range samples {
		offset := int(binary.LittleEndian.Uint32(b[sampleTable+i*4:]))
		if offset != 0 && b[offset+46] == 0x40 {
			hasOPL = true
		}
	}
	projected, err := projectMPTBehaviours(flags, *got.lastSavedVersion, u16(44)&8 != 0, hasOPL, instruments != 0)
	if err != nil {
		t.Fatal(err)
	}
	if projected.known != flags || !projected.oplNoResetAtEnvelopeEnd {
		t.Fatalf("unexpected target behaviour projection: %+v", projected)
	}
	loop, err := projectMPTLoopProfile(flags, *got.lastSavedVersion, false)
	if err != nil {
		t.Fatal(err)
	}
	if loop != (itPatternLoopProfile{true, true, true, itJumpAlwaysWins}) {
		t.Fatalf("unexpected target loop profile: %+v", loop)
	}
	panbrello, err := projectMPTPanbrelloProfile(flags, *got.lastSavedVersion)
	if err != nil || panbrello != (itPanbrelloProfile{preciseWaveform: true, hold: true, panOverride: true, sampleAndHold: true}) {
		t.Fatalf("unexpected target panbrello profile: %+v, error %v", panbrello, err)
	}
	acrossInstruments, err := projectMPTDuplicateSampleAcrossInstruments(flags, *got.lastSavedVersion)
	if err != nil || !acrossInstruments {
		t.Fatalf("unexpected target duplicate-sample projection: %v, error %v", acrossInstruments, err)
	}
}

func FuzzParseMPTExtensions(f *testing.F) {
	f.Add(mptExtensionFixture(), uint8(2))
	f.Add([]byte("STPM"), uint8(0))
	f.Fuzz(func(t *testing.T, data []byte, count uint8) {
		_, _ = parseMPTExtensions(data, int(count))
	})
}
