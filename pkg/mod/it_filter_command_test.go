package mod

import "testing"

func TestITS0FilterCommandMemory(t *testing.T) {
	for arg := byte(1); arg < 16; arg++ {
		p, err := unpackITPattern([]byte{0x81, 8, 19, arg, 0, 0x81, 8, 19, 0, 0}, 2)
		if err != nil {
			t.Fatalf("S%02X: %v", arg, err)
		}
		w := NewTracker(SongAssets{IT: true, NumTracks: 1}, referenceRate, 1)
		w.TrackCursors[0].Panning = .25
		w.TrackMemories[0].ITLastSpecial = 0x80
		for row := range p.Rows {
			w.PendingCmds[0] = p.Rows[row][0]
			VolumeEffectSystem(w)
			if w.TrackMemories[0].ITLastSpecial != arg || w.TrackCursors[0].Panning != .25 {
				t.Fatalf("S%02X row %d: special memory=%02X panning=%v", arg, row, w.TrackMemories[0].ITLastSpecial, w.TrackCursors[0].Panning)
			}
		}
		if _, err := unpackITPatternMode([]byte{0x81, 8, 19, arg, 0}, 1, true); err == nil {
			t.Fatalf("unsupported MPTM S%02X accepted", arg)
		}
	}
}
