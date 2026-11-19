package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
)

func smoothMacroFixture(t *testing.T, rows []byte) []byte {
	t.Helper()
	b := mptS70Fixture(t, rows)
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	pcm := int(binary.LittleEndian.Uint32(b[sample+72:]))
	for i := range 4096 {
		value := int16(18000 * math.Sin(2*math.Pi*float64(i)*.29))
		binary.LittleEndian.PutUint16(b[pcm+i*2:], uint16(value))
	}
	return b
}

func TestOpenMPTSmoothFilterMacro(t *testing.T) {
	for _, speed := range []byte{1, 3, 6} {
		for _, test := range []struct {
			name   string
			arg    byte
			cutoff byte
			res    byte
		}{
			{"cutoff up", 127, 40, 0},
			{"cutoff down", 0, 100, 0},
			{"fixed resonance", 0x8c, 70, 32},
			{"empty fixed", 0xff, 70, 32},
		} {
			t.Run(fmt.Sprintf("%s/speed=%d", test.name, speed), func(t *testing.T) {
				b := smoothMacroFixture(t, []byte{0x81, 3, 96, 2, 0, 0x81, 8, 28, test.arg, 0, 0, 0})
				b[50] = speed
				inst := int(binary.LittleEndian.Uint32(b[198:]))
				b[inst+58], b[inst+59] = 128|test.cutoff, 128|test.res
				ref, actual := mptSharedEffectRenderForDuration(t, b, 500)
				refBase, actualBase := rmsWindow(ref, 8, 11), rmsWindow(actual, 8, 11)
				if refBase < .001 || actualBase < .001 {
					t.Fatal("silent baseline")
				}
				for tick := int(speed); tick < int(speed)*2; tick++ {
					ms := tick*20 + 15
					r, a := rmsWindow(ref, ms, ms+3)/refBase, rmsWindow(actual, ms, ms+3)/actualBase
					if math.IsNaN(r) || math.IsNaN(a) || math.Abs(r-a) > .06 {
						t.Errorf("tick %d: OpenMPT ratio %.3f, tracker %.3f", tick, r, a)
					}
					if speed > 1 && tick == int(speed)*2-1 && strings.HasPrefix(test.name, "cutoff") && math.Abs(r-1) < .1 {
						t.Errorf("cutoff slide did not measurably change the reference: %.3f", r)
					}
				}
				assertMPTPatternDelayChunking(t, b, 9000)
			})
		}
	}
}

func TestMPTSmoothMacroChannelIsolation(t *testing.T) {
	b := smoothMacroFixture(t, []byte{0x81, 11, 60, 2, 28, 0, 0x82, 11, 64, 2, 28, 127, 0})
	b[50] = 6
	b[bytesIndex(t, b, "...C")+6] = 2
	inst := int(binary.LittleEndian.Uint32(b[198:]))
	b[inst+58] = 128 | 90
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, referenceRate, 256)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 6, 125, -1
	if got := w.Read(make([]float32, 2), 1); got != 1 {
		t.Fatalf("short render: %d", got)
	}
	if left, right := w.TrackCursors[0].ITFilterCutoff, w.TrackCursors[1].ITFilterCutoff; left != 75 || right != 96 {
		t.Fatalf("channel cutoff values %d and %d, want 75 and 96", left, right)
	}
}

func TestMPTSmoothMacroTickValues(t *testing.T) {
	b := mptS70Fixture(t, []byte{0x81, 3, 60, 2, 0, 0x81, 8, 28, 127, 0})
	b[50] = 6
	inst := int(binary.LittleEndian.Uint32(b[198:]))
	b[inst+58] = 128 | 40
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, referenceRate, 960)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 6, 125, -1
	out := make([]float32, 960*2)
	for range 6 {
		if got := w.Read(out, 960); got != 960 {
			t.Fatalf("short initial render: %d", got)
		}
	}
	for tick, want := range []uint8{55, 69, 84, 98, 113, 127} {
		if got := w.Read(out, 1); got != 1 {
			t.Fatalf("tick %d: short render: %d", tick, got)
		}
		if got := w.TrackCursors[0].ITFilterCutoff; got != want {
			t.Fatalf("tick %d: cutoff %d, want %d", tick, got, want)
		}
		if got := w.Read(out, 959); got != 959 {
			t.Fatalf("tick %d: short render: %d", tick, got)
		}
	}
}

