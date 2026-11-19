package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestMPTMTempoSlideLoader(t *testing.T) {
	for arg := 0; arg <= 255; arg++ {
		p, err := unpackITPatternMode([]byte{0x81, 8, 20, byte(arg), 0, 0x81, 0x80, 0}, 2, true)
		if err != nil {
			t.Fatalf("T%02X: %v", arg, err)
		}
		for _, row := range p.Rows {
			if row[0].EffectCmd != 0x20 || row[0].EffectArg != byte(arg) {
				t.Fatalf("T%02X: %+v", arg, row[0])
			}
		}
	}
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	flags &^= 1 << mptMODVBlankTiming
	for _, arg := range []byte{0, 1, 0x1f, 0x20, 0xff} {
		b := mptSharedEffectFixture(t, []byte{0x81, 8, 20, arg, 0}, flags)
		binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01250000)
		_, err := loadMPTM(b)
		if arg < 0x20 {
			if err == nil || !strings.Contains(err.Error(), "tempo command") {
				t.Fatalf("pre-1.26 T%02X: %v", arg, err)
			}
		} else if err != nil {
			t.Fatalf("pre-1.26 T%02X: %v", arg, err)
		}
	}
	b := mptSharedEffectFixture(t, []byte{0x81, 8, 20, 125, 0}, flags|(1<<mptMODVBlankTiming))
	binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01250000)
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "tempo command") {
		t.Fatalf("pre-1.26 VBlank T7D: %v", err)
	}
}

func TestOpenMPTMTempoSlides(t *testing.T) {
	tests := []struct {
		name    string
		speed   byte
		initial uint32
		clamp   bool
		rows    [][]byte
		tempos  []float64
	}{
		{"down-memory", 3, 125, true, [][]byte{{0x82, 8, 20, 3}, {0x82, 8, 20, 0}, nil}, []float64{125, 122, 119, 119, 116, 113, 113, 113, 113}},
		{"up-memory", 3, 125, true, [][]byte{{0x82, 8, 20, 0x14}, {0x82, 8, 20, 0}, nil}, []float64{125, 129, 133, 133, 137, 141, 141, 141, 141}},
		{"set-memory", 3, 125, true, [][]byte{{0x82, 8, 20, 0x14}, {0x82, 8, 20, 100}, {0x82, 8, 20, 0}}, []float64{125, 129, 133, 100, 100, 100, 100, 100, 100}},
		{"channel-memory", 3, 125, true, [][]byte{{0x82, 8, 20, 0x14}, {0x83, 8, 20, 0}, {0x82, 8, 20, 0}}, []float64{125, 129, 133, 133, 133, 133, 133, 137, 141}},
		{"simultaneous", 3, 125, true, [][]byte{{0x82, 8, 20, 3, 0x83, 8, 20, 0x14}, {0x82, 8, 20, 0, 0x83, 8, 20, 0}, nil}, []float64{125, 126, 127, 127, 128, 129, 129, 129, 129}},
		{"lower-limit", 3, 35, true, [][]byte{{0x82, 8, 20, 0x0f}, {0x82, 8, 20, 0}, nil}, []float64{35, 32, 32, 32, 32, 32, 32, 32, 32}},
		{"upper-clamped", 3, 250, true, [][]byte{{0x82, 8, 20, 0x1f}, {0x82, 8, 20, 0}, nil}, []float64{250, 255, 255, 255, 255, 255, 255, 255, 255}},
		{"upper-unclamped", 3, 250, false, [][]byte{{0x82, 8, 20, 0x1f}, {0x82, 8, 20, 0}, nil}, []float64{250, 265, 280, 280, 295, 310, 310, 310, 310}},
		{"maximum-unclamped", 3, 990, false, [][]byte{{0x82, 8, 20, 0x1f}, {0x82, 8, 20, 0}, nil}, []float64{990, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000}},
		{"speed-one", 1, 125, true, [][]byte{{0x82, 8, 20, 0x1f}, {0x82, 8, 20, 0}, nil}, []float64{125, 125, 125}},
	}
	for _, tt := range tests {
		for _, modern := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/modern=%t", tt.name, modern), func(t *testing.T) {
				flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
				flags &^= 1 << mptMODVBlankTiming
				if !tt.clamp {
					flags &^= 1 << mptTempoClamp
				}
				var rows []byte
				for i, row := range tt.rows {
					if i == 0 {
						rows = append(rows, 0x81, 3, 60, 1)
					}
					rows = append(rows, row...)
					rows = append(rows, 0)
				}
				rows = append(rows, 0x81, 1, 254, 0)
				b := mptSharedEffectFixture(t, rows, flags)
				b[50], b[51] = tt.speed, byte(min(tt.initial, 255))
				b[bytesIndex(t, b, "...C")+6] = 3
				if modern {
					b[46] |= 4
					b[bytesIndex(t, b, "..MT")+6] = 2
				}
				if tt.initial > 255 {
					bounds, err := locateMPTExtensions(b)
					if err != nil {
						t.Fatal(err)
					}
					field := mptExtensionField("..TD", 4, binary.LittleEndian.AppendUint32(nil, tt.initial))
					tail := bounds.serializedStart
					b = append(append(append([]byte(nil), b[:tail]...), field...), b[tail:]...)
					binary.LittleEndian.PutUint32(b[len(b)-4:], uint32(tail+len(field)))
				}
				exact := 0.0
				for _, tempo := range tt.tempos {
					if modern {
						exact += 48000.0 * 60 / (tempo * float64(tt.speed) * 8)
					} else {
						exact += math.Floor(48000 * 2.5 / tempo)
					}
				}
				expected := int(exact)
				ref, actual := mptSharedEffectRenderForDuration(t, b, (expected+4799)/48+50)
				lastAudible := func(pcm []float32) int {
					for i := len(pcm)/2 - 1; i >= 0; i-- {
						if math.Abs(float64(pcm[2*i])) > 1e-6 || math.Abs(float64(pcm[2*i+1])) > 1e-6 {
							return i
						}
					}
					return -1
				}
				want, got := lastAudible(ref), lastAudible(actual)
				if absInt(want-expected) > 5 || absInt(got-want) > 5 {
					t.Fatalf("cut: OpenMPT %d, tracker %d, expected about %d", want, got, expected)
				}
			})
		}
	}
}

