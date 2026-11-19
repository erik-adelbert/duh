package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTMWaveformSelection(t *testing.T) {
	all := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, family := range []byte{0x30, 0x40, 0x50} {
		for _, waveform := range []byte{0, 1, 2, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15} {
			for _, precise := range []bool{false, true} {
				if !precise && waveform&3 == 3 {
					continue
				}
				for _, old := range []bool{false, true} {
					t.Run(fmt.Sprintf("S%02X/precise=%t/old=%t", family|waveform, precise, old), func(t *testing.T) {
						flags := all
						if !precise {
							flags &^= 1 << mptITVibratoTremoloPanbrello
						}
						effect := byte(8)
						switch family {
						case 0x40:
							effect = 18
						case 0x50:
							effect = 25
						}
						rows := []byte{0x81, 15, 60, 1, 32, 19, family | waveform, 0,
							0x81, 8, effect, 0x8f, 0, 0x81, 15, 60, 1, 32, effect, 0, 0,
							0x81, 8, 19, 0, 0, 0x81, 8, effect, 0, 0}
						fixture := func(rows []byte) []byte {
							b := mptSharedEffectFixture(t, rows, flags)
							b[50] = 3
							if old {
								b[44] |= 0x10
							}
							return b
						}
						ref, actual := mptSharedEffectRenderForDuration(t, fixture(rows), 300)
						var refBase, actualBase []float32
						if family == 0x40 {
							refBase, actualBase = mptSharedEffectRenderForDuration(t, fixture([]byte{0x81, 7, 60, 1, 32, 0}), 300)
						}
						for tick := range 15 {
							ms := tick*20 + 19
							switch family {
							case 0x30:
								want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
								if math.IsNaN(got) || math.Abs(got/want-1) > .004 {
									t.Errorf("tick %d: OpenMPT %.2f Hz tracker %.2f Hz", tick, want, got)
								}
							case 0x40:
								assertMPTMVolumeRatio(t, ref, actual, refBase, actualBase, tick, .07)
							case 0x50:
								balance := func(pcm []float32) float64 {
									l, r := channelRMS(pcm, ms, ms+1, 0), channelRMS(pcm, ms, ms+1, 1)
									return (r - l) / (r + l)
								}
								want, got := balance(ref), balance(actual)
								if math.IsNaN(want) || math.IsNaN(got) || math.Abs(got-want) > .05 {
									t.Errorf("tick %d: OpenMPT balance %.3f tracker %.3f", tick, want, got)
								}
							}
						}
					})
				}
			}
		}
	}
}

func TestMPTMWaveformValidation(t *testing.T) {
	all := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for arg := byte(0x30); arg <= 0x5f; arg++ {
		modern := mptSharedEffectFixture(t, []byte{0x81, 8, 19, arg, 0}, all)
		if _, err := loadMPTM(modern); err != nil {
			t.Errorf("modern S%02X: %v", arg, err)
		}
		b := mptSharedEffectFixture(t, []byte{0x81, 8, 19, arg, 0}, all&^(1<<mptITVibratoTremoloPanbrello))
		_, err := loadMPTM(b)
		if arg&3 == 3 {
			if err == nil || !strings.Contains(err.Error(), "legacy random") {
				t.Errorf("S%02X: got %v, want legacy random rejection", arg, err)
			}
		} else if err != nil {
			t.Errorf("S%02X: %v", arg, err)
		}
		binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01250000)
		if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "pre-1.26") {
			t.Errorf("old S%02X: got %v", arg, err)
		}
	}
}

func TestMPTMWaveformOPLState(t *testing.T) {
	tests := []struct {
		family byte
		effect byte
		check  func(*Tracker, VoiceID) bool
	}{
		{0x30, 8, func(w *Tracker, v VoiceID) bool { return w.VoicePitches[v].VibratoOffset != 0 }},
		{0x40, 18, func(w *Tracker, v VoiceID) bool { return w.VoiceAmps[v].itTremoloOffset != 0 }},
		{0x50, 25, func(w *Tracker, v VoiceID) bool { return w.VoiceAmps[v].itPanbrelloOffset != 0 }},
	}
	all := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, tt := range tests {
		for _, precise := range []bool{false, true} {
			t.Run(fmt.Sprintf("S%02X/precise=%t", tt.family|1, precise), func(t *testing.T) {
				flags := all
				if !precise {
					flags &^= 1 << mptITVibratoTremoloPanbrello
				}
				rows := []byte{0x81, 15, 60, 2, 32, 19, tt.family | 1, 0,
					0x81, 8, tt.effect, 0x4f, 0}
				b := mptSharedEffectFixture(t, rows, flags)
				b[50] = 3
				loaded, err := loadMPTM(b)
				if err != nil {
					t.Fatal(err)
				}
				w := NewTracker(loaded.Assets, referenceRate, 256)
				w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
				buf := make([]float32, 512)
				audible := false
				for remaining := 4*960 + 1; remaining > 0; {
					n := min(remaining, 256)
					if got := w.Read(buf[:n*2], n); got != n {
						t.Fatalf("short render: %d", got)
					}
					for _, sample := range buf[:n*2] {
						audible = audible || sample != 0
					}
					remaining -= n
				}
				v := w.TrackCursors[0].ActiveVoice
				if v < 0 || !w.VoiceLives[v].Active || w.VoiceSamples[v].oplChannel == 0 || !audible || !tt.check(w, v) {
					t.Fatal("expected an active, audible and modulated OPL voice")
				}
			})
		}
	}
}

func TestOpenMPTMWaveformSavedVersions(t *testing.T) {
	all := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, family := range []byte{0x30, 0x40, 0x50} {
		for _, version := range []uint32{0x01260000, 0x01321200} {
			for _, precise := range []bool{false, true} {
				t.Run(fmt.Sprintf("S%02X/version=%08x/precise=%t", family|2, version, precise), func(t *testing.T) {
					flags := all
					if !precise {
						flags &^= 1 << mptITVibratoTremoloPanbrello
					}
					effect := byte(8)
					switch family {
					case 0x40:
						effect = 18
					case 0x50:
						effect = 25
					}
					fixture := func(rows []byte) []byte {
						b := mptSharedEffectFixture(t, rows, flags)
						b[50] = 2
						binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], version)
						return b
					}
					rows := []byte{0x81, 15, 60, 1, 32, 19, family | 2, 0, 0x81, 8, effect, 0x4f, 0}
					ref, actual := mptSharedEffectRender(t, fixture(rows))
					baseRef, baseActual := mptSharedEffectRender(t, fixture([]byte{0x81, 7, 60, 1, 32, 0}))
					for tick := 2; tick < 6; tick++ {
						switch family {
						case 0x30:
							want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
							if math.Abs(got/want-1) > .004 {
								t.Errorf("tick %d: OpenMPT %.2f Hz tracker %.2f Hz", tick, want, got)
							}
						case 0x40:
							assertMPTMVolumeRatio(t, ref, actual, baseRef, baseActual, tick, .07)
						case 0x50:
							ms := tick*20 + 19
							balance := func(pcm []float32) float64 {
								l, r := channelRMS(pcm, ms, ms+1, 0), channelRMS(pcm, ms, ms+1, 1)
								return (r - l) / (r + l)
							}
							want, got := balance(ref), balance(actual)
							if math.IsNaN(want) || math.IsNaN(got) || math.Abs(got-want) > .05 {
								t.Errorf("tick %d: OpenMPT balance %.3f tracker %.3f", tick, want, got)
							}
						}
					}
				})
			}
		}
	}
}