func TestOpenMPTSmoothMacroFilterSwing(t *testing.T) {
	for _, resonance := range []bool{false, true} {
		t.Run(fmt.Sprintf("resonance=%t", resonance), func(t *testing.T) {
			arg := byte(127)
			if resonance {
				arg = 0x8c
			}
			b := smoothMacroFixture(t, []byte{0x81, 11, 60, 2, 28, arg, 0, 0, 0})
			b[50] = 6
			inst := int(binary.LittleEndian.Uint32(b[198:]))
			b[inst+58], b[inst+59] = 128|80, 128|48
			loaded, err := loadMPTM(b)
			if err != nil {
				t.Fatal(err)
			}
			random := rand.New(rand.NewSource(1)).Intn(256) - 128
			refData := append([]byte(nil), b...)
			if resonance {
				loaded.Assets.ITInstruments[1].mptResonanceSwing = 64
				refData[inst+59] = 128 | mptRandomCutoff(48, 64, random)
			} else {
				loaded.Assets.ITInstruments[1].mptCutoffSwing = 64
				refData[inst+58] = 128 | mptRandomCutoff(80, 64, random)
			}
			ref := referenceMPTMForDuration(t, refData, 360)
			w := NewTracker(loaded.Assets, referenceRate, 256)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 6, 125, -1
			actual := make([]float32, len(ref))
			for frame := 0; frame < len(actual)/2; frame += 256 {
				n := min(256, len(actual)/2-frame)
				if got := w.Read(actual[frame*2:(frame+n)*2], n); got != n {
					t.Fatalf("short render: %d", got)
				}
			}
			for tick := range 6 {
				ms := tick*20 + 15
				want, got := rmsWindow(ref, ms, ms+3), rmsWindow(actual, ms, ms+3)
				if want < .001 || got < .001 || math.Abs(got/want-1) > .08 {
					t.Errorf("tick %d: OpenMPT %g, tracker %g", tick, want, got)
				}
			}
		})
	}
}

func TestMPTSmoothMacroValidation(t *testing.T) {
	b := mptS70Fixture(t, []byte{0x81, 8, 28, 0x20, 0})
	b = embeddedMacroFixture(t, b, -1, map[int]string{0: "Bc07z"}, nil)
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "unsupported embedded MIDI macros") {
		t.Fatalf("unsupported macro: %v", err)
	}
	delayed := mptS70Fixture(t, []byte{0x81, 8, 19, 0xe1, 0x82, 8, 28, 64, 0})
	if _, err := loadMPTM(delayed); err == nil || !strings.Contains(err.Error(), "pattern delay") {
		t.Fatalf("pattern delay: %v", err)
	}
	legacy := mptS70Fixture(t, []byte{0x81, 8, 28, 64, 0})
	binary.LittleEndian.PutUint32(legacy[bytesIndex(t, legacy, "VWSL")+6:], 0x01250000)
	if _, err := loadMPTM(legacy); err == nil || !strings.Contains(err.Error(), "legacy playback profile") {
		t.Fatalf("legacy profile: %v", err)
	}
	old := mptS70Fixture(t, []byte{0x81, 8, 28, 64, 0})
	old[bytesIndex(t, old, ".FSM")+6] |= 2
	bounds, err := locateMPTExtensions(old)
	if err != nil {
		t.Fatal(err)
	}
	insert := bounds.propertiesStart + 4
	field := mptExtensionField("..SC", 1, []byte{0, 40})
	oldWithSwing := append([]byte(nil), old[:insert]...)
	oldWithSwing = append(oldWithSwing, field...)
	oldWithSwing = append(oldWithSwing, old[insert:len(old)-4]...)
	oldWithSwing = binary.LittleEndian.AppendUint32(oldWithSwing, uint32(bounds.serializedStart+len(field)))
	if _, err := loadMPTM(oldWithSwing); err == nil || !strings.Contains(err.Error(), "old filter swing") {
		t.Fatalf("old filter swing: %v", err)
	}
}

