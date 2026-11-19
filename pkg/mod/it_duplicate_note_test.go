package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestITDuplicateNotesValidation(t *testing.T) {
	for check := byte(1); check <= 3; check++ {
		for action := byte(0); action <= 2; action++ {
			b := testITInstrument()
			h := b[len(b)-554:]
			h[18], h[19] = check, action
			loaded, err := loadIT(b)
			if err != nil {
				t.Fatalf("check=%d action=%d: %v", check, action, err)
			}
			inst := loaded.Assets.ITInstruments[0]
			if inst.DuplicateCheck != check || inst.DuplicateAction != action {
				t.Fatalf("check=%d action=%d: got %+v", check, action, inst)
			}
			_, err = loadITData(b, true)
			if (err == nil) != (check == 1 && action == 0 || check == 2 && action == 0 || check == 3 && action == 1) {
				t.Fatalf("MPTM check=%d action=%d: %v", check, action, err)
			}
		}
	}
}

func TestITDuplicateNotesState(t *testing.T) {
	for check := uint8(1); check <= 3; check++ {
		for action := uint8(0); action <= 2; action++ {
			t.Run(fmt.Sprintf("check=%d/action=%d", check, action), func(t *testing.T) {
				w := NewTracker(SongAssets{IT: true, NumTracks: 2, ITInstruments: []ITInstrument{
					{DuplicateCheck: check, DuplicateAction: action, VolumeEnvelope: ITVolumeEnvelope{Enabled: true}}, {},
				}}, referenceRate, 1)
				for i := range 5 {
					w.VoiceLives[i] = VoiceLifecycle{Active: true, ParentTrack: 0, ITInstrument: 1}
					w.VoicePitches[i] = PitchState{Note: 61, itRealNote: 61}
					w.VoiceSamples[i].SampleID = 0
				}
				w.VoicePitches[1] = PitchState{Note: 62, itRealNote: 62}
				w.VoiceSamples[2].SampleID = 1
				w.VoiceLives[3].ITInstrument = 2
				w.VoiceLives[4].ParentTrack = 1
				w.TrackCursors[0].ActiveVoice = 0
				w.applyITDuplicateNotes(0, 0, 1, 61, 61)
				for i := range 5 {
					match := i == 0 || i == 1 && check != 1 || i == 2 && check != 2
					life := w.VoiceLives[i]
					if life.Active != (!match || action != 0) || life.KeyOff != (match && action == 1) || life.ITFade != (match && action == 2) {
						t.Errorf("voice %d match=%t: %+v", i, match, life)
					}
				}
			})
		}
	}
}

func TestITDuplicateNotesOpenMPT(t *testing.T) {
	for check := byte(1); check <= 3; check++ {
		for action := byte(0); action <= 2; action++ {
			for _, legacy := range []bool{false, true} {
				t.Run(fmt.Sprintf("check=%d/action=%d/legacy=%t", check, action, legacy), func(t *testing.T) {
					fixture := func(dct byte) []byte {
						b := testITDuplicateSampleFixture(dct, 1, 1, legacy, 1)
						h := b[len(b)-1108:]
						h[19] = action
						binary.LittleEndian.PutUint16(h[20:], 128)
						for i := range 4096 {
							binary.LittleEndian.PutUint16(b[512+i*2:], 12000)
						}

						return b
					}
					ref, actual := referencePCMWithChunks(t, fixture(check), "1", 140, 256)
					refBase, actualBase := referencePCMWithChunks(t, fixture(0), "1", 140, 256)
					for _, ms := range []int{38, 78, 118, 138} {
						want := channelRMS(ref, ms-1, ms, 0) / channelRMS(refBase, ms-1, ms, 0)
						got := channelRMS(actual, ms-1, ms, 0) / channelRMS(actualBase, ms-1, ms, 0)
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .06 {
							t.Errorf("%dms: OpenMPT ratio %.3f tracker %.3f", ms, want, got)
						}
					}
				})
			}
		}
	}
}
