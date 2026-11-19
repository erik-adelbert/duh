package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTVolumePitchSlides(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"down", []byte{0x81, 3, 60, 1, 0, 0x81, 4, 106, 0, 0x81, 4, 114, 0}},
		{"up", []byte{0x81, 3, 60, 1, 0, 0x81, 4, 116, 0, 0x81, 4, 124, 0}},
		{"opposite memory", []byte{0x81, 3, 60, 1, 0, 0x81, 4, 114, 0, 0x81, 4, 115, 0}},
		{"effect memory", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 6, 0xf2, 0, 0x81, 4, 105, 0}},
		{"volume to effect memory", []byte{0x81, 3, 60, 1, 0, 0x81, 4, 116, 0, 0x81, 8, 5, 0, 0}},
		{"with volume slide", []byte{0x81, 3, 60, 1, 0, 0x81, 12, 116, 4, 16, 0}},
		{"with vibrato", []byte{0x81, 3, 60, 1, 0, 0x81, 12, 106, 8, 0x15, 0}},
		{"with new note", []byte{0x81, 3, 60, 1, 0, 0x81, 7, 72, 1, 116, 0}},
	}
	for _, tt := range tests {
		for _, older := range []bool{false, true} {
			for _, opl := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/older=%t/opl=%t", tt.name, older, opl), func(t *testing.T) {
					b := mptmPlaybackFixture()
					b[50] = 2
					b[bytesIndex(t, b, "..MT")+6] = 0
					pattern := int(binary.LittleEndian.Uint32(b[202:]))
					packed := append(append([]byte(nil), tt.rows...), make([]byte, 64)...)
					if opl {
						packed[3] = 2
					}
					binary.LittleEndian.PutUint16(b[pattern:], uint16(len(packed)))
					copy(b[pattern+8:], packed)
					loaded, err := loadMPTM(b)
					if err != nil {
						t.Fatal(err)
					}
					if !older {
						binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01300600)
						copy(b[bytesIndex(t, b, ".FSM")+6:], []byte{0x80, 0xa6, 0x37, 0xd7, 0x86, 0xf2, 0x05})
						loaded.Assets.ITCompatibility.itVolColFinePortamento = false
					}
					ref := referenceTransition(t, b)
					w := NewTracker(loaded.Assets, referenceRate, 5760)
					w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 2, 125, -1
					actual := make([]float32, 5760*2)
					if n := w.Read(actual, 5760); n != 5760 {
						t.Fatalf("short render: %d", n)
					}
					refBase, actualBase := referenceTickFrequency(t, ref, 0), referenceTickFrequency(t, actual, 0)
					for tick := 1; tick < 6; tick++ {
						want := referenceTickFrequency(t, ref, tick) / refBase
						got := referenceTickFrequency(t, actual, tick) / actualBase
						if math.Abs(got-want) > .008 {
							t.Errorf("tick %d pitch ratio: OpenMPT %.5f tracker %.5f", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestMPTVolumePitchSlideValidation(t *testing.T) {
	for _, volume := range []byte{105, 106, 114, 115, 116, 124} {
		row := []byte{0x81, 4, volume, 0}
		if _, err := unpackITPatternMode(row, 1, true); err != nil {
			t.Fatalf("MPTM volume %d: %v", volume, err)
		}
		if _, err := unpackITPattern(row, 1); err != nil {
			t.Fatalf("IT volume %d: %v", volume, err)
		}
	}
	for _, volume := range []byte{125, 126, 127} {
		if _, err := unpackITPatternMode([]byte{0x81, 4, volume, 0}, 1, true); err == nil {
			t.Fatalf("unsupported MPTM volume %d accepted", volume)
		}
	}
	for _, effect := range []byte{5, 6, 7} {
		if _, err := unpackITPatternMode([]byte{0x81, 12, 106, effect, 1, 0}, 1, true); err == nil || !strings.Contains(err.Error(), "volume pitch slide") {
			t.Fatalf("effect %d: expected explicit combination rejection, got %v", effect, err)
		}
	}
	if _, err := unpackITPatternMode([]byte{0x81, 12, 106, 19, 0xd1, 0}, 1, true); err == nil || !strings.Contains(err.Error(), "volume pitch slide") {
		t.Fatalf("note delay: expected explicit combination rejection, got %v", err)
	}
}

func TestMPTVolumePitchSlideMemoryAndIsolation(t *testing.T) {
	assets := SongAssets{
		IT: true, NumTracks: 2,
		ITCompatibility: ITCompatibility{LinearSlides: true, mptVolumePitchSlides: true},
		Samples:         []SampleAsset{{Data: make([]float32, 32), C5Speed: 8363, Volume: 64}},
	}
	w := NewTracker(assets, referenceRate, 64)
	w.Clock.Tick = 0
	w.PendingCmds[0] = ModCommand{HasVolume: true, Volume: 116}
	PitchEffectSystem(w)
	if w.TrackMemories[0].LastPortaUp != 4 || w.TrackMemories[0].LastPortaDown != 4 ||
		w.TrackMemories[0].LastPortaSpeed != 4 || w.TrackMemories[1].LastPortaUp != 0 {
		t.Fatal("volume pitch slide memory was lost or leaked across tracks")
	}
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	voice := w.TrackCursors[0].ActiveVoice
	w.PendingCmds[0] = ModCommand{HasVolume: true, Volume: 105}
	PitchEffectSystem(w)
	base := w.VoicePitches[voice].BaseFreqHz
	w.Clock.Tick = 1
	PitchEffectSystem(w)
	if w.VoicePitches[voice].BaseFreqHz >= base {
		t.Fatal("zero-parameter downward slide did not recall the previous upward slide speed")
	}
}
