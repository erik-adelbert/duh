package pcm

import (
	"testing"
	"time"
)

// midweb sizes its tap ring as 100ms; the ring must still hold a write chunk plus an analysis window.
func TestRingHoldsChunkAndWindow(t *testing.T) {
	f, err := NewFormat('s', 16, 2, 48000)
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewRing(f, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	const chunk, window = 1024, 2048 // frames
	if got := len(r.buf) / int(f.FrameSize()); got < chunk+window {
		t.Fatalf("ring holds %d frames, need at least %d", got, chunk+window)
	}
}
