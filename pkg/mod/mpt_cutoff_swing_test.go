package mod

import (
	"encoding/binary"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func TestMPTCutoffSwingValidation(t *testing.T) {
	inst := make([]ITInstrument, 2)
	if err := applyMPTCutoffSwing(inst, []uint8{26, 18}); err != nil ||
		inst[0].mptCutoffSwing != 26 || inst[1].mptCutoffSwing != 18 {
		t.Fatalf("cutoff variations %+v, error %v", inst, err)
	}
	before := append([]ITInstrument(nil), inst...)
	for _, invalid := range [][]uint8{{1}, {1, 65}} {
		if err := applyMPTCutoffSwing(inst, invalid); err == nil {
			t.Fatalf("accepted invalid values %v", invalid)
		}
		if !reflect.DeepEqual(inst, before) {
			t.Fatal("invalid values partially modified instruments")
		}
	}
}

func TestMPTRandomCutoff(t *testing.T) {
	for _, tt := range []struct {
		cutoff, swing uint8
		random        int
		want          uint8
	}{
		{94, 26, -128, 76}, {94, 26, 127, 113},
		{127, 64, -128, 65}, {127, 64, 127, 127},
		{0, 64, -128, 0}, {0, 64, 127, 0},
		{94, 26, -1, 94},
	} {
		if got := mptRandomCutoff(tt.cutoff, tt.swing, tt.random); got != tt.want {
			t.Errorf("%+v: got %d", tt, got)
		}
	}
}

func TestMPTCutoffSwingVoiceIsolation(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	h[17], h[58], h[59], h[304] = 1, 128|94, 128, 0
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyMPTCutoffSwing(loaded.Assets.ITInstruments, []uint8{64}); err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	prng := rand.New(rand.NewSource(1))
	var previous VoiceID = -1
	var previousCutoff uint8
	var previousFilter ITFilterState
	for note := uint8(61); note < 64; note++ {
		w.PendingCmds[0] = ModCommand{Note: note, Instrument: 1}
		VoiceAllocationSystem(w)
		v := w.TrackCursors[0].ActiveVoice
		want := mptRandomCutoff(94, 64, prng.Intn(256)-128)
		if w.VoiceLives[v].ITFilterCutoff != want || w.TrackCursors[0].ITFilterCutoff != 94 {
			t.Fatalf("note %d: voice cutoff %d, base %d, want %d", note, w.VoiceLives[v].ITFilterCutoff, w.TrackCursors[0].ITFilterCutoff, want)
		}
		expected := newITFilter(want, 0, 48000, true, false, false)
		if w.VoiceFilters[v] != expected {
			t.Fatalf("note %d: unexpected filter coefficients", note)
		}
		if w.VoiceRightFilters[v] != expected {
			t.Fatalf("note %d: stereo filter coefficients differ", note)
		}
		if previous != -1 && (!w.VoiceLives[previous].Active ||
			w.VoiceLives[previous].ITFilterCutoff != previousCutoff ||
			w.VoiceFilters[previous] != previousFilter ||
			w.VoiceRightFilters[previous] != previousFilter) {
			t.Fatalf("note %d: previous voice's cutoff or filters changed", note)
		}
		previous, previousCutoff, previousFilter = v, want, expected
	}
}

func TestOpenMPTOPLCutoffSwing(t *testing.T) {
	flags := []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}
	b := oplEnvelopeEndFixture(t, 0x01280100, flags, []byte{0})
	instrument := int(binary.LittleEndian.Uint32(b[194:]))
	b[instrument+304] = 0
	b[instrument+58], b[instrument+59] = 128|94, 128
	sample := int(binary.LittleEndian.Uint32(b[206:]))
	patchOffset := int(binary.LittleEndian.Uint32(b[sample+72:]))
	patch := oplPatch{0x21, 0x21, 0, 0, 0xf0, 0xf0, 0x0f, 0x0f, 0, 0, 0, 0}
	copy(b[patchOffset:patchOffset+12], patch[:])
	loaded, err := loadITData(b, true)
	if err != nil {
		t.Fatal(err)
	}

	loaded.Assets.ITInstruments[0].mptCutoffSwing = 64
	w := NewTracker(loaded.Assets, referenceRate, 5760)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 1, 125, -1
	actual := make([]float32, 5760*2)
	if n := w.Read(actual, 5760); n != 5760 {
		t.Fatalf("short render %d", n)
	}
	random := rand.New(rand.NewSource(1)).Intn(256) - 128
	cutoff := mptRandomCutoff(94, 64, random)
	reference := append([]byte(nil), b...)
	reference[instrument+58] = 128 | cutoff
	ref := referenceTransition(t, reference)
	base := append([]byte(nil), b...)
	base[instrument+58] = 255
	refBase := referenceTransition(t, base)
	baseLoaded, err := loadITData(base, true)
	if err != nil {
		t.Fatal(err)
	}
	baseWorld := NewTracker(baseLoaded.Assets, referenceRate, 5760)
	baseWorld.Clock.Speed, baseWorld.Clock.Tempo, baseWorld.Clock.Tick = 1, 125, -1
	actualBase := make([]float32, 5760*2)
	if n := baseWorld.Read(actualBase, 5760); n != 5760 {
		t.Fatalf("short baseline render %d", n)
	}
	actualRatio := rmsWindow(actual, 5, 15)
	refRatio := rmsWindow(ref, 5, 15)
	referenceBaseline, actualBaseline := rmsWindow(refBase, 5, 15), rmsWindow(actualBase, 5, 15)
	if refRatio < .001 || actualRatio < .001 || referenceBaseline < .001 || actualBaseline < .001 {
		t.Fatalf("silent OPL cutoff comparison: tracker %g reference %g", actualRatio, refRatio)
	}
	t.Logf("cutoff=%d tracker=%g/%g reference=%g/%g", cutoff, actualRatio, actualBaseline, refRatio, referenceBaseline)
	if math.Abs(actualRatio/actualBaseline-refRatio/referenceBaseline) > .08 {
		t.Fatalf("OPL cutoff relative RMS: tracker %g reference %g", actualRatio/actualBaseline, refRatio/referenceBaseline)
	}
}

