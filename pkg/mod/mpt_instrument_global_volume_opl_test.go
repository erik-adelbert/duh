package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTMInstrumentGlobalVolumeOPL(t *testing.T) {
	fixture := func(value uint32) []byte {
		b := mptInstrumentGlobalVolumeFixture(t, 64)
		field := bytesIndex(t, b, "..VG")
		binary.LittleEndian.PutUint32(b[field+6:], value)
		pattern := int(binary.LittleEndian.Uint32(b[210:]))
		b[pattern+11] = 1
		sample := int(binary.LittleEndian.Uint32(b[206:]))
		patch := int(binary.LittleEndian.Uint32(b[sample+72:]))
		copy(b[patch:patch+12], percussiveOPLPatch[:])
		return b
	}
	baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture(64), 100)
	ref, actual := mptSharedEffectRenderForDuration(t, fixture(32), 100)
	refBase, actualBase := rmsWindow(baseRef, 30, 70), rmsWindow(baseActual, 30, 70)
	if refBase < 1e-6 || actualBase < 1e-6 {
		t.Fatalf("silent OPL baseline: OpenMPT %g, tracker %g", refBase, actualBase)
	}
	want := rmsWindow(ref, 30, 70) / refBase
	got := rmsWindow(actual, 30, 70) / actualBase
	if math.IsNaN(want) || math.IsNaN(got) || math.Abs(want-got) > .05 {
		t.Errorf("OPL volume ratio: OpenMPT %g, tracker %g", want, got)
	}
}