func TestOpenMPTSmoothMacroTransitions(t *testing.T) {
	b := smoothMacroFixture(t, []byte{
		0x81, 8, 19, 0xf1, 0,
		0x81, 11, 60, 2, 28, 100, 0,
		0x81, 8, 28, 20, 0,
		0x81, 9, 255, 28, 80, 0,
		0x81, 11, 64, 2, 28, 0x80, 0,
		0, 0,
	})
	b[50] = 3
	inst := int(binary.LittleEndian.Uint32(b[198:]))
	b[inst+58], b[inst+59] = 128|70, 128|30
	b = embeddedMacroFixture(t, b, -1, map[int]string{0: "F0F000z", 1: "F0F001z"}, map[int]string{0: "F0F00030"})
	ref, actual := mptSharedEffectRenderForDuration(t, b, 420)
	for ms := 75; ms < 290; ms += 20 {
		r, a := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
		if r < .0001 || a < .0001 || math.Abs(a/r-1) > .12 {
			t.Errorf("%dms: OpenMPT %g, tracker %g", ms, r, a)
		}
	}
	assertMPTPatternDelayChunking(t, b, referenceRate/3)
}

func TestOpenMPTSmoothMacroFilterEnvelope(t *testing.T) {
	b := smoothMacroFixture(t, []byte{0x81, 11, 60, 2, 28, 110, 0, 0, 0})
	b[50] = 6
	inst := int(binary.LittleEndian.Uint32(b[198:]))
	b[inst+58], b[inst+59] = 128|60, 128|32
	setTestEnvelope(b[inst:], 468, 0x81, []int8{-16, 16, -8}, []uint16{0, 3, 5})
	ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
	for ms := 15; ms < 115; ms += 20 {
		r, a := rmsWindow(ref, ms, ms+4), rmsWindow(actual, ms, ms+4)
		if r < .0001 || a < .0001 || math.Abs(a/r-1) > .12 {
			t.Errorf("%dms: OpenMPT %g, tracker %g", ms, r, a)
		}
	}
}

func TestOpenMPTSmoothMacroBeforeNote(t *testing.T) {
	b := smoothMacroFixture(t, []byte{0x81, 8, 28, 20, 0, 0x81, 3, 60, 2, 0, 0})
	b[50] = 3
	inst := int(binary.LittleEndian.Uint32(b[198:]))
	b[inst+58], b[inst+59] = 0, 0
	ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
	r, a := rmsWindow(ref, 95, 105), rmsWindow(actual, 95, 105)
	if r < .0001 || a < .0001 || math.Abs(a/r-1) > .12 {
		t.Fatalf("OpenMPT %g, tracker %g", r, a)
	}
}

func TestOpenMPTSmoothMacroOPL(t *testing.T) {
	b := mptS70Fixture(t, []byte{0x81, 11, 60, 1, 28, 110, 0, 0, 0})
	b[50] = 6
	inst := int(binary.LittleEndian.Uint32(b[194:]))
	b[inst+304] = 0
	b[inst+58], b[inst+59] = 128|60, 128
	ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
	for ms := 15; ms < 115; ms += 20 {
		r, a := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
		if r < .0001 || a < .0001 || math.Abs(a/r-1) > .2 {
			t.Errorf("%dms: OpenMPT %g, tracker %g", ms, r, a)
		}
	}
}
