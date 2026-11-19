package mod

import (
	"encoding/binary"
	"math"
	"reflect"
	"strings"
	"testing"
)

func mptExtraExtensionsFixture(t *testing.T, instrumentFields, songFields []byte) []byte {
	t.Helper()
	b := mptResamplingFixture(t, 1, 60, false)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	data := append([]byte(nil), b[:bounds.propertiesStart+4]...)
	data = append(data, instrumentFields...)
	data = append(data, b[bounds.propertiesStart+4:bounds.serializedStart]...)
	data = append(data, songFields...)
	tail := len(data)
	data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(data, uint32(tail))
}

func mptTestCue(sample uint16, points ...uint32) []byte {
	field := binary.LittleEndian.AppendUint16(nil, sample)
	for _, point := range points {
		field = binary.LittleEndian.AppendUint32(field, point)
	}
	return mptExtensionField("CUES", uint16(len(field)), field)
}

func TestMPTExtraExtensions(t *testing.T) {
	b := append([]byte("XTPM"), mptExtensionField("HEVP", 1, []byte{0, 1})...)
	b = append(b, mptExtensionField("HOVP", 1, []byte{2, 3})...)
	b = append(b, []byte("STPM")...)
	b = append(b, mptExtensionField(".BPR", 4, []byte{8, 0, 0, 0})...)
	b = append(b, mptExtensionField(".MPR", 4, []byte{32, 0, 0, 0})...)
	b = append(b, mptTestCue(1, 123, 456)...)
	b = append(b, mptTestCue(2, 789)...)
	ext, err := parseMPTExtensions(b, 2)
	if err != nil {
		t.Fatal(err)
	}
	clear(b)
	if !reflect.DeepEqual(ext.instrumentPluginVelocity, []uint8{0, 1}) ||
		!reflect.DeepEqual(ext.instrumentPluginVolume, []uint8{2, 3}) ||
		*ext.rowsPerBeat != 8 || *ext.rowsPerMeasure != 32 ||
		!reflect.DeepEqual(ext.sampleCues, map[uint16][]uint32{1: {123, 456}, 2: {789}}) {
		t.Fatalf("unexpected extensions: %+v", ext)
	}
	for _, tt := range []struct {
		name string
		data []byte
		want string
	}{
		{"velocity width", append([]byte("XTPM"), mptExtensionField("HEVP", 2, make([]byte, 4))...), "invalid size"},
		{"volume width", append([]byte("XTPM"), mptExtensionField("HOVP", 2, make([]byte, 4))...), "invalid size"},
		{"beat width", append([]byte("STPM"), mptExtensionField(".BPR", 2, make([]byte, 2))...), "invalid size"},
		{"measure width", append([]byte("STPM"), mptExtensionField(".MPR", 2, make([]byte, 2))...), "invalid size"},
		{"cue empty", append([]byte("STPM"), mptExtensionField("CUES", 2, []byte{1, 0})...), "invalid size"},
		{"cue malformed", append([]byte("STPM"), mptExtensionField("CUES", 7, make([]byte, 7))...), "invalid size"},
		{"cue too long", append([]byte("STPM"), mptExtensionField("CUES", 42, make([]byte, 42))...), "invalid size"},
		{"cue zero sample", append([]byte("STPM"), mptTestCue(0, 10)...), "sample index 0"},
		{"cue repeated sample", append(append([]byte("STPM"), mptTestCue(1, 10)...), mptTestCue(1, 20)...), "duplicate MPTM cues"},
		{"cue truncated", append([]byte("STPM"), mptExtensionField("CUES", 6, []byte{1, 0})...), "truncated"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseMPTExtensions(tt.data, 2); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
}

func TestMPTExtraExtensionsLoader(t *testing.T) {
	for _, tt := range []struct {
		name       string
		instrument []byte
		song       []byte
		want       string
	}{
		{"supported", append(mptExtensionField("HEVP", 1, []byte{0, 1}), mptExtensionField("HOVP", 1, []byte{2, 3})...), append(mptTestCue(1, 100), mptTestCue(2, 200)...), ""},
		{"invalid velocity", mptExtensionField("HEVP", 1, []byte{0, 2}), nil, "instrument 2 uses unsupported plugin velocity"},
		{"invalid volume", mptExtensionField("HOVP", 1, []byte{2, 4}), nil, "instrument 2 uses unsupported plugin volume"},
		{"missing sample", nil, mptTestCue(3, 10), "nonexistent sample 3"},
		{"beat out of range", nil, mptExtensionField(".BPR", 4, []byte{0, 0, 1, 0}), "unsupported MPTM rows per beat"},
		{"measure out of range", nil, mptExtensionField(".MPR", 4, []byte{1, 0, 1, 0}), "unsupported MPTM rows per measure"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadMPTM(mptExtraExtensionsFixture(t, tt.instrument, tt.song))
			if tt.want == "" && err != nil || tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}

func TestMPTExtraExtensionsModernClock(t *testing.T) {
	b := mptExtraExtensionsFixture(t, nil, mptExtensionField(".BPR", 4, []byte{8, 0, 0, 0}))
	b[bytesIndex(t, b, "..MT")+6] = 2
	b[30], b[46] = 4, b[46]|4
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	length := int(binary.LittleEndian.Uint16(b[pattern:]))
	clear(b[pattern+8 : pattern+8+length])
	binary.LittleEndian.PutUint16(b[pattern+2:], 32)
	copy(b[pattern+8:], []byte{0x81, 3, 60, 2, 0, 0x81, 1, 254, 0})
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Assets.mptRowsPerBeat != 8 {
		t.Fatalf("rows per beat %d, want 8", loaded.Assets.mptRowsPerBeat)
	}
	ref, actual := mptSharedEffectRenderForDuration(t, b, 130)
	for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
		if rmsWindow(pcm, 30, 50) < .001 || rmsWindow(pcm, 75, 100) > .0001 {
			t.Fatalf("%s: unexpected note-cut timing, before=%g after=%g", name, rmsWindow(pcm, 30, 50), rmsWindow(pcm, 75, 100))
		}
	}
}

func TestOpenMPTExtraExtensionsInert(t *testing.T) {
	instrument := append(mptExtensionField("HEVP", 1, []byte{0, 1}), mptExtensionField("HOVP", 1, []byte{2, 3})...)
	song := append(mptExtensionField(".MPR", 4, []byte{16, 0, 0, 0}), mptTestCue(1, 42, 256)...)
	ref, actual := mptSharedEffectRenderForDuration(t, mptExtraExtensionsFixture(t, instrument, song), 100)
	baseRef, baseActual := mptSharedEffectRenderForDuration(t, mptExtraExtensionsFixture(t, nil, nil), 100)
	var energy float64
	for i := range ref {
		energy += float64(ref[i]) * float64(ref[i])
		if math.Abs(float64(ref[i]-baseRef[i])) > 1e-6 || actual[i] != baseActual[i] {
			t.Fatalf("sample %d: extensions changed playback", i)
		}
	}
	if energy < .001 {
		t.Fatal("reference baseline is silent")
	}
}

func TestMPTExtensionValidationOrder(t *testing.T) {
	ext := mptExtensions{
		instrumentMIDIProgram:      []uint8{1},
		instrumentMIDIBank:         []uint16{1},
		instrumentFilterMode:       []uint8{2},
		instrumentPitchReleaseNode: []uint8{0},
	}
	for _, want := range []string{"extended MIDI program", "extended MIDI bank", "filter mode", "pitch-envelope release node"} {
		err := ext.validatePlaybackFields(0)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("got %v, want %s", err, want)
		}
		switch want {
		case "extended MIDI program":
			ext.instrumentMIDIProgram[0] = 0
		case "extended MIDI bank":
			ext.instrumentMIDIBank[0] = 0
		case "filter mode":
			ext.instrumentFilterMode[0] = 0xff
		case "pitch-envelope release node":
			ext.instrumentPitchReleaseNode[0] = 0xff
		}
	}
	if err := ext.validatePlaybackFields(0); err != nil {
		t.Fatal(err)
	}
}
