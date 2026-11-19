package mod

import (
	"math"
	"testing"
)

func TestOpenMPTCarryAfterNoteOffFlagChangesReference(t *testing.T) {
	a := referenceMPTMForDuration(t, mptCarryAfterNoteOffFixture(t, false, true, 255), 240)
	b := referenceMPTMForDuration(t, mptCarryAfterNoteOffFixture(t, true, true, 255), 240)
	var delta float64
	for i := 150 * 96; i < 210*96; i++ {
		d := float64(a[i] - b[i])
		delta += d * d
	}
	if math.IsNaN(delta) || delta < .01 {
		t.Fatalf("reference does not distinguish bit 132: squared difference %g", delta)
	}
}
