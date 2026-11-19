package mod

import "testing"

func TestAlchemistITPlayback(t *testing.T) {
	loaded := loadITPlaybackFixture(t, "MPTECS_IT_ALCHEMIST", "Alchemist")
	counts := make(map[uint8]int)
	for _, pattern := range loaded.Assets.Patterns {
		for _, row := range pattern.Rows {
			for _, cmd := range row {
				if cmd.EffectCmd == 0x1b {
					counts[cmd.EffectArg]++
				}
			}
		}
	}
	if counts[0] == 0 || counts[4] == 0 {
		t.Fatalf("missing Q04 or Q00: %v", counts)
	}
	checkITPlaybackSmoke(t, loaded)
}
