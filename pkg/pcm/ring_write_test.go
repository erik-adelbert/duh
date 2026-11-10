package pcm

import (
	"bytes"
	"testing"
	"time"
)

// drain reads every complete frame out of r, one at a time.
func drain(r *Ring) [][]byte {
	var frames [][]byte

	out := make([]byte, r.FrameSize())
	for r.ReadWindow(out, 1) {
		frames = append(frames, bytes.Clone(out))
	}

	return frames
}

// seq returns the bytes 1..n.
func seq(n int) []byte {
	p := make([]byte, n)
	for i := range p {
		p[i] = byte(i + 1)
	}

	return p
}

func TestRingWriteKeepsFramesSplitAcrossWrites(t *testing.T) {
	tests := []struct {
		name   string
		format string
		chunks []int
	}{
		{"mono16 odd chunks", "s16c1r1000", []int{3, 5, 7, 5}},
		{"mono16 one frame split in two", "s16c1r1000", []int{3, 3}},
		{"mono16 byte at a time", "s16c1r1000", []int{1, 1, 1, 1, 1, 1, 1, 1}},
		{"stereo16 odd chunks", "s16c2r1000", []int{1, 6, 2, 3, 4}},
		{"stereo16 byte at a time", "s16c2r1000", []int{1, 1, 1, 1, 1, 1, 1, 1}},
		{"mono24 three-byte frames", "s24c1r1000", []int{2, 2, 5, 3}},
		{"chunk ends inside a frame that is still incomplete", "s16c2r1000", []int{1, 1, 6}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := ParseFormat(tc.format)
			if err != nil {
				t.Fatal(err)
			}

			r, err := NewRing(f, time.Second)
			if err != nil {
				t.Fatal(err)
			}

			fs := int(f.FrameSize())

			// The last chunk may end mid-frame; that partial frame is not readable yet.
			data := seq(sum(tc.chunks))

			rest := data
			for _, n := range tc.chunks {
				if got, err := r.Write(rest[:n]); err != nil || got != n {
					t.Fatalf("Write(%d bytes) = %d, %v", n, got, err)
				}
				rest = rest[n:]
			}

			frames := drain(r)
			if got, want := len(frames), len(data)/fs; got != want {
				t.Fatalf("read %d frames, want %d", got, want)
			}

			for i, frame := range frames {
				if want := data[i*fs : (i+1)*fs]; !bytes.Equal(frame, want) {
					t.Fatalf("frame %d = %v, want %v", i, frame, want)
				}
			}
		})
	}
}

// Callers such as io.TeeReader reuse their buffer as soon as Write returns.
func TestRingWriteDoesNotKeepCallerBuffer(t *testing.T) {
	f, err := ParseFormat("s16c1r1000")
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewRing(f, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	buf := []byte{1, 2, 3}
	if _, err := r.Write(buf); err != nil { // frame [1 2] plus the first byte of [3 4]
		t.Fatal(err)
	}

	clear(buf) // the caller recycles its buffer

	if _, err := r.Write([]byte{4, 5, 6}); err != nil {
		t.Fatal(err)
	}

	want := [][]byte{{1, 2}, {3, 4}, {5, 6}}

	got := drain(r)
	if len(got) != len(want) {
		t.Fatalf("read %d frames %v, want %v", len(got), got, want)
	}

	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Fatalf("frame %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func sum(xs []int) int {
	total := 0
	for _, x := range xs {
		total += x
	}

	return total
}
