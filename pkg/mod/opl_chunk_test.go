package mod

import "testing"

func TestOPLTrackerChunkingAndIndependence(t *testing.T) {
	whole, split := testOPLWorld(ModCommand{}), testOPLWorld(ModCommand{})
	a, b := make([]float32, 5760*2), make([]float32, 5760*2)
	if whole.Read(a, 5760) != 5760 {
		t.Fatal("whole render ended early")
	}
	for i := 0; i < 5760; i += 240 {
		if split.Read(b[i*2:(i+240)*2], 240) != 240 {
			t.Fatal("split render ended early")
		}
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("sample %d: whole %g split %g", i, a[i], b[i])
		}
	}
	another := testOPLWorld(ModCommand{})
	c := make([]float32, len(a))
	if another.Read(c, 5760) != 5760 {
		t.Fatal("independent render ended early")
	}
	for i := range a {
		if a[i] != c[i] {
			t.Fatalf("independent synth differs at sample %d", i)
		}
	}
}
