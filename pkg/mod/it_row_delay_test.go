package mod

import (
	"fmt"
	"strings"
	"testing"
)

func TestOpenMPTITRowDelay(t *testing.T) {
	for _, tt := range []struct {
		first, second byte
		noteOnDelay   bool
	}{
		{0xe0, 0xe2, false},
		{0xe2, 0xe1, false},
		{0xe2, 0xe1, true},
		{0xef, 0xe0, false},
	} {
		t.Run(fmt.Sprintf("S%02X-S%02X/note=%t", tt.first, tt.second, tt.noteOnDelay), func(t *testing.T) {
			var rows []byte
			if tt.noteOnDelay {
				rows = append(rows, 0x81, 11, 60, 1, 19, tt.first)
			} else {
				rows = append(rows, 0x81, 8, 19, tt.first)
			}
			rows = append(rows, 0x82, 8, 19, tt.second, 0, 0x81, 3, 60, 1, 0)
			b := testITPitchFixture(rows, 5, true, false)
			if tt.noteOnDelay {
				// Make note attacks audible but leave the rest of the sample silent.
				b[256+18] &^= 0x10
				clear(b[512+1800*2 : 512+4096*2])
			}
			onset := 100 * (1 + int(tt.first&15))
			ref, actual := referencePCMWithChunks(t, b, "2", onset+30, 97)
			for _, renderer := range []struct {
				name string
				pcm  []float32
			}{{"OpenMPT", ref}, {"tracker", actual}} {
				if got := rmsWindow(renderer.pcm, onset+5, onset+25); got < .005 {
					t.Errorf("%s: no note after delayed row at %d ms (RMS %g)", renderer.name, onset, got)
				}
				start := 0
				if tt.noteOnDelay {
					if got := rmsWindow(renderer.pcm, 5, 25); got < .005 {
						t.Errorf("%s: no note at start of delayed row (RMS %g)", renderer.name, got)
					}
					start = 85
				}
				if got := rmsWindow(renderer.pcm, start, onset-1); got > .00001 {
					t.Errorf("%s: unexpected note before advancing (RMS %g)", renderer.name, got)
				}
			}
		})
	}
}

func TestITRowDelayValidation(t *testing.T) {
	for _, rows := range [][]byte{
		{0x81, 8, 19, 0xe2, 0x82, 8, 19, 0xd1, 0},
		{0x81, 8, 19, 0xe2, 0x82, 4, 48, 0},
		{0x81, 8, 19, 0xe2, 0, 0x81, 8, 19, 0, 0},
	} {
		_, err := loadIT(testITPitchFixture(rows, 5, true, false))
		if err == nil || !strings.Contains(err.Error(), "pattern delay") {
			t.Errorf("unsupported row delay interaction: got %v", err)
		}
	}
}
