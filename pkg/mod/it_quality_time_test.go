package mod

import "testing"

func TestQualityTimeITPlayback(t *testing.T) {
	loaded := loadITPlaybackFixture(t, "MPTECS_IT_QUALITY_TIME", "Quality Time")
	if len(loaded.Assets.Patterns) <= 71 || len(loaded.Assets.Patterns[71].Rows) <= 57 ||
		len(loaded.Assets.Patterns[71].Rows[57]) <= 13 {
		t.Fatal("fixture is missing pattern 71 row 57 channel 14")
	}
	cmd := loaded.Assets.Patterns[71].Rows[57][13]
	if cmd.Instrument != 26 || cmd.Note != 0 {
		t.Fatalf("unexpected instrument-only cell: %+v", cmd)
	}
	checkITPlaybackSmoke(t, loaded)
}
