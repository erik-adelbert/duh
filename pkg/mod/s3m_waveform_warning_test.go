package mod

import "testing"

func TestS3MRandomWaveformAccepted(t *testing.T) {
	for _, arg := range []byte{0x33, 0x37, 0x43, 0x53} {
		data := referenceS3M(19, arg, nil)
		loaded, err := loadS3M(data)
		if err != nil {
			t.Fatal(err)
		}
		if len(loaded.Warnings) != 0 {
			t.Errorf("S%02X warnings: %v", arg, loaded.Warnings)
		}
	}
}
