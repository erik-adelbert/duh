package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestMPTDuplicateSampleBehaviourUpgrade(t *testing.T) {
	for _, tt := range []struct {
		version uint32
		strict  bool
		across  bool
	}{
		{0x01280100, true, true},
		{0x01290000, true, false},
		{0x01290001, true, true},
		{0x01290056, true, true},
		{0x01290057, true, false},
		{0x01300600, true, false},
		{0x01300600, false, true},
	} {
		across, err := projectMPTDuplicateSampleBehaviour(0, tt.version, tt.strict)
		if err != nil || across != tt.across {
			t.Errorf("version %#x strict %v: across %v error %v", tt.version, tt.strict, across, err)
		}
	}
	if across, err := projectMPTDuplicateSampleBehaviour(0, 0x01321201, true); err != nil || across {
		t.Fatalf("future version: across %v error %v", across, err)
	}
}

func TestOpenMPTModernDuplicateSampleBehaviour(t *testing.T) {
	for _, tt := range []struct {
		version uint32
		strict  bool
	}{
		{0x01290000, true},
		{0x01290001, true},
		{0x01290056, true},
		{0x01290057, true},
		{0x01300600, true},
		{0x01300600, false},
	} {
		for _, nextInstrument := range []byte{1, 2} {
			t.Run(fmt.Sprintf("%08x/strict=%t/instrument=%d", tt.version, tt.strict, nextInstrument), func(t *testing.T) {
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
					b = append(b, mptExtensionField("VWSL", 4, binary.LittleEndian.AppendUint32(nil, tt.version))...)
					flags := make([]byte, 14)
					copy(flags, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7})
					if tt.strict {
						flags[13] = 1
					}
					b = append(b, mptExtensionField(".FSM", uint16(len(flags)), flags)...)
					tail := len(b)
					b = append(b, mptTestSequences([]mptTestEntry{{"l", []byte{1, 0}}, {"a", []byte{0, 0}}})...)
					b = binary.LittleEndian.AppendUint32(b, uint32(tail))
					return b, loaded
				}
				module, loaded := fixture(2, 1)
				base, baseLoaded := fixture(0, 0)
				across, err := projectMPTDuplicateSampleBehaviour(0, tt.version, tt.strict)
				if err != nil {
					t.Fatal(err)
				}
				render := func(loaded *S3MLoadResult) []float32 {
					loaded.Assets.ITCompatibility.itDCTSampleAcrossInstruments = across
					w := NewTracker(loaded.Assets, referenceRate, 960)
					w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
					pcm := make([]float32, 11520)
					for i := 0; i < 5760; i += 960 {
						if w.Read(pcm[i*2:(i+960)*2], 960) != 960 {
							t.Fatal("short render")
						}
					}
					return pcm
				}
				actual, actualBase := render(loaded), render(baseLoaded)
				ref := renderMPTSequenceReference(t, module, "--repeat", "-1", "--end-time", "0.120")
				refBase := renderMPTSequenceReference(t, base, "--repeat", "-1", "--end-time", "0.120")
				if len(ref) < 11520 || len(refBase) < 11520 {
					t.Fatal("short OpenMPT render")
				}
				for _, ms := range []int{28, 68, 108} {
					baseRef, baseActual := channelRMS(refBase, ms, ms+5, 0), channelRMS(actualBase, ms, ms+5, 0)
					if baseRef < 1e-6 || baseActual < 1e-6 {
						t.Fatalf("%dms: silent baseline", ms)
					}
					want := channelRMS(ref, ms, ms+5, 0) / baseRef
					got := channelRMS(actual, ms, ms+5, 0) / baseActual
					if math.Abs(got-want) > .05 {
						t.Errorf("%dms: tracker ratio %.3f, OpenMPT %.3f", ms, got, want)
					}
					if ms == 68 && nextInstrument == 2 && (across && want > 1.2 || !across && want < 1.5) {
						t.Errorf("reference does not demonstrate cross-instrument distinction: across %v, ratio %.3f", across, want)
					}
				}
			})
		}
	}
}
