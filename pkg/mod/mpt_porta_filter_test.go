package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func mptPortaFilterFixture(t *testing.T, enabled bool) []byte {
	t.Helper()
	b := mptmPlaybackFixture()
	b[50] = 2
	b[bytesIndex(t, b, "..MT")+6] = 0
	binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01310400)
	first, second := int(binary.LittleEndian.Uint32(b[194:])), int(binary.LittleEndian.Uint32(b[198:]))
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	copy(b[second:second+80], b[first:first+80])
	pcm := int(binary.LittleEndian.Uint32(b[first+72:]))
	samples := append([]byte(nil), b[pcm:pcm+4096*2]...)
	withSample := append([]byte(nil), b[:bounds.propertiesStart]...)
	withSample = append(withSample, samples...)
	withSample = append(withSample, b[bounds.propertiesStart:]...)
	binary.LittleEndian.PutUint32(withSample[second+72:], uint32(bounds.propertiesStart))
	binary.LittleEndian.PutUint32(withSample[len(withSample)-4:], uint32(bounds.serializedStart+len(samples)))
	b = withSample
	pattern := int(binary.LittleEndian.Uint32(b[202:]))
	rows := []byte{0x81, 11, 72, 1, 26, 30, 0, 0x81, 11, 84, 2, 7, 16, 0}
	rows = append(rows, make([]byte, 62)...)
	binary.LittleEndian.PutUint16(b[pattern:], uint16(len(rows)))
	copy(b[pattern+8:], rows)
	bounds, err = locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	field := bytesIndex(t, b, ".FSM")
	flags := make([]byte, 15)
	copy(flags, b[field+6:field+13])
	if enabled {
		flags[mptITResetFilterOnPortaBit/8] |= 1 << (mptITResetFilterOnPortaBit % 8)
	}
	out := append([]byte(nil), b[:field]...)
	out = append(out, mptExtensionField(".FSM", uint16(len(flags)), flags)...)
	out = append(out, b[field+13:bounds.serializedStart]...)
	tail := len(out)
	out = append(out, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(out, uint32(tail))
}

func TestOpenMPTPortaSampleFilterReset(t *testing.T) {
	var reference, rendered [2][]float32
	for i := range 2 {
		b := mptPortaFilterFixture(t, i == 1)
		ref, actual := mptSharedEffectRenderForDuration(t, b, 100)
		reference[i], rendered[i] = ref, actual
	}
	var difference, actualDifference, cross float64
	for i := 40 * 96; i < 42*96; i++ {
		d := float64(reference[0][i] - reference[1][i])
		a := float64(rendered[0][i] - rendered[1][i])
		difference += d * d
		actualDifference += a * a
		cross += d * a
	}
	if math.IsNaN(difference) || difference < 1e-8 {
		t.Fatal("reference fixture did not distinguish filter reset behavior")
	}
	correlation := cross / math.Sqrt(difference*actualDifference)
	ratio := math.Sqrt(actualDifference / difference)
	if math.IsNaN(correlation) || correlation < .99 || ratio < .98 || ratio > 1.02 {
		t.Fatalf("filter transient: correlation %g, RMS ratio %g", correlation, ratio)
	}
}

func TestMPTPortaSampleFilterResetState(t *testing.T) {
	for _, tt := range []struct {
		enabled        bool
		sameSample     bool
		instrumentMode bool
		reset          bool
	}{
		{false, false, false, false},
		{true, false, false, true},
		{true, true, false, false},
		{true, false, true, false},
	} {
		loaded, err := loadMPTM(mptPortaFilterFixture(t, tt.enabled))
		if err != nil {
			t.Fatal(err)
		}
		if tt.instrumentMode {
			loaded.Assets.ITInstruments = make([]ITInstrument, 2)
			loaded.Assets.ITInstruments[1].Samples[83] = 2
		}
		w := NewTracker(loaded.Assets, referenceRate, 256)
		cursor := &w.TrackCursors[0]
		cursor.ActiveVoice = 0
		w.VoiceLives[0] = VoiceLifecycle{Active: true, ITFilterCutoff: 30}
		w.VoiceSamples[0].SampleID = 0
		w.VoiceFilters[0] = newITFilter(30, 0, referenceRate, true, false, false)
		w.VoiceRightFilters[0] = w.VoiceFilters[0]
		w.VoiceFilters[0].Y1, w.VoiceRightFilters[0].Y1 = .5, -.5
		instrument := uint8(2)
		if tt.sameSample {
			instrument = 1
		}
		w.applyITPortamentoSampleChange(cursor, ModCommand{Note: 84, Instrument: instrument, EffectCmd: 3})
		if reset := w.VoiceFilters[0].Y1 == 0 && w.VoiceRightFilters[0].Y1 == 0; reset != tt.reset {
			t.Fatalf("%+v: left/right filter history %g/%g", tt, w.VoiceFilters[0].Y1, w.VoiceRightFilters[0].Y1)
		}
	}
}

func TestMPTPortaFilterFlagVersion(t *testing.T) {
	for _, tt := range []struct {
		version uint32
		enabled bool
	}{
		{0x01290000, false}, {0x01300000, true}, {0x01300801, false},
		{0x01300802, true}, {0x01310400, true},
	} {
		b := mptPortaFilterFixture(t, true)
		binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], tt.version)
		loaded, err := loadMPTM(b)
		if err != nil {
			t.Fatal(err)
		}
		if got := loaded.Assets.ITCompatibility.mptResetFilterOnPorta; got != tt.enabled {
			t.Errorf("version %#x: enabled=%v, want %v", tt.version, got, tt.enabled)
		}
	}
}
