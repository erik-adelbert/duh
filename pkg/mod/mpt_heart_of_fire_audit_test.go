package mod

import (
	"encoding/binary"
	"math"
	"os"
	"testing"
)

func TestAuditHeartOfFireMPTM(t *testing.T) {
	path := os.Getenv("MPTECS_MPTM_HEART_OF_FIRE")
	if path == "" {
		t.Skip("set MPTECS_MPTM_HEART_OF_FIRE to the 486 Heart of Fire fixture")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bounds, err := locateMPTExtensions(data)
	if err != nil {
		t.Fatal(err)
	}
	instruments := int(binary.LittleEndian.Uint16(data[34:]))
	ext, err := parseMPTExtensions(data[bounds.propertiesStart:bounds.serializedStart], instruments)
	if err != nil {
		t.Fatal(err)
	}
	if ext.defaultTempo == nil || *ext.defaultTempo != 980 || ext.tempoMode == nil || *ext.tempoMode != 0 ||
		ext.createdVersion == nil || *ext.createdVersion != 0x01290700 ||
		ext.lastSavedVersion == nil || *ext.lastSavedVersion != 0x01300600 ||
		ext.mixLevels == nil || *ext.mixLevels != 3 || len(ext.instrumentCutSwing) != 25 ||
		ext.instrumentCutSwing[0] != 26 || ext.instrumentCutSwing[15] != 18 {
		t.Fatalf("unexpected 486 Heart of Fire extensions: %+v", ext)
	}
	_, additional, err := parseMPTPlaybackProfileFlags(ext.playbackFlags)
	if err != nil || additional != (mptAdditionalPlayBehaviours{
		oplFlexibleNoteOff: true, itRestoreChannelPan: true, itDuplicateCheck: true,
		oplWithNNA: true, oplNoResetAtEnvelopeEnd: true,
	}) {
		t.Fatalf("unexpected 486 playback flags: %+v, error %v", additional, err)
	}
	seq, err := parseMPTSequences(data[bounds.serializedStart:bounds.serializedEnd], int(binary.LittleEndian.Uint16(data[38:])))
	if err != nil || len(seq.sequences) != 1 {
		t.Fatalf("sequence: %+v, error %v", seq, err)
	}
	instrumentTable := 192 + int(binary.LittleEndian.Uint16(data[32:]))
	samples := int(binary.LittleEndian.Uint16(data[36:]))
	parsed := make([]ITInstrument, instruments)
	for i := range parsed {
		ptr := int(binary.LittleEndian.Uint32(data[instrumentTable+i*4:]))
		if ptr == 0 {
			t.Fatalf("missing instrument %d", i+1)
		}
		parsed[i], err = parseITInstrument(data[ptr:ptr+554], samples)
		if err != nil {
			t.Fatalf("instrument %d: %v", i+1, err)
		}
	}
	if err := applyMPTCutoffSwing(parsed, ext.instrumentCutSwing); err != nil {
		t.Fatal(err)
	}
	if parsed[0].mptCutoffSwing != 26 || parsed[15].mptCutoffSwing != 18 {
		t.Fatal("target cutoff variation was not applied")
	}
	legacy, err := mptLegacySequenceDefaults(data[51], data[50], ext.defaultTempo, ext.defaultTempoFraction)
	if err != nil {
		t.Fatal(err)
	}
	defaults, err := resolveMPTSequenceDefaults(seq.sequences[0], legacy)
	if err != nil || defaults.tempo != 9800000 || defaults.speed != 10 {
		t.Fatalf("effective defaults: %+v, error %v", defaults, err)
	}
	loaded, err := loadMPTM(data)
	if err != nil {
		t.Fatalf("loading target: %v", err)
	}
	w := NewTracker(loaded.Assets, referenceRate, 4096)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	actual := make([]float32, 5*referenceRate*2)
	if n := w.Read(actual, 5*referenceRate); n != 5*referenceRate {
		t.Fatalf("target render stopped after %d frames", n)
	}
	for _, v := range actual {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatal("target render contains non-finite samples")
		}
	}
	ref := renderMPTSequenceReference(t, data, "--end-time", "5")
	for second := range 5 {
		start, end := second*1000, (second+1)*1000
		got, want := rmsWindow(actual, start, end), rmsWindow(ref, start, end)
		if got < .00001 || want < .00001 {
			t.Fatalf("second %d is silent: tracker %g, OpenMPT %g", second, got, want)
		}
		t.Logf("second %d RMS: tracker %.4f, OpenMPT %.4f", second, got, want)
	}
}
