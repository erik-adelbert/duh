package mod

import "testing"

func TestHamsterBallITPlayback(t *testing.T) {
	loaded := loadITPlaybackFixture(t, "MPTECS_IT_HAMSTERBALL", "Hamster Ball")
	if len(loaded.Assets.Patterns) <= 80 || len(loaded.Assets.Patterns[80].Rows) <= 110 ||
		len(loaded.Assets.Patterns[80].Rows[110]) <= 9 {
		t.Fatal("fixture is missing pattern 80 row 110 channel 10")
	}
	cmd := loaded.Assets.Patterns[80].Rows[110][9]
	if cmd.Instrument != 88 || !cmd.HasVolume || cmd.Volume != 0 {
		t.Fatalf("unexpected invalid-instrument cell: %+v", cmd)
	}
	checkITPlaybackSmoke(t, loaded)
}
