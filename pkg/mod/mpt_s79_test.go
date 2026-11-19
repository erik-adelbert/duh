package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTMS79(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"later", []byte{0x81, 3, 60, 2, 0, 0x81, 8, 19, 0x79, 0}},
		{"with note", []byte{0x81, 11, 60, 2, 19, 0x79, 0}},
		{"new note", []byte{0x81, 3, 60, 2, 0, 0x81, 8, 19, 0x79, 0, 0x81, 3, 60, 2, 0}},
		{"memory", []byte{0x81, 3, 60, 2, 0, 0x81, 8, 19, 0x79, 0, 0x81, 8, 19, 0, 0}},
		{"other channel", []byte{0x81, 3, 60, 2, 0, 0x82, 8, 19, 0x79, 0}},
	}
	for _, tt := range tests {
		for _, enabled := range []bool{false, true} {
			for _, position := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/enabled=%t/position=%t", tt.name, enabled, position), func(t *testing.T) {
					b := mptS70Fixture(t, tt.rows)
					b[bytesIndex(t, b, "...C")+6] = 2
					instrument := int(binary.LittleEndian.Uint32(b[198:]))
					envFlags := byte(0)
					if enabled {
						envFlags = 1
					}
					setTestEnvelope(b[instrument:instrument+554], 386, envFlags, []int8{-24, 24}, []uint16{0, 12})
					flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITEnvelopePositionHandling)/8
					if position {
						b[flag] |= 1 << (uint(mptITEnvelopePositionHandling) % 8)
					} else {
						b[flag] &^= 1 << (uint(mptITEnvelopePositionHandling) % 8)
					}
					ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
					for tick := range 12 {
						ms := tick*20 + 15
						rl, rr := channelRMS(ref, ms, ms+3, 0), channelRMS(ref, ms, ms+3, 1)
						al, ar := channelRMS(actual, ms, ms+3, 0), channelRMS(actual, ms, ms+3, 1)
						want, got := rl/(rl+rr), al/(al+ar)
						if math.IsNaN(want) || math.IsNaN(got) || math.Abs(got-want) > .035 {
							t.Errorf("tick %d left balance: OpenMPT %.3f tracker %.3f", tick, want, got)
						}
					}
				})
			}
		}
	}
}
func TestMPTMS79Validation(t *testing.T) {
	p, err := unpackITPatternMode([]byte{0x81, 8, 19, 0x79, 0, 0x81, 0x80, 0}, 2, true)
	if err != nil {
		t.Fatal(err)
	}

	for _, row := range p.Rows {
		if row[0].EffectCmd != 0x15 || row[0].EffectArg != 0x79 {
			t.Fatalf("unexpected S79 command: %+v", row[0])
		}
	}
	for _, arg := range []byte{0x71, 0x72, 0x7d} {
		if _, err := unpackITPatternMode([]byte{0x81, 8, 19, arg, 0}, 1, true); err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("unexpected S%02X acceptance: %v", arg, err)
		}
	}
}

func TestMPTS79ChunkingAndAllocation(t *testing.T) {
	b := mptS70Fixture(t, []byte{0x81, 3, 60, 2, 0, 0x81, 8, 19, 0x79, 0})
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	setTestEnvelope(b[instrument:instrument+554], 386, 1, []int8{-24, 24}, []uint16{0, 12})
	flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITEnvelopePositionHandling)/8
	b[flag] &^= 1 << (uint(mptITEnvelopePositionHandling) % 8)
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	const frames = 8000
	render := func(chunk int) []float32 {
		w := NewTracker(loaded.Assets, referenceRate, 1024)
		w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
		out := make([]float32, frames*2)
		for pos := 0; pos < frames; {
			n := min(chunk, frames-pos)
			if got := w.Read(out[pos*2:(pos+n)*2], n); got != n {
				t.Fatalf("short render at frame %d: %d", pos, got)
			}
			pos += n
		}
		return out
	}
	want := render(1024)
	for _, chunk := range []int{1, 17, 127, 960} {
		got := render(chunk)
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("chunk %d sample %d: got %g, want %g", chunk, i, got[i], want[i])
			}
		}
	}
	w := NewTracker(loaded.Assets, referenceRate, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 2}
	VoiceAllocationSystem(w)
	voice := w.TrackCursors[0].ActiveVoice
	if voice < 0 {
		t.Fatal("expected an active voice")
	}
	if got := testing.AllocsPerRun(100, func() {
		w.VoiceLives[voice].ITPanEnvelopeValue = -24
		w.applyITInstrumentControl(0, voice, 0x79)
	}); got != 0 {
		t.Fatalf("S79 allocated %g times", got)
	}
}

func TestOpenMPTMS79OPL(t *testing.T) {
	for _, position := range []bool{false, true} {
		t.Run(fmt.Sprintf("position=%t", position), func(t *testing.T) {
			b := mptS70Fixture(t, []byte{0x81, 3, 60, 2, 0, 0x81, 8, 19, 0x79, 0})
			instrument := int(binary.LittleEndian.Uint32(b[198:]))
			for note := range 120 {
				b[instrument+65+note*2] = 2
			}
			setTestEnvelope(b[instrument:instrument+554], 386, 1, []int8{-32, 32}, []uint16{0, 12})
			flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITEnvelopePositionHandling)/8
			if position {
				b[flag] |= 1 << (uint(mptITEnvelopePositionHandling) % 8)
			} else {
				b[flag] &^= 1 << (uint(mptITEnvelopePositionHandling) % 8)
			}
			ref, actual := mptSharedEffectRenderForDuration(t, b, 160)
			for tick := range 8 {
				ms := tick*20 + 15
				rl, rr := channelRMS(ref, ms, ms+3, 0), channelRMS(ref, ms, ms+3, 1)
				al, ar := channelRMS(actual, ms, ms+3, 0), channelRMS(actual, ms, ms+3, 1)
				want, got := rl/(rl+rr), al/(al+ar)
				if math.IsNaN(want) || math.IsNaN(got) || math.Abs(got-want) > .05 {
					t.Errorf("tick %d left balance: OpenMPT %.3f tracker %.3f", tick, want, got)
				}
			}
		})
	}
}
