package pcm

import (
	"testing"
	"time"
)

func TestFormatBufferSize(t *testing.T) {
	tests := []struct {
		in   string
		dt   time.Duration
		want int64
	}{
		{"s16c1r48000", 200 * ms, 19200},
		{"s16c2r48000", 200 * ms, 38400},
		{"s24c2r48000", 200 * ms, 57600},
		{"s8c2r44100", 200 * ms, 17640},
		{"f32c2r48000", 200 * ms, 76800},
		{"s16c6r48000", 200 * ms, 115200},
		{"s16c2r48000", 0, 38400}, // non-positive means the 200ms default
	}

	for _, tc := range tests {
		f, err := ParseFormat(tc.in)
		if err != nil {
			t.Fatalf("ParseFormat(%q): %v", tc.in, err)
		}

		if got := f.BufferSize(tc.dt); got != tc.want {
			t.Errorf("%s.BufferSize(%v) = %d, want %d", tc.in, tc.dt, got, tc.want)
		}

		if got, want := f.BufferSize(tc.dt)%f.FrameSize(), int64(0); got != want {
			t.Errorf("%s.BufferSize(%v) is not frame-aligned", tc.in, tc.dt)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		in     string
		nframe int64
		want   time.Duration
	}{
		{"s16c2r48000", 48000, time.Second},
		{"s24c2r8000", 8000, time.Second},
		{"f32c2r48000", 24000, 500 * ms},
		{"a8c1r8000", 8000, time.Second},
		// Depths that are not a multiple of 8 are stored in whole bytes.
		{"s12c1r8000", 8000, time.Second},
		{"s20c1r8000", 8000, time.Second},
		{"s4c1r8000", 8000, time.Second},
	}

	for _, tc := range tests {
		f, err := ParseFormat(tc.in)
		if err != nil {
			t.Fatalf("ParseFormat(%q): %v", tc.in, err)
		}

		if got := f.Duration(tc.nframe * f.FrameSize()); got != tc.want {
			t.Errorf("%s.Duration(%d frames) = %v, want %v", tc.in, tc.nframe, got, tc.want)
		}
	}
}
