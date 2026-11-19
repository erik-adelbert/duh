package mod

import "testing"

func TestOpenMPTS3MInitialTonePortamento(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = 224
	}
	ref, actual := referencePCM(t, referenceS3M(7, 4, sample))
	for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
		if level := rmsWindow(pcm, 5, 15); level < .005 {
			t.Errorf("%s: expected initial Gxx note, RMS=%g", name, level)
		}
	}
}