func TestMPTMModernTempoSlideRemainder(t *testing.T) {
	rows := [][]ModCommand{
		{{EffectCmd: 0x20, EffectArg: 0x11}},
		{{EffectCmd: 0x20, EffectArg: 0}},
		{{EffectCmd: 0x20, EffectArg: 137}},
		{{EffectCmd: 0x20, EffectArg: 0}},
	}
	w := NewTracker(SongAssets{IT: true, NumTracks: 1, OrderList: []int{0}, Patterns: []PatternAsset{{Rows: rows}}, mptRowsPerBeat: 8}, 48000, 1)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 7, 115, -1
	total, exact := 0, 0.0
	for tick := 0; tick < len(rows)*7; tick++ {
		if !TickSchedulerSystem(w) {
			t.Fatalf("tick %d did not start", tick)
		}
		PatternFetchSystem(w)
		GlobalFlowSystem(w)
		exact += 48000.0 * 60 / (w.Clock.Tempo * 7 * 8)
		total += w.Clock.SamplesPerTick
		if total != int(exact) {
			t.Fatalf("tick %d: total %d, want %d", tick, total, int(exact))
		}
		w.Clock.SamplesRemaining = 0
	}
}

func TestOpenMPTMTempoAsSpeed(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	rows := []byte{
		0x81, 3, 60, 1, 0x82, 8, 1, 4, 0x83, 8, 20, 3, 0,
		0x82, 8, 20, 0, 0x83, 8, 1, 2, 0,
		0x82, 8, 1, 3, 0x83, 8, 20, 4, 0,
		0x81, 1, 254, 0,
	}
	for _, modern := range []bool{false, true} {
		t.Run(fmt.Sprintf("modern=%t", modern), func(t *testing.T) {
			b := mptSharedEffectFixture(t, rows, flags)
			b[50], b[bytesIndex(t, b, "...C")+6] = 2, 3
			if modern {
				b[bytesIndex(t, b, "..MT")+6] = 2
			}
			ref, actual := mptSharedEffectRenderForDuration(t, b, 250)
			const expected = 8640
			for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
				last := -1
				for i := len(pcm)/2 - 1; i >= 0; i-- {
					if math.Abs(float64(pcm[2*i])) > 1e-6 || math.Abs(float64(pcm[2*i+1])) > 1e-6 {
						last = i
						break
					}
				}
				if absInt(last-expected) > 5 {
					t.Errorf("%s cut at %d, want about %d", name, last, expected)
				}
			}
		})
	}
}

