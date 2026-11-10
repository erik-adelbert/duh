package pcm

import (
	"testing"
	"time"
)

func TestRingSkip(t *testing.T) {
	f, err := NewFormat('s', 16, 1, 1000)
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewRing(f, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	data := make([]byte, 10*2)
	for i := range 10 {
		data[2*i] = byte(i)
	}
	if _, err := r.Write(data); err != nil {
		t.Fatal(err)
	}

	r.Skip(3)

	out := make([]byte, 2*2)
	if !r.ReadWindow(out, 1) {
		t.Fatal("window not available after skip")
	}
	if out[0] != 3 || out[2] != 4 {
		t.Fatalf("window starts at %d,%d; want 3,4", out[0], out[2])
	}

	r.Skip(100) // more than is buffered
	if r.ReadWindow(out, 1) {
		t.Fatal("ring should be empty after skipping everything")
	}

	r.Skip(-1) // no-op
}
