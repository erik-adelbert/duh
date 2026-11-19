package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestMPTDuplicateSampleProjection(t *testing.T) {
	for _, tt := range []struct {
		name    string
		flags   mptSavedPlayBehaviours
		version uint32
		wantErr bool
	}{
		{"lower bound", 0, 0x01260000, false},
		{"target", 0x7ffffffffff81, 0x01280100, false},
		{"upper bound", 0, 0x01289999, false},
		{"old version", 0, 0x01259999, true},
		{"new compatibility export", 0, 0x01290000, true},
		{"new version", 0, 0x01290057, true},
		{"unknown flags", 1 << mptKnownPlayBehaviourCount, 0x01280100, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := projectMPTDuplicateSampleAcrossInstruments(tt.flags, tt.version)
			if (err != nil) != tt.wantErr || err == nil && !got {
				t.Fatalf("across instruments=%v, error=%v", got, err)
			}
		})
	}
}

func TestOpenMPTDuplicateSampleProjection(t *testing.T) {
	for _, version := range []uint32{0x01260000, 0x01280100, 0x01289999} {
		for _, nextInstrument := range []byte{1, 2} {
			t.Run(fmt.Sprintf("version=%08x/instrument=%d", version, nextInstrument), func(t *testing.T) {
				flags := mptSavedPlayBehaviours(0x7ffffffffff81)
				across, err := projectMPTDuplicateSampleAcrossInstruments(flags, version)
				if err != nil {
					t.Fatal(err)
				}
				fixture := func(dct, nna byte) ([]byte, *S3MLoadResult) {
					b := testITDuplicateSampleFixture(dct, nna, nna, false, nextInstrument)
					loaded, err := loadIT(b)
					if err != nil {
						t.Fatal(err)
					}
					binary.LittleEndian.PutUint16(b[40:], 0x891)
					patternEnd := 344 + int(binary.LittleEndian.Uint16(b[336:]))
					binary.LittleEndian.PutUint32(b[206:], uint32(len(b)))
					b = append(b, b[336:patternEnd]...)
					b = append(b, []byte("STPM")...)
					b = append(b, mptExtensionField("VWSL", 4, binary.LittleEndian.AppendUint32(nil, version))...)
					flagBytes := make([]byte, 7)
					for i := range flagBytes {
						flagBytes[i] = byte(uint64(flags) >> (8 * i))
					}
					b = append(b, mptExtensionField(".FSM", 7, flagBytes)...)
					tail := len(b)
					b = append(b, mptTestSequences([]mptTestEntry{
						{"l", []byte{1, 0}}, {"a", []byte{0, 0}},
					})...)
					b = binary.LittleEndian.AppendUint32(b, uint32(tail))
					bounds, err := locateMPTExtensions(b)
					if err != nil {
						t.Fatal(err)
					}
					ext, err := parseMPTExtensions(b[bounds.propertiesStart:bounds.serializedStart], 2)
					if err != nil || ext.lastSavedVersion == nil || *ext.lastSavedVersion != version {
						t.Fatalf("fixture extensions: %+v, error %v", ext, err)
					}
					parsed, err := parseMPTSavedPlayBehaviours(ext.playbackFlags)
					if err != nil || parsed != flags {
						t.Fatalf("fixture flags = %#x, error %v, want %#x", parsed, err, flags)
					}
					return b, loaded
				}
				module, loaded := fixture(2, 1)
				baseline, baselineLoaded := fixture(0, 0)
				ref := renderMPTSequenceReference(t, module, "--repeat", "-1", "--end-time", "0.120")
				refBase := renderMPTSequenceReference(t, baseline, "--repeat", "-1", "--end-time", "0.120")
				if len(ref) < 11520 || len(refBase) < 11520 {
					t.Fatal("short OpenMPT render")
				}
				render := func(module *S3MLoadResult, acrossInstruments bool) []float32 {
					assets := module.Assets
					assets.ITCompatibility.itDCTSampleAcrossInstruments = acrossInstruments
					w := NewTracker(assets, referenceRate, 960)
					w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = module.InitialSpeed, module.InitialTempo, -1
					out := make([]float32, 11520)
					for i := 0; i < len(out)/2; i += 960 {
						if got := w.Read(out[i*2:(i+960)*2], 960); got != 960 {
							t.Fatalf("short render at frame %d: %d", i, got)
						}
					}
					return out
				}
				actual, strict, actualBase := render(loaded, across), render(loaded, false), render(baselineLoaded, false)
				for _, ms := range []int{28, 68, 108} {
					refRatio := channelRMS(ref, ms, ms+5, 0) / channelRMS(refBase, ms, ms+5, 0)
					actualRatio := channelRMS(actual, ms, ms+5, 0) / channelRMS(actualBase, ms, ms+5, 0)
					if math.IsNaN(refRatio) || math.IsNaN(actualRatio) || math.Abs(actualRatio-refRatio) > .05 {
						t.Errorf("%dms: OpenMPT ratio %.3f, tracker %.3f", ms, refRatio, actualRatio)
					}
				}
				if nextInstrument == 2 {
					cutLevel := channelRMS(actual, 68, 73, 0)
					strictLevel := channelRMS(strict, 68, 73, 0)
					if cutLevel < .001 || strictLevel/cutLevel < 1.5 {
						t.Fatalf("fixture does not distinguish cross-instrument cuts: cut %g strict %g", cutLevel, strictLevel)
					}
				}
			})
		}
	}
}