func TestOpenMPTPCMCutoffSwing(t *testing.T) {
	b := oplEnvelopeEndFixture(t, 0x01280100, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}, []byte{0})
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	b[pattern+11] = 2
	b[instrument+58], b[instrument+59] = 128|94, 128
	loaded, err := loadITData(b, true)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Assets.ITInstruments[1].mptCutoffSwing = 64
	w := NewTracker(loaded.Assets, referenceRate, 5760)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 1, 125, -1
	actual := make([]float32, 5760*2)
	if n := w.Read(actual, 5760); n != 5760 {
		t.Fatalf("short render %d", n)
	}
	cutoff := mptRandomCutoff(94, 64, rand.New(rand.NewSource(1)).Intn(256)-128)
	refData := append([]byte(nil), b...)
	refData[instrument+58] = 128 | cutoff
	ref := referenceTransition(t, refData)
	base := append([]byte(nil), b...)
	base[instrument+58] = 255
	refBase := referenceTransition(t, base)
	baseLoaded, err := loadITData(base, true)
	if err != nil {
		t.Fatal(err)
	}
	baseWorld := NewTracker(baseLoaded.Assets, referenceRate, 5760)
	baseWorld.Clock.Speed, baseWorld.Clock.Tempo, baseWorld.Clock.Tick = 1, 125, -1
	actualBase := make([]float32, 5760*2)
	if n := baseWorld.Read(actualBase, 5760); n != 5760 {
		t.Fatalf("short baseline render %d", n)
	}
	gotBase, wantBase := rmsWindow(actualBase, 5, 15), rmsWindow(refBase, 5, 15)
	if gotBase < .001 || wantBase < .001 {
		t.Fatal("silent PCM baseline")
	}
	got, want := rmsWindow(actual, 5, 15)/gotBase, rmsWindow(ref, 5, 15)/wantBase
	if math.Abs(got-want) > .05 {
		t.Fatalf("PCM cutoff %d relative RMS: tracker %g, OpenMPT %g", cutoff, got, want)
	}
}

func TestLoadMPTMCutoffSwing(t *testing.T) {
	b := oplEnvelopeEndFixture(t, 0x01280100, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}, []byte{0})
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	properties := append([]byte("XTPM"), mptExtensionField("..SC", 1, []byte{26, 18})...)
	properties = append(properties, b[bounds.propertiesStart:bounds.serializedStart]...)
	for _, field := range []struct {
		code string
		data []byte
	}{
		{"...C", []byte{1, 0}}, {"..MT", []byte{2}},
		{".MMP", []byte{4, 0, 0, 0}}, {".APS", []byte{64, 0, 0, 0}}, {"VTSV", []byte{64, 0, 0, 0}},
	} {
		properties = append(properties, mptExtensionField(field.code, uint16(len(field.data)), field.data)...)
	}
	b[30], b[44] = 8, b[44]|8
	data := append([]byte(nil), b[:bounds.propertiesStart]...)
	data = append(data, properties...)
	tail := len(data)
	data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
	data = binary.LittleEndian.AppendUint32(data, uint32(tail))
	loaded, err := loadMPTM(data)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Assets.ITInstruments[0].mptCutoffSwing != 26 || loaded.Assets.ITInstruments[1].mptCutoffSwing != 18 {
		t.Fatal("loaded cutoff variation differs from extension")
	}
	if !loaded.Assets.ITCompatibility.oplNoResetAtEnvelopeEnd {
		t.Fatal("OPL envelope no-reset upgrade was not applied")
	}
	instrument := int(binary.LittleEndian.Uint32(data[194:]))
	data[instrument+468] = 0x81
	data[instrument+469] = 1
	if _, err := loadMPTM(data); err != nil {
		t.Fatalf("OPL filter envelope and swing: %v", err)
	}
	field := strings.Index(string(data), "..SC")
	if field < 0 {
		t.Fatal("cutoff variation field not found")
	}
	data[field+6] = 0
	pattern := int(binary.LittleEndian.Uint32(data[210:]))
	copy(data[pattern+8:], []byte{0x81, 8, 26, 48, 0})
	if _, err := loadMPTM(data); err != nil {
		t.Fatalf("OPL filter envelope and macro: %v", err)
	}
}
