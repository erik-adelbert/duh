package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTITChannelVolume(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"set and silence", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 13, 48, 0, 0x81, 8, 13, 0, 0}},
		{"out of range", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 13, 48, 0, 0x81, 8, 13, 65, 0}},
		{"maximum out of range", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 13, 48, 0, 0x81, 8, 13, 255, 0}},
		{"before note", []byte{0x81, 8, 13, 32, 0, 0x81, 3, 60, 1, 0, 0x81, 8, 13, 64, 0}},
		{"volume column", []byte{0x81, 3, 60, 1, 0, 0x81, 12, 86, 13, 32, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			name := tt.name + "/IT"
			if legacy {
				name = tt.name + "/ModPlug"
			}
			t.Run(name, func(t *testing.T) {
				b := testITPitchFixture(tt.rows, 2, true, legacy)
				b[256+19] = 32
				ref, actual := referencePCMWithInterpolation(t, b, "2")
				base := 36
				if tt.name == "before note" {
					base = 76
				}
				refBase := channelRMS(ref, base, base+3, 0)
				actualBase := channelRMS(actual, base, base+3, 0)
				if refBase < 1e-6 || actualBase < 1e-6 {
					t.Fatal("silent comparison baseline")
				}
				for _, ms := range []int{76, 116} {
					want := channelRMS(ref, ms, ms+3, 0) / refBase
					got := channelRMS(actual, ms, ms+3, 0) / actualBase
					if math.Abs(want-got) > .035 {
						t.Errorf("%dms: relative volume OpenMPT %.3f, tracker %.3f", ms, want, got)
					}
				}
			})
		}
	}
}

func TestOpenMPTITChannelVolumePastNote(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
		base int
	}{
		{"after new note", []byte{0x81, 3, 60, 1, 0, 0x81, 7, 60, 1, 0, 0, 0x81, 8, 13, 0, 0}, 66},
		{"on new note", []byte{0x81, 3, 60, 1, 0, 0x81, 15, 60, 1, 0, 13, 0, 0}, 36},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			name := tt.name + "/IT"
			if legacy {
				name = tt.name + "/ModPlug"
			}
			t.Run(name, func(t *testing.T) {
				b := testITInstrument()
				b[50] = 2
				b[256+18] |= 0x10
				binary.LittleEndian.PutUint32(b[256+56:], 3840)
				binary.LittleEndian.PutUint32(b[256+60:], 3840)
				b[len(b)-554+304] = 0
				if legacy {
					binary.LittleEndian.PutUint16(b[40:], 0x217)
					binary.LittleEndian.PutUint16(b[42:], 0x200)
				}
				packed := append(append([]byte(nil), tt.rows...), make([]byte, 64)...)
				binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
				copy(b[344:], packed)
				ref, actual := referencePCMWithInterpolation(t, b, "2")
				refBase := channelRMS(ref, tt.base, tt.base+3, 0)
				actualBase := channelRMS(actual, tt.base, tt.base+3, 0)
				if refBase < 1e-6 || actualBase < 1e-6 {
					t.Fatal("silent comparison baseline")
				}
				want := channelRMS(ref, 106, 109, 0) / refBase
				got := channelRMS(actual, 106, 109, 0) / actualBase
				if math.Abs(want-got) > .035 {
					t.Errorf("past note after M00: OpenMPT %.3f, tracker %.3f", want, got)
				}
			})
		}
	}
}

func TestITChannelVolumeState(t *testing.T) {
	assets := SongAssets{IT: true, NumTracks: 2, ITChannelVolume: [MaxTracks]uint8{64, 64}}
	w, other := NewTracker(assets, 48000, 1), NewTracker(assets, 48000, 1)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x16, EffectArg: 48}
	VolumeEffectSystem(w)
	if w.ITChannelVolume[0] != 48 || w.ITChannelVolume[1] != 64 || other.ITChannelVolume[0] != 64 || w.Assets.ITChannelVolume[0] != 64 {
		t.Fatal("M30 did not update only this world's channel")
	}
	w.PendingCmds[0].EffectArg = 0
	w.Clock.Tick = 1
	VolumeEffectSystem(w)
	if w.ITChannelVolume[0] != 48 {
		t.Fatal("M00 changed volume after tick zero")
	}
	w.Clock.Tick = 0
	VolumeEffectSystem(w)
	if w.ITChannelVolume[0] != 0 {
		t.Fatal("M00 did not set volume to zero")
	}
	w.PendingCmds[0].EffectArg = 65
	VolumeEffectSystem(w)
	if w.ITChannelVolume[0] != 0 {
		t.Fatal("out-of-range Mxx changed channel volume")
	}
}

func TestITChannelVolumePackedReuse(t *testing.T) {
	p, err := unpackITPattern([]byte{0x81, 8, 13, 48, 0, 0x81, 0x80, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	for row := range p.Rows {
		if cmd := p.Rows[row][0]; cmd.EffectCmd != 0x16 || cmd.EffectArg != 48 {
			t.Errorf("row %d: %+v", row, cmd)
		}
	}
}
