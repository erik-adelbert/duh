package mod

import "testing"

func TestOccasionITPlayback(t *testing.T) {
	loaded := loadITPlaybackFixture(t, "MPTECS_IT_OCCASION", "Occasion")
	var channelSlide, filterCommand bool
	for _, pattern := range loaded.Assets.Patterns {
		for _, row := range pattern.Rows {
			for _, cmd := range row {
				channelSlide = channelSlide || cmd.EffectCmd == 0x1e && cmd.EffectArg == 0x10
				filterCommand = filterCommand || cmd.EffectCmd == 0x15 && cmd.EffectArg == 1
			}
		}
	}
	if !channelSlide || !filterCommand {
		t.Fatal("fixture is missing N10 or S01")
	}
	checkITPlaybackSmoke(t, loaded)
}
