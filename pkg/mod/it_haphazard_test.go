package mod

import "testing"

func TestHaphazardITPlayback(t *testing.T) {
	loaded := loadITPlaybackFixture(t, "MPTECS_IT_HAPHAZARD", "Hap Hazard")
	counts := make(map[uint8]int)
	var retriggers int
	for _, pattern := range loaded.Assets.Patterns {
		for _, row := range pattern.Rows {
			for _, cmd := range row {
				if cmd.HasVolume && (cmd.Volume == 109 || cmd.Volume == 113 || cmd.Volume == 203) {
					counts[cmd.Volume]++
				}
				if cmd.EffectCmd == 0x1b && cmd.EffectArg == 0x83 {
					retriggers++
				}
			}
		}
	}
	for _, volume := range []uint8{109, 113, 203} {
		if counts[volume] == 0 {
			t.Errorf("missing volume command %d", volume)
		}
	}
	if retriggers == 0 {
		t.Error("missing Q83")
	}
	checkITPlaybackSmoke(t, loaded)
}
