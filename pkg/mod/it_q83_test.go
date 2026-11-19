package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestITQ83TickState(t *testing.T) {
	w := NewTracker(SongAssets{IT: true, NumTracks: 1}, referenceRate, 1)
	w.TrackCursors[0].ActiveVoice = 0
	w.VoiceLives[0].Active = true
	w.VoiceAmps[0].Volume = .4
	w.PendingCmds[0] = ModCommand{Note: 61, EffectCmd: 0x1b, EffectArg: 0x83}
	for tick := range 12 {
		w.Clock.Tick = tick % 6
		if tick == 6 {
			w.PendingCmds[0].Note = 0
		}
		w.VoiceSamples[0].Position = 100
		PitchEffectSystem(w)
		want := float64(100)
		if tick == 3 || tick == 6 || tick == 9 {
			want = 0
		}
		if got := w.VoiceSamples[0].Position; got != want {
			t.Errorf("tick %d: position %g, want %g", tick, got, want)
		}
		if got := w.VoiceAmps[0].Volume; got != .4 {
			t.Errorf("tick %d: volume %g, want .4", tick, got)
		}
	}
}

func TestITQ83OpenMPT(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"note and consecutive retrigger rows", []byte{
			0x81, 15, 60, 1, 22, 17, 0x83, 0,
			0x81, 12, 22, 17, 0x83, 0,
		}},
		{"retrigger without note", []byte{
			0x81, 3, 60, 1, 0,
			0x81, 8, 17, 0x83, 0,
		}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			for _, instrument := range []bool{false, true} {
				name := tt.name
				if legacy {
					name += "/legacy"
				}
				if instrument {
					name += "/instrument"
				}
				t.Run(name, func(t *testing.T) {
					b := testITPitchFixture(tt.rows, 6, false, legacy)
					if instrument {
						b = testITInstrument()
						b[len(b)-554+304] = 0
						if legacy {
							binary.LittleEndian.PutUint16(b[40:], 0x217)
							binary.LittleEndian.PutUint16(b[42:], 0x200)
						}
						packed := append(append([]byte(nil), tt.rows...), make([]byte, 64)...)
						binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
						copy(b[344:], packed)
					}
					binary.LittleEndian.PutUint32(b[256+60:], 48000)
					for i := range 4096 {
						var sample int16
						if i < 700 {
							sample = 16000
						}
						binary.LittleEndian.PutUint16(b[512+i*2:], uint16(sample))
					}
					ref, actual := referencePCMWithChunks(t, b, "1", 240, 256)
					refBaseline, actualBaseline := rmsWindow(ref, 8, 10), rmsWindow(actual, 8, 10)
					for tick := range 12 {
						want := rmsWindow(ref, tick*20+8, tick*20+10)
						got := rmsWindow(actual, tick*20+8, tick*20+10)
						if (want > 0.001) != (got > 0.001) {
							t.Errorf("tick %d: OpenMPT RMS %g, tracker RMS %g", tick, want, got)
						}
						if want > 0.001 && (math.IsNaN(got) || math.Abs(got/actualBaseline-want/refBaseline) > .12) {
							t.Errorf("tick %d: normalized RMS OpenMPT %g, tracker %g", tick, want/refBaseline, got/actualBaseline)
						}
					}
				})
			}
		}
	}
}

func TestITQ83Validation(t *testing.T) {
	if _, err := unpackITPattern([]byte{0x81, 8, 17, 0x83, 0}, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := unpackITPattern([]byte{0x81, 8, 17, 0x82, 0}, 1); err != nil {
		t.Fatal(err)
	}
}
