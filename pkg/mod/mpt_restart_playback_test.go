package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func mptRestartPlaybackFixture(t *testing.T, jump byte, legacy byte, override *byte) []byte {
	t.Helper()
	b := mptPortaFilterFixture(t, false)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	pattern := int(binary.LittleEndian.Uint32(b[202:]))
	first := []byte{0x81, 3, 60, 1, 0, 0, 0, 0}
	if jump != 0 {
		first = []byte{0x81, 11, 60, 1, 2, jump, 0, 0, 0, 0}
	}
	binary.LittleEndian.PutUint16(b[pattern:], uint16(len(first)))
	binary.LittleEndian.PutUint16(b[pattern+2:], 4)
	copy(b[pattern+8:], first)
	second := make([]byte, 8)
	rows := []byte{0x81, 3, 72, 1, 0, 0, 0, 0}
	binary.LittleEndian.PutUint16(second, uint16(len(rows)))
	binary.LittleEndian.PutUint16(second[2:], 4)
	second = append(second, rows...)
	binary.LittleEndian.PutUint16(b[38:], 2)
	binary.LittleEndian.PutUint32(b[206:], uint32(bounds.propertiesStart))
	out := append([]byte(nil), b[:bounds.propertiesStart]...)
	out = append(out, second...)
	out = append(out, b[bounds.propertiesStart:bounds.serializedStart]...)
	out = append(out, mptExtensionField("..PR", 2, []byte{legacy, 0})...)
	tail := len(out)
	entries := []mptTestEntry{{"l", []byte{2, 0}}, {"a", []byte{0, 0, 1, 0}}}
	if override != nil {
		entries = append(entries, mptTestEntry{"r", []byte{*override, 0}})
	}
	out = append(out, mptTestSequences(entries)...)
	return binary.LittleEndian.AppendUint32(out, uint32(tail))
}

func TestOpenMPTNoRepeatRestart(t *testing.T) {
	zero, one := byte(0), byte(1)
	for _, tt := range []struct {
		name     string
		jump     byte
		legacy   byte
		override *byte
	}{
		{"natural end restart zero", 0, 0, nil},
		{"natural end restart one", 0, 1, nil},
		{"out of range restart zero", 99, 0, nil},
		{"out of range legacy restart one", 99, 1, nil},
		{"override legacy restart", 99, 1, &zero},
		{"override restart one", 99, 0, &one},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := mptRestartPlaybackFixture(t, tt.jump, tt.legacy, tt.override)
			ref := renderMPTSequenceReference(t, b, "--subsong", "0")
			loaded, err := loadMPTM(b)
			if err != nil {
				t.Fatal(err)
			}
			w := NewTracker(loaded.Assets, referenceRate, 256)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
			buf := make([]float32, 512)
			var actual []float32
			for !w.Clock.SongFinished && len(actual) < referenceRate*4 {
				n := w.Read(buf, 256)
				actual = append(actual, buf[:n*2]...)
			}
			// openmpt123 appends 100 ms of post-song output.
			if !w.Clock.SongFinished || math.Abs(float64(len(ref)-9600-len(actual))) > 2 {
				t.Fatalf("duration samples reference=%d actual=%d finished=%v", len(ref), len(actual), w.Clock.SongFinished)
			}
			if len(actual) > 4000 {
				assertMPTPCMCorrelation(t, ref, actual, 5, min(30, len(actual)/96))
			}
			if tt.jump != 0 && len(actual) > 100*96 {
				assertMPTPCMCorrelation(t, ref, actual, 50, 90)
			}
		})
	}
}
