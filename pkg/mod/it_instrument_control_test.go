package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestITS7EnvelopeControlsOpenMPT(t *testing.T) {
	tests := []struct {
		name        string
		offset      int
		off, on     byte
		values      []int8
		initiallyOn bool
	}{
		{"volume off on", 304, 0x77, 0x78, []int8{48, 24}, true},
		{"volume on off", 304, 0x78, 0x77, []int8{48, 24}, false},
		{"pan off on", 386, 0x79, 0x7a, []int8{-24, 24}, true},
		{"pan on off", 386, 0x7a, 0x79, []int8{-24, 24}, false},
		{"pitch off on", 468, 0x7b, 0x7c, []int8{-16, 16}, true},
		{"pitch on off", 468, 0x7c, 0x7b, []int8{-16, 16}, false},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%t", tt.name, legacy), func(t *testing.T) {
				b := testITInstrument()
				b[50] = 2
				h := b[len(b)-554:]
				h[304] = 0
				flags := byte(0)
				if tt.initiallyOn {
					flags = 1
				}
				setTestEnvelope(h, tt.offset, flags, tt.values, []uint16{0, 12})
				if legacy {
					binary.LittleEndian.PutUint16(b[40:], 0x217)
					binary.LittleEndian.PutUint16(b[42:], 0x200)
				}
				fixture := testITPitchFixture(nil, 2, true, legacy)
				copy(b[512:512+8192], fixture[512:512+8192])
				b[44] |= 8
				rows := []byte{0x81, 3, 60, 1, 0, 0x81, 8, 19, tt.off, 0, 0x81, 8, 19, tt.on, 0}
				packed := append(rows, make([]byte, 64)...)
				binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
				copy(b[344:], packed)
				ref, actual := referencePCMWithChunks(t, b, "2", 120, 256)
				for tick := range 6 {
					if tt.offset == 468 {
						want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
						if math.Abs(got/want-1) > .003 {
							t.Errorf("tick %d pitch OpenMPT %.2f tracker %.2f", tick, want, got)
						}
					} else {
						start := tick*20 + 19
						refL, refR := channelRMS(ref, start, start+1, 0), channelRMS(ref, start, start+1, 1)
						gotL, gotR := channelRMS(actual, start, start+1, 0), channelRMS(actual, start, start+1, 1)
						if tt.offset == 386 {
							want, got := refL/(refL+refR), gotL/(gotL+gotR)
							if math.IsNaN(got) || math.Abs(got-want) > .035 {
								t.Errorf("tick %d balance OpenMPT %.3f tracker %.3f", tick, want, got)
							}
						} else {
							wantBase := channelRMS(ref, 19, 20, 0)
							gotBase := channelRMS(actual, 19, 20, 0)
							want, got := refL/wantBase, gotL/gotBase
							if math.IsNaN(got) || math.Abs(got-want) > .12 {
								t.Errorf("tick %d relative volume OpenMPT %.3f tracker %.3f", tick, want, got)
							}
						}
					}
				}
			})
		}
	}
}

func TestITS7PastNotesAndNewNoteActions(t *testing.T) {
	for arg := uint8(0x70); arg <= 0x76; arg++ {
		t.Run(fmt.Sprintf("S%02X", arg), func(t *testing.T) {
			assets := SongAssets{IT: true, NumTracks: 2,
				Samples:       []SampleAsset{{Data: make([]float32, 32), C5Speed: 8363, Volume: 64}},
				ITInstruments: []ITInstrument{{NewNoteAction: 1, Fadeout: 256}},
			}

			for i := range assets.ITInstruments[0].Samples {
				assets.ITInstruments[0].Samples[i], assets.ITInstruments[0].Notes[i] = 1, uint8(i)
			}
			w := NewTracker(assets, referenceRate, 16)
			w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
			VoiceAllocationSystem(w)
			w.PendingCmds[0] = ModCommand{Note: 62, Instrument: 1}
			VoiceAllocationSystem(w)
			past, current := VoiceID(0), w.TrackCursors[0].ActiveVoice
			w.PendingCmds[0] = ModCommand{EffectCmd: 0x15, EffectArg: arg}
			VolumeEffectSystem(w)
			if arg <= 0x72 {
				if w.TrackCursors[0].ActiveVoice != current || !w.VoiceLives[current].Active {
					t.Fatal("past-note control affected foreground")
				}
				if arg == 0x70 && w.VoiceLives[past].Active ||
					arg == 0x71 && !w.VoiceLives[past].KeyOff ||
					arg == 0x72 && !w.VoiceLives[past].ITFade {
					t.Fatalf("past note control failed: %+v", w.VoiceLives[past])
				}
				return
			}
			w.PendingCmds[0] = ModCommand{Note: 63, Instrument: 1}
			VoiceAllocationSystem(w)
			life := w.VoiceLives[current]
			switch arg {
			case 0x73:
				if life.Active && w.TrackCursors[0].ActiveVoice != current {
					t.Fatal("S73 did not cut old voice")
				}
			case 0x74:
				if !life.Active || !life.ITPastNote {
					t.Fatal("S74 did not continue old voice")
				}
			case 0x75:
				if !life.Active || !life.KeyOff {
					t.Fatal("S75 did not release old voice")
				}
			case 0x76:
				if !life.Active || !life.ITFade {
					t.Fatal("S76 did not fade old voice")
				}
			}
		})
	}
}

func TestITS7NewNoteActionsOpenMPT(t *testing.T) {
	for _, arg := range []byte{0x73, 0x74, 0x75, 0x76} {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("S%02X/legacy=%t", arg, legacy), func(t *testing.T) {
				fixture := func(control byte) []byte {
					b := testITInstrument()
					b[50] = 2
					h := b[len(b)-554:]
					h[17], h[304] = 1, 0
					binary.LittleEndian.PutUint16(h[20:], 128)
					for i := range 4096 {
						binary.LittleEndian.PutUint16(b[512+i*2:], 12000)
					}
					if legacy {
						binary.LittleEndian.PutUint16(b[40:], 0x217)
						binary.LittleEndian.PutUint16(b[42:], 0x200)
					}
					rows := []byte{0x81, 3, 60, 1, 0, 0x81, 8, 19, control, 0, 0x81, 1, 60, 0}
					packed := append(rows, make([]byte, 64)...)
					binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
					copy(b[344:], packed)
					return b
				}
				ref, actual := referencePCMWithChunks(t, fixture(arg), "1", 120, 256)
				refBase, actualBase := referencePCMWithChunks(t, fixture(0x73), "1", 120, 256)
				for _, ms := range []int{99, 119} {
					want := channelRMS(ref, ms-1, ms, 0) / channelRMS(refBase, ms-1, ms, 0)
					got := channelRMS(actual, ms-1, ms, 0) / channelRMS(actualBase, ms-1, ms, 0)
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .12 {
						t.Errorf("%dms: OpenMPT ratio %.3f tracker %.3f", ms, want, got)
					}
				}
			})
		}
	}
}
