package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTCompatibleGlobalVolume(t *testing.T) {
	for _, source := range []string{"pcm", "opl", "mixed"} {
		for _, test := range []struct {
			name    string
			initial byte
			changes []byte
			levels  []byte
		}{
			{"initial maximum", 128, nil, []byte{128, 128, 128, 128, 128, 128}},
			{"initial half", 64, nil, []byte{64, 64, 64, 64, 64, 64}},
			{"initial low", 1, nil, []byte{1, 1, 1, 1, 1, 1}},
			{"initial zero", 0, nil, []byte{0, 0, 0, 0, 0, 0}},
			{"changes", 128, []byte{64, 0, 1, 127}, []byte{128, 64, 0, 1, 127, 127}},
			{"restore from silence", 0, []byte{0, 128, 64, 128}, []byte{0, 0, 128, 64, 128, 128}},
		} {
			t.Run(fmt.Sprintf("%s/%s", source, test.name), func(t *testing.T) {
				fixture := func(initial byte, changes []byte) ([]byte, SongAssets) {
					return mptMixGlobalFixture(t, source == "opl", source == "mixed", 64, 64, testOPLPatch, initial, changes)
				}
				module, assets := fixture(test.initial, test.changes)
				base, baseAssets := fixture(128, nil)
				reference := renderMPTSequenceReference(t, module, "--repeat", "-1", "--end-time", "0.360")
				referenceBase := renderMPTSequenceReference(t, base, "--repeat", "-1", "--end-time", "0.360")
				frames := referenceRate * 360 / 1000
				actual, actualBase := renderMPTMixFixture(t, assets, frames), renderMPTMixFixture(t, baseAssets, frames)
				if len(reference) < len(actual) || len(referenceBase) < len(actual) {
					t.Fatal("short reference render")
				}
				for i, level := range test.levels {
					start := i*60 + 35
					wantBase, gotBase := rmsWindow(referenceBase, start, start+10), rmsWindow(actualBase, start, start+10)
					if wantBase < .001 || gotBase < .001 {
						t.Fatalf("%dms: silent baseline", start)
					}
					want := rmsWindow(reference, start, start+10) / wantBase
					got := rmsWindow(actual, start, start+10) / gotBase
					expected := float64(level) / 128
					if math.Abs(want-expected) > .01 || math.Abs(got-want) > .015 {
						t.Errorf("%dms: OpenMPT %.5f, tracker %.5f, expected %.5f", start, want, got, expected)
					}
				}
				for i, level := range test.changes {
					start := (i + 1) * 60
					wantBase, gotBase := rmsWindow(referenceBase, start, start+1), rmsWindow(actualBase, start, start+1)
					if wantBase < .001 || gotBase < .001 {
						t.Fatalf("%dms: silent transition baseline", start)
					}
					want := rmsWindow(reference, start, start+1) / wantBase
					got := rmsWindow(actual, start, start+1) / gotBase
					if expected := float64(level) / 128; math.Abs(want-expected) > .02 || math.Abs(got-want) > .02 {
						t.Errorf("%dms transition: OpenMPT %.5f, tracker %.5f, expected %.5f", start, want, got, expected)
					}
				}
			})
		}
	}
}

func TestMPTCompatibleGlobalVolumeState(t *testing.T) {
	profile, err := projectMPTMixProfile(4, 64, 64)
	if err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []bool{false, true} {
		assets := SongAssets{
			IT: true, NumTracks: 1, ITGlobalVolume: 128, ITMixGain: 1,
			ITCompatibility: ITCompatibility{LegacyModPlug: legacy, mptMixProfile: &profile},
			ITChannelVolume: [MaxTracks]uint8{64}, Samples: []SampleAsset{{ITGlobalVolume: 64}},
		}
		w, other := NewTracker(assets, 48000, 4), NewTracker(assets, 48000, 4)
		w.VoiceLives[0] = VoiceLifecycle{Active: true, ParentTrack: 0, FadeOutVol: 1}
		w.VoiceAmps[0].Volume = 1
		for _, volume := range []uint8{128, 64, 1, 0} {
			w.ITGlobalVolume = volume
			if got := voiceVolume(w, 0, 0); got != 1 {
				t.Errorf("legacy=%v volume=%d: voice gain %v, want 1", legacy, volume, got)
			}
			for i := range 4 {
				w.VoiceSamples[0].ScratchBuf[i] = 1
			}
			w.VoiceAmps[0].CurrentLeft, w.VoiceAmps[0].CurrentRight = .25, .25
			w.VoiceAmps[0].TargetLeft, w.VoiceAmps[0].TargetRight = .25, .25
			out := make([]float32, 8)
			MixerAccumulationSystem(w, out, 4)
			for _, got := range out {
				if want := float32(volume) / 128 * .25; got != want {
					t.Errorf("legacy=%v volume=%d: output %v, want %v", legacy, volume, got, want)
				}
			}
		}
		if other.ITGlobalVolume != 128 || w.Assets.ITGlobalVolume != 128 {
			t.Fatal("global volume changed shared assets or another world")
		}
	}
}

func TestMPTCompatibleGlobalVolumeChunking(t *testing.T) {
	_, assets := mptMixGlobalFixture(t, false, true, 64, 64, testOPLPatch, 128, []byte{64, 0, 1, 127})
	render := func(chunk int) []float32 {
		w := NewTracker(assets, referenceRate, 960)
		w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 3, 125, -1
		out := make([]float32, 14400*2)
		for frame := 0; frame < len(out)/2; {
			n := min(chunk, len(out)/2-frame)
			if got := w.Read(out[frame*2:(frame+n)*2], n); got != n {
				t.Fatalf("short render at frame %d: %d", frame, got)
			}
			frame += n
		}
		return out
	}
	whole, split := render(960), render(137)
	for i := 960 * 2; i < len(whole); i++ {
		if math.Abs(float64(whole[i]-split[i])) > 1e-5 {
			t.Fatalf("sample %d: chunked output %v, want %v", i, split[i], whole[i])
		}
	}
}