func TestOpenMPTMModernTempoWithoutHighlights(t *testing.T) {
	flags := mptSavedPlayBehaviours((1<<mptKnownPlayBehaviourCount)-1) &^ (1 << mptMODVBlankTiming)
	b := mptSharedEffectFixture(t, []byte{0x81, 11, 60, 1, 20, 0x14, 0, 0x81, 1, 254, 0}, flags)
	b[50], b[46] = 3, b[46]&^4
	b[bytesIndex(t, b, "..MT")+6] = 2
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Assets.mptRowsPerBeat != 4 {
		t.Fatalf("rows per beat %d, want 4", loaded.Assets.mptRowsPerBeat)
	}
	ref, actual := mptSharedEffectRenderForDuration(t, b, 200)
	exact := 48000.0*60/(125*3*4) + 48000.0*60/(129*3*4) + 48000.0*60/(133*3*4)
	expected := int(exact)
	for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
		last := -1
		for i := len(pcm)/2 - 1; i >= 0; i-- {
			if math.Abs(float64(pcm[2*i])) > 1e-6 || math.Abs(float64(pcm[2*i+1])) > 1e-6 {
				last = i
				break
			}
		}
		if absInt(last-expected) > 5 {
			t.Errorf("%s cut at %d, want about %d", name, last, expected)
		}
	}
}

func TestOpenMPTMTempoSlideVersions(t *testing.T) {
	rows := []byte{0x81, 11, 60, 1, 20, 0x14, 0, 0x81, 8, 20, 0, 0, 0, 0x81, 1, 254, 0}
	for _, version := range []uint32{0x01260000, 0x01280100, 0x01321200} {
		for _, old := range []bool{false, true} {
			for _, flags := range []mptSavedPlayBehaviours{0, mptSavedPlayBehaviours((1<<mptKnownPlayBehaviourCount)-1) &^ (1 << mptMODVBlankTiming)} {
				t.Run(fmt.Sprintf("%08x/old=%t/flags=%x", version, old, flags), func(t *testing.T) {
					b := mptSharedEffectFixture(t, rows, flags)
					b[50] = 3
					if old {
						b[44] |= 0x10
					}
					binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], version)
					ref, actual := mptSharedEffectRenderForDuration(t, b, 220)
					for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
						last := -1
						for i := len(pcm)/2 - 1; i >= 0; i-- {
							if math.Abs(float64(pcm[2*i])) > 1e-6 || math.Abs(float64(pcm[2*i+1])) > 1e-6 {
								last = i
								break
							}
						}
						if absInt(last-7973) > 5 {
							t.Errorf("%s cut at %d, want about 7973", name, last)
						}
					}
				})
			}
		}
	}
}

func TestMPTMTempoSlideOPL(t *testing.T) {
	flags := mptSavedPlayBehaviours((1<<mptKnownPlayBehaviourCount)-1) &^ (1 << mptMODVBlankTiming)
	b := mptSharedEffectFixture(t, []byte{0x81, 11, 60, 2, 20, 0x13, 0}, flags)
	b[50] = 3
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, referenceRate, 128)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	out := make([]float32, 2*961)
	for pos := 0; pos < 961; {
		n := min(128, 961-pos)
		if got := w.Read(out[2*pos:2*(pos+n)], n); got != n {
			t.Fatalf("short OPL render at %d: %d", pos, got)
		}
		pos += n
	}
	voice := w.TrackCursors[0].ActiveVoice
	if voice < 0 || w.VoiceSamples[voice].oplChannel == 0 || w.Clock.Tick != 1 || w.Clock.Tempo != 128 {
		t.Fatalf("voice=%d tick=%d tempo=%g", voice, w.Clock.Tick, w.Clock.Tempo)
	}
}
