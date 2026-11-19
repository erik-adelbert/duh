package mod

import (
	"encoding/binary"
	"testing"
)

func TestS3MIgnoredLegacyStereoControlUpdatesMemory(t *testing.T) {
	data := referenceS3M(19, 0xa1, make([]byte, 4096))
	binary.LittleEndian.PutUint16(data[0x28:], 0x1301)
	binary.LittleEndian.PutUint16(data[0x128:], 2)
	loaded, err := loadS3M(data)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, referenceRate, 1)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	if got := w.Read(make([]float32, 2), 1); got != 1 {
		t.Fatalf("read %d frames, want 1", got)
	}
	if got := w.TrackMemories[0].ITLastSpecial; got != 0xa1 {
		t.Fatalf("special-command memory = %02X, want A1", got)
	}
}
