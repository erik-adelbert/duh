package mod

import "testing"

func s3mSustainedFrequency(t *testing.T, pcm []float32) float64 {
	t.Helper()
	first, last, count := 0.0, 0.0, 0
	for i := 480; i < 18500; i++ {
		a, b := float64(pcm[2*(i-1)]), float64(pcm[2*i])
		if a <= 0 && b > 0 {
			pos := float64(i-1) - a/(b-a)
			if count == 0 {
				first = pos
			}
			last, count = pos, count+1
		}
	}
	if count < 2 {
		t.Fatal("insufficient zero crossings")
	}
	return float64(count-1) * referenceRate / (last - first)
}
