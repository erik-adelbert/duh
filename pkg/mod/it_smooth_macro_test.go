package mod

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTITSmoothFilterMacro(t *testing.T) {
	for _, tt := range []struct {
		name      string
		arg       byte
		frequency float64
		resonance bool
		values    [6]uint8
	}{
		{"cutoff", 20, 2000, false, [6]uint8{62, 54, 46, 37, 29, 20}},
		{"resonance to zero", 0x80, 1100, true, [6]uint8{80, 64, 48, 32, 16, 0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := testITInstrument()
			h := b[len(b)-554:]
			h[58], h[59], h[304] = 128|70, 128|96, 0
			binary.LittleEndian.PutUint32(b[256+56:], 3840)
			for i := range 4096 {
				v := int16(16000 * math.Sin(2*math.Pi*float64(i)*tt.frequency/referenceRate))
				binary.LittleEndian.PutUint16(b[512+i*2:], uint16(v))
			}
			rows := append([]byte{0x81, 3, 60, 1, 0, 0x81, 8, 28, tt.arg, 0}, make([]byte, 62)...)
			binary.LittleEndian.PutUint16(b[336:], uint16(len(rows)))
			copy(b[344:], rows)

			loaded, err := loadIT(b)
			if err != nil {
				t.Fatal(err)
			}
			w := NewTracker(loaded.Assets, referenceRate, 960)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
			out := make([]float32, 960*2)
			for tick := range 12 {
				if got := w.Read(out, 960); got != 960 {
					t.Fatalf("tick %d: rendered %d frames", tick, got)
				}
				if tick >= 6 {
					cursor := w.TrackCursors[0]
					got := cursor.ITFilterCutoff
					if tt.resonance {
						got = cursor.ITFilterResonance
					}
					if want := tt.values[tick-6]; got != want {
						t.Errorf("tick %d: filter parameter %d, want %d", tick, got, want)
					}
				}
			}

			ref, actual := referencePCMWithChunks(t, b, "1", 240, 97)
			r0, a0 := rmsWindow(ref, 95, 98), rmsWindow(actual, 95, 98)
			if r0 < .001 || a0 < .001 {
				t.Fatal("silent baseline")
			}
			for tick := 6; tick < 12; tick++ {
				ms := tick*20 + 15
				want, got := rmsWindow(ref, ms, ms+3)/r0, rmsWindow(actual, ms, ms+3)/a0
				if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .06 {
					t.Errorf("tick %d: OpenMPT RMS ratio %.3f, tracker %.3f", tick, want, got)
				}
			}
			if end := rmsWindow(ref, 235, 238) / r0; math.Abs(end-1) < .1 {
				t.Errorf("reference ramp did not measurably change the audio: RMS ratio %.3f", end)
			}
		})
	}
}

func TestITSmoothMacroValidation(t *testing.T) {
	rows := []byte{0x81, 8, 28, 0x80, 0}
	unsupported := embeddedMacroFixture(t, testITPitchFixture(rows, 6, true, false), -1, nil, map[int]string{0: "Bc0710"})
	delayed := testITPitchFixture([]byte{0x81, 8, 19, 0xe2, 0x82, 8, 28, 0x80, 0}, 6, true, false)
	for _, tt := range []struct {
		b    []byte
		want string
	}{
		{unsupported, `unsupported embedded MIDI macros for \80`},
		{testITPitchFixture(rows, 6, true, true), "legacy ModPlug"},
		{delayed, "pattern delay"},
	} {
		if _, err := loadIT(tt.b); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("got %v, want %q", err, tt.want)
		}
	}
}
