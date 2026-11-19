package mod

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestITCutoffMacroPlayback(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
		base []byte
	}{
		{"with note", []byte{0x81, 11, 60, 1, 26, 32, 0}, []byte{0x81, 11, 60, 1, 26, 127, 0}},
		{"before note", []byte{0x81, 8, 26, 32, 0, 0x81, 3, 60, 1, 0}, []byte{0, 0x81, 3, 60, 1, 0}},
		{"change playing note", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 26, 32, 0}, []byte{0x81, 3, 60, 1, 0, 0}},
		{"empty SF1", []byte{0x81, 8, 19, 0xf1, 0, 0x81, 11, 60, 1, 26, 32, 0}, []byte{0, 0x81, 3, 60, 1, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			render := func(rows []byte) ([]byte, []float32) {
				b := testITPitchFixture(rows, 2, true, false)
				loaded, err := loadIT(b)
				if err != nil {
					t.Fatal(err)
				}
				w := NewTracker(loaded.Assets, referenceRate, 5760)
				w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 2, 125, -1
				pcm := make([]float32, 5760*2)
				if n := w.Read(pcm, 5760); n != 5760 {
					t.Fatalf("short render: %d", n)
				}
				return b, pcm
			}
			b, actual := render(tt.rows)
			base, actualBase := render(tt.base)
			ref, refBase := referenceTransition(t, b), referenceTransition(t, base)
			wantBase, gotBase := rmsWindow(refBase, 85, 110), rmsWindow(actualBase, 85, 110)
			if wantBase < .001 || gotBase < .001 {
				t.Fatal("silent comparison baseline")
			}
			want := rmsWindow(ref, 85, 110) / wantBase
			got := rmsWindow(actual, 85, 110) / gotBase
			if math.Abs(got-want) > .06 {
				t.Fatalf("relative RMS: OpenMPT %g, tracker %g", want, got)
			}
		})
	}
}

func TestITMacroValidation(t *testing.T) {
	for _, tt := range []struct {
		name string
		data []byte
		want string
	}{
		{"resonance", testITPitchFixture([]byte{0x81, 8, 26, 0x80, 0}, 2, true, false), ""},
		{"legacy cutoff", testITPitchFixture([]byte{0x81, 8, 26, 32, 0}, 2, true, true), ""},
		{"legacy resonance", testITPitchFixture([]byte{0x81, 8, 26, 0x84, 0}, 2, true, true), "unsupported filter macro"},
		{"embedded unsupported", embeddedMacroFixture(t, testITPitchFixture([]byte{0x81, 8, 26, 32, 0}, 2, true, false), -1, map[int]string{0: "Bc07z"}, nil), "unsupported embedded"},
		{"embedded cutoff", embeddedMacroFixture(t, testITPitchFixture([]byte{0x81, 8, 26, 32, 0}, 2, true, false), -1, map[int]string{0: "F0F000z"}, nil), ""},
		{"empty fixed", testITPitchFixture([]byte{0x81, 8, 26, 0xff, 0}, 2, true, false), ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadIT(tt.data)
			if tt.want == "" && err != nil || tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}

func TestOpenMPTITLegacyCutoffMacro(t *testing.T) {
	for _, extended := range []bool{false, true} {
		fixture := func(cutoff byte, macro bool) []byte {
			rows := []byte{0x81, 3, 60, 1, 0, 0}
			if macro {
				rows = []byte{0x81, 3, 60, 1, 0, 0x81, 8, 26, cutoff, 0}
			}
			b := testITPitchFixture(rows, 2, true, true)
			if extended {
				b[45] |= 0x10
			}
			return b
		}
		baseRef, baseActual := referencePCM(t, fixture(0, false))
		for _, cutoff := range []byte{61, 127} {
			ref, actual := referencePCM(t, fixture(cutoff, true))
			want := rmsWindow(ref, 60, 90) / rmsWindow(baseRef, 60, 90)
			got := rmsWindow(actual, 60, 90) / rmsWindow(baseActual, 60, 90)
			if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .05 {
				t.Errorf("extended=%t Z%02X: OpenMPT ratio %.3f, tracker %.3f", extended, cutoff, want, got)
			}
		}
	}
}
func TestITNoteFade(t *testing.T) {
	pattern, err := unpackITPattern([]byte{0x81, 1, 246, 0, 0x81, 0x10, 0}, 2)
	if err != nil || pattern.Rows[0][0].Note != 253 || pattern.Rows[1][0].Note != 253 {
		t.Fatalf("fade note or reuse: %+v, %v", pattern, err)
	}
	b := testITInstrument()
	ptr := int(binary.LittleEndian.Uint32(b[202:]))
	copy(b[ptr+8:], []byte{0x81, 1, 246, 0})
	if _, err := loadIT(b); err != nil {
		t.Fatal(err)
	}
}

func TestOpenMPTITNoteFade(t *testing.T) {
	fixture := func(fadeout uint16) ([]byte, []float32) {
		b := testITInstrument()
		b[50] = 1
		instrument := int(binary.LittleEndian.Uint32(b[194:]))
		b[instrument+304] = 0
		binary.LittleEndian.PutUint16(b[instrument+20:], fadeout)
		pattern := int(binary.LittleEndian.Uint32(b[202:]))
		rows := append([]byte{0x81, 3, 60, 1, 0, 0x81, 1, 246, 0}, make([]byte, 62)...)
		binary.LittleEndian.PutUint16(b[pattern:], uint16(len(rows)))
		copy(b[pattern+8:], rows)
		loaded, err := loadIT(b)
		if err != nil {
			t.Fatal(err)
		}
		w := NewTracker(loaded.Assets, referenceRate, 5760)
		w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 1, 125, -1
		pcm := make([]float32, 5760*2)
		if n := w.Read(pcm, 5760); n != 5760 {
			t.Fatalf("short render: %d", n)
		}
		return b, pcm
	}
	b, actual := fixture(256)
	base, actualBase := fixture(0)
	ref, refBase := referenceTransition(t, b), referenceTransition(t, base)
	for _, start := range []int{35, 55, 75, 95} {
		wantBase, gotBase := rmsWindow(refBase, start, start+5), rmsWindow(actualBase, start, start+5)
		if wantBase < .001 || gotBase < .001 {
			t.Fatal("silent comparison baseline")
		}
		want := rmsWindow(ref, start, start+5) / wantBase
		got := rmsWindow(actual, start, start+5) / gotBase
		if math.Abs(got-want) > .05 {
			t.Errorf("%dms: OpenMPT %g, tracker %g", start, want, got)
		}
	}
}
