package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestMPTInstrumentFadeouts(t *testing.T) {
	instruments := []ITInstrument{{Fadeout: 104}, {Fadeout: 32}}
	original := append([]ITInstrument(nil), instruments...)
	if err := applyMPTInstrumentFadeouts(instruments, nil); err != nil || !reflect.DeepEqual(instruments, original) {
		t.Fatalf("absent extension changed instruments: %+v, error %v", instruments, err)
	}
	for _, invalid := range [][]uint32{{1}, {3333, 32769}} {
		if err := applyMPTInstrumentFadeouts(instruments, invalid); err == nil || !reflect.DeepEqual(instruments, original) {
			t.Fatalf("invalid fadeouts %v changed instruments or were accepted: %+v, error %v", invalid, instruments, err)
		}
	}
	if err := applyMPTInstrumentFadeouts(instruments, []uint32{3333, 0}); err != nil {
		t.Fatal(err)
	}
	if got := instruments[0].fadeoutStep(); got != float32(3333)/32768 {
		t.Fatalf("extended fadeout step = %v", got)
	}
	if got := instruments[1].fadeoutStep(); got != 0 {
		t.Fatalf("explicit zero fadeout step = %v", got)
	}
	if got := original[0].fadeoutStep(); got != float32(104)/1024 {
		t.Fatalf("IT fadeout step = %v", got)
	}
}

func TestMPTInstrumentFadeoutRelease(t *testing.T) {
	for _, tt := range []struct {
		name     string
		header   uint16
		extended uint32
		wantFade bool
	}{
		{"extended nonzero overrides zero header", 0, 3333, true},
		{"extended zero overrides nonzero header", 104, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			instruments := []ITInstrument{{Fadeout: tt.header, VolumeEnvelope: ITVolumeEnvelope{Enabled: true, Loop: true}}}
			if err := applyMPTInstrumentFadeouts(instruments, []uint32{tt.extended}); err != nil {
				t.Fatal(err)
			}
			w := NewTracker(SongAssets{ITInstruments: instruments}, 48000, 1)
			w.VoiceLives[0] = VoiceLifecycle{Active: true, ITInstrument: 1, FadeOutVol: 1}
			releaseITVoice(w, 0)
			if w.VoiceLives[0].ITFade != tt.wantFade {
				t.Fatalf("release fade = %v, want %v", w.VoiceLives[0].ITFade, tt.wantFade)
			}
		})
	}
}

func TestMPTInstrumentFadeoutTicks(t *testing.T) {
	for _, tt := range []struct {
		fadeout uint32
		ticks   int
	}{
		{0, 40},
		{1, 25},
		{3333, 10},
		{32768, 1},
	} {
		t.Run(fmt.Sprint(tt.fadeout), func(t *testing.T) {
			instruments := []ITInstrument{{Fadeout: 104}}
			if err := applyMPTInstrumentFadeouts(instruments, []uint32{tt.fadeout}); err != nil {
				t.Fatal(err)
			}
			w := NewTracker(SongAssets{ITInstruments: instruments}, 48000, 1)
			w.VoiceLives[0] = VoiceLifecycle{Active: true, ITInstrument: 1, ITFade: true, FadeOutVol: 1}
			for tick := 1; tick <= tt.ticks; tick++ {
				processITInstrument(w, 0)
				if got := w.VoiceAmps[0].itEnvelopeRamping; got != (tt.fadeout != 0) {
					t.Fatalf("tick %d: fadeout ramping = %v", tick, got)
				}
				remaining := 32768 - tick*int(tt.fadeout)
				if remaining <= 0 {
					if w.VoiceLives[0].Active {
						t.Fatalf("tick %d: voice is still active", tick)
					}
					break
				}
				if !w.VoiceLives[0].Active || w.VoiceLives[0].FadeOutVol != float32(remaining)/32768 {
					t.Fatalf("tick %d: active=%v volume=%v, want %d/32768", tick, w.VoiceLives[0].Active, w.VoiceLives[0].FadeOutVol, remaining)
				}
			}
		})
	}
}

func TestOpenMPTInstrumentFadeout(t *testing.T) {
	for _, fadeout := range []uint32{0, 3328, 3333, 8192} {
		t.Run(fmt.Sprint(fadeout), func(t *testing.T) {
			makeFixture := func(value uint32) ([]byte, *S3MLoadResult) {
				b := testITInstrument()
				instrument := len(b) - 554
				b[instrument+304] = 0
				binary.LittleEndian.PutUint16(b[instrument+20:], 104)
				b[50] = 1
				packed := append([]byte{0x81, 3, 60, 1, 0, 0x81, 1, 255, 0}, make([]byte, 62)...)
				binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
				copy(b[344:], packed)
				loaded, err := loadIT(b)
				if err != nil {
					t.Fatal(err)
				}
				binary.LittleEndian.PutUint16(b[40:], 0x891)
				binary.LittleEndian.PutUint32(b[202:], uint32(len(b)))
				b = append(b, b[336:344+len(packed)]...)
				b = append(b, []byte("XTPM")...)
				b = append(b, mptExtensionField("..OF", 4, binary.LittleEndian.AppendUint32(nil, value))...)
				b = append(b, []byte("STPM")...)
				tail := len(b)
				b = append(b, mptTestSequences([]mptTestEntry{{"l", []byte{1, 0}}, {"a", []byte{0, 0}}})...)
				b = binary.LittleEndian.AppendUint32(b, uint32(tail))
				bounds, err := locateMPTExtensions(b)
				if err != nil {
					t.Fatal(err)
				}
				ext, err := parseMPTExtensions(b[bounds.propertiesStart:bounds.serializedStart], 1)
				if err != nil {
					t.Fatal(err)
				}
				if err := applyMPTInstrumentFadeouts(loaded.Assets.ITInstruments, ext.instrumentFadeout); err != nil {
					t.Fatal(err)
				}
				return b, loaded
			}
			module, loaded := makeFixture(fadeout)
			base, baseLoaded := makeFixture(0)
			ref := renderMPTSequenceReference(t, module, "--repeat", "-1", "--end-time", "0.200")
			refBase := renderMPTSequenceReference(t, base, "--repeat", "-1", "--end-time", "0.200")
			if len(ref) < referenceRate*2/5 || len(refBase) < referenceRate*2/5 {
				t.Fatal("short OpenMPT render")
			}
			render := func(loaded *S3MLoadResult, chunk int) []float32 {
				w := NewTracker(loaded.Assets, referenceRate, 960)
				w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
				out := make([]float32, referenceRate*2/5)
				for i := 0; i < len(out)/2; {
					n := min(chunk, len(out)/2-i)
					if w.Read(out[2*i:2*(i+n)], n) != n {
						t.Fatalf("short tracker render at frame %d", i)
					}
					i += n
				}
				return out
			}
			actual, actualBase := render(loaded, 960), render(baseLoaded, 960)
			for _, chunk := range []int{1, 17, 127} {
				if got := render(loaded, chunk); !reflect.DeepEqual(got, actual) {
					t.Errorf("render differs with chunk size %d", chunk)
				}
			}
			for _, ms := range []int{21, 25, 30, 35, 45, 85, 145, 185} {
				want := channelRMS(ref, ms, ms+5, 0) / channelRMS(refBase, ms, ms+5, 0)
				got := channelRMS(actual, ms, ms+5, 0) / channelRMS(actualBase, ms, ms+5, 0)
				if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .035 {
					t.Errorf("%dms: OpenMPT ratio %.5f, tracker %.5f", ms, want, got)
				}
			}
		})
	}
}
