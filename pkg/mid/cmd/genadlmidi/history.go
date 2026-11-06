package main

// mono ring of the most recent samples; analysis uses whatever is stored
type history struct {
	buf  []int16
	n    int // samples stored, up to len(buf)
	head int // next write position
}

func newHistory(size int) *history { return &history{buf: make([]int16, size)} }

func (h *history) reset() { h.n, h.head = 0, 0 }

func (h *history) add(s []int16) {
	// only the newest len(buf) samples can matter
	if len(s) > len(h.buf) {
		s = s[len(s)-len(h.buf):]
	}

	k := copy(h.buf[h.head:], s)
	copy(h.buf, s[k:]) // wrapped remainder, if any

	h.head = (h.head + len(s)) % len(h.buf)
	h.n = min(h.n+len(s), len(h.buf))
}

// window copies the stored samples into dst in chronological order.
func (h *history) window(dst []int16) []int16 {
	start := 0
	if h.n == len(h.buf) {
		start = h.head
	}

	for i := range h.n {
		dst[i] = h.buf[(start+i)%len(h.buf)]
	}

	return dst[:h.n]
}
