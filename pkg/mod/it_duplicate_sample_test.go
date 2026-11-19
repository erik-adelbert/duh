package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestITDuplicateSampleCut(t *testing.T) {
	for _, tt := range []struct {
		name         string
		legacy       bool
		experimental bool
		instrument   int
		sample       int
		wantCut      bool
	}{
		{"same instrument and sample", false, false, 1, 0, true},
		{"different sample", false, false, 1, 1, false},
		{"different instrument", false, false, 2, 0, false},
		{"legacy different instrument", true, false, 2, 0, true},
		{"experimental different instrument", false, true, 2, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := NewTracker(SongAssets{
				IT: true, NumTracks: 2,
				ITCompatibility: ITCompatibility{LegacyModPlug: tt.legacy, itDCTSampleAcrossInstruments: tt.experimental},
				ITInstruments:   []ITInstrument{{DuplicateCheck: 2}, {}},
			}, 48000, 1)
			for i := range 4 {
				w.VoiceLives[i] = VoiceLifecycle{Active: true, ParentTrack: 0, ITInstrument: 1}
				w.VoiceSamples[i].SampleID = 0
			}
			w.VoiceLives[1].ITPastNote = true
			w.VoiceLives[2].ParentTrack = 1
			w.VoiceLives[3].ITInstrument = 2
			w.TrackCursors[0].ActiveVoice, w.TrackCursors[1].ActiveVoice = 0, 2
			w.applyITDuplicateNotes(0, tt.sample, tt.instrument, 61, 61)
			for _, i := range []int{0, 1} {
				if w.VoiceLives[i].Active == tt.wantCut {
					t.Errorf("voice %d active=%v, want cut=%v", i, w.VoiceLives[i].Active, tt.wantCut)
				}
			}
			if !w.VoiceLives[2].Active || !w.VoiceLives[3].Active || w.TrackCursors[1].ActiveVoice != 2 {
				t.Fatal("duplicate cut affected another track or an instrument without DCT")
			}
			if tt.wantCut && w.TrackCursors[0].ActiveVoice != -1 {
				t.Fatal("duplicate cut left a stale foreground voice")
			}
		})
	}
}

func testITDuplicateSampleFixture(firstInstrumentDCT, firstNNA, secondNNA byte, legacy bool, secondInstrument byte) []byte {
	b := testITInstrument()
	first := len(b) - 554
	b = append(b, b[first:first+554]...)
	binary.LittleEndian.PutUint16(b[34:], 2)
	binary.LittleEndian.PutUint32(b[198:], uint32(first+554))
	binary.LittleEndian.PutUint32(b[202:], 256)
	binary.LittleEndian.PutUint32(b[206:], 336)
	b[first+17], b[first+18], b[first+19], b[first+304] = firstNNA, firstInstrumentDCT, 0, 0
	b[first+554+17], b[first+554+18], b[first+554+304] = secondNNA, 0, 0
	b[50] = 2
	if legacy {
		binary.LittleEndian.PutUint16(b[40:], 0x217)
		binary.LittleEndian.PutUint16(b[42:], 0x200)
	}
	rows := []byte{0x81, 3, 60, 1, 0, 0x81, 3, 60, secondInstrument, 0, 0x81, 3, 60, 1, 0}
	packed := append(rows, make([]byte, 61)...)
	binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
	copy(b[344:], packed)
	return b
}

func TestOpenMPTITDuplicateSampleCut(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, instrument := range []byte{0, 1, 2} {
			t.Run(fmt.Sprintf("legacy=%v/second-instrument=%d", legacy, instrument), func(t *testing.T) {
				b := testITDuplicateSampleFixture(2, 1, 1, legacy, instrument)
				baseline := testITDuplicateSampleFixture(0, 0, 0, legacy, instrument)
				ref, actual := referencePCMWithInterpolation(t, b, "2")
				refBase, actualBase := referencePCMWithInterpolation(t, baseline, "2")
				for _, ms := range []int{28, 68, 108} {
					want := channelRMS(ref, ms, ms+5, 0) / channelRMS(refBase, ms, ms+5, 0)
					got := channelRMS(actual, ms, ms+5, 0) / channelRMS(actualBase, ms, ms+5, 0)
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .05 {
						t.Errorf("%dms: OpenMPT ratio %.3f, tracker %.3f", ms, want, got)
					}

				}

			})
		}
	}
}
