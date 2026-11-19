package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestITInvalidInstrumentOpenMPT(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"note and volume", []byte{0x81, 3, 60, 1, 0, 0x81, 7, 67, 88, 38, 0, 0, 0x81, 3, 60, 1, 0}},
		{"remember invalid", []byte{0x81, 3, 60, 1, 0, 0x81, 3, 67, 88, 0, 0x81, 1, 62, 0, 0x81, 3, 60, 1, 0}},
		{"with panning", []byte{0x81, 3, 60, 1, 0, 0x81, 7, 67, 88, 160, 0, 0, 0x81, 3, 60, 1, 0}},
		{"with effect", []byte{0x81, 3, 60, 1, 0, 0x81, 11, 67, 88, 13, 16, 0, 0, 0x81, 3, 60, 1, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%t", tt.name, legacy), func(t *testing.T) {
				fixture := func(rows []byte) []byte {
					b := testITInstrument()
					b[50], b[len(b)-554+304] = 2, 0
					binary.LittleEndian.PutUint32(b[256+60:], 3840)
					for i := range 4096 {
						binary.LittleEndian.PutUint16(b[512+i*2:], 12000)
					}
					if legacy {
						binary.LittleEndian.PutUint16(b[40:], 0x217)
						binary.LittleEndian.PutUint16(b[42:], 0x200)
					}
					packed := append(append([]byte(nil), rows...), make([]byte, 64)...)
					binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
					copy(b[344:], packed)
					return b
				}
				ref, actual := referencePCMWithChunks(t, fixture(tt.rows), "1", 160, 256)
				refBase, actualBase := referencePCMWithChunks(t, fixture([]byte{0x81, 3, 60, 1, 0}), "1", 160, 256)
				for _, ms := range []int{38, 58, 78, 98, 118, 138, 158} {
					for channel := range 2 {
						want := channelRMS(ref, ms-1, ms, channel) / channelRMS(refBase, ms-1, ms, channel)
						got := channelRMS(actual, ms-1, ms, channel) / channelRMS(actualBase, ms-1, ms, channel)
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .05 {
							t.Errorf("%dms channel %d: OpenMPT ratio %.3f, tracker %.3f", ms, channel, want, got)
						}
					}
				}
			})
		}
	}
}

func TestITInvalidInstrumentValidationAndMemory(t *testing.T) {
	fixture := func(rows []byte, instrumentMode, legacy bool) []byte {
		var b []byte
		if instrumentMode {
			b = testITInstrument()
		} else {
			b = testIT()
		}
		if legacy {
			binary.LittleEndian.PutUint16(b[40:], 0x217)
			binary.LittleEndian.PutUint16(b[42:], 0x200)
		}
		packed := append(append([]byte(nil), rows...), make([]byte, 64)...)
		binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
		copy(b[344:], packed)
		return b
	}
	for _, legacy := range []bool{false, true} {
		rows := []byte{0x81, 3, 60, 1, 0, 0x81, 7, 67, 88, 38, 0, 0x81, 1, 62, 0}
		loaded, err := loadIT(fixture(rows, true, legacy))
		if err != nil {
			t.Fatal(err)
		}
		w := NewTracker(loaded.Assets, referenceRate, 1)
		w.PendingCmds[0] = loaded.Assets.Patterns[0].Rows[0][0]
		VoiceAllocationSystem(w)
		v := w.TrackCursors[0].ActiveVoice
		if v == -1 {
			t.Fatal("initial note did not start")
		}
		w.PendingCmds[0] = loaded.Assets.Patterns[0].Rows[1][0]
		VoiceAllocationSystem(w)
		want := float32(38) / 64
		if legacy {
			want = 0
		}
		if w.TrackMemories[0].LastInstrument != 88 || w.TrackCursors[0].ActiveVoice != v || w.VoiceAmps[v].Volume != want {
			t.Fatalf("legacy=%t: invalid note changed foreground or volume incorrectly", legacy)
		}
		w.PendingCmds[0] = loaded.Assets.Patterns[0].Rows[2][0]
		VoiceAllocationSystem(w)
		if w.TrackCursors[0].ActiveVoice != v || w.TrackMemories[0].LastInstrument != 88 {
			t.Fatalf("legacy=%t: following note did not retain invalid instrument", legacy)
		}
	}
	for _, tt := range []struct {
		name           string
		rows           []byte
		instrumentMode bool
		mptm           bool
	}{
		{"sample mode", []byte{0x81, 3, 60, 88, 0}, false, false},
		{"instrument only", []byte{0x81, 2, 88, 0}, true, false},
		{"note cut", []byte{0x81, 3, 254, 88, 0}, true, false},
		{"tone portamento", []byte{0x81, 11, 60, 88, 7, 1, 0}, true, false},
		{"MPTM", []byte{0x81, 3, 60, 88, 0}, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadITData(fixture(tt.rows, tt.instrumentMode, false), tt.mptm)
			if err == nil || !strings.Contains(err.Error(), "references missing instrument or sample") {
				t.Fatalf("got %v, want missing instrument error", err)
			}
		})
	}
}
