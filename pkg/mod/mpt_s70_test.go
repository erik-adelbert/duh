package mod

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func mptS70Fixture(t *testing.T, rows []byte) []byte {
	t.Helper()
	b := mptResamplingFixture(t, 1, 60, true)
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	b[instrument+17], b[instrument+304] = 1, 0
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	pcm := int(binary.LittleEndian.Uint32(b[sample+72:]))
	for i := range 4096 {
		binary.LittleEndian.PutUint16(b[pcm+i*2:], 12000)
	}
	return mptPatternRowsFixture(t, b, rows)
}

func mptPatternRowsFixture(t *testing.T, b, rows []byte) []byte {
	t.Helper()
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	rows = append(append([]byte(nil), rows...), make([]byte, 64)...)
	oldLength := int(binary.LittleEndian.Uint16(b[pattern:]))
	oldTail := binary.LittleEndian.Uint32(b[len(b)-4:])
	data := append([]byte(nil), b[:pattern+8]...)
	binary.LittleEndian.PutUint16(data[pattern:], uint16(len(rows)))
	data = append(data, rows...)
	data = append(data, b[pattern+8+oldLength:len(b)-4]...)
	return binary.LittleEndian.AppendUint32(data, oldTail+uint32(len(rows)-oldLength))
}

func TestMPTS70PatternValidation(t *testing.T) {
	pattern, err := unpackITPatternMode([]byte{0x81, 8, 19, 0x70, 0}, 1, true)
	if err != nil || pattern.Rows[0][0].EffectCmd != 0x15 || pattern.Rows[0][0].EffectArg != 0x70 {
		t.Fatalf("S70: %+v, %v", pattern, err)
	}
	for _, arg := range []byte{0x71, 0x72, 0x7d} {
		if _, err := unpackITPatternMode([]byte{0x81, 8, 19, arg, 0}, 1, true); err == nil || !strings.Contains(err.Error(), "unsupported IT effect") {
			t.Fatalf("unexpected S%02X acceptance: %v", arg, err)
		}
	}
}

func TestOpenMPTMS70PastNoteCut(t *testing.T) {
	for _, withCut := range []bool{false, true} {
		t.Run(map[bool]string{false: "continue", true: "cut"}[withCut], func(t *testing.T) {
			rows := []byte{0x81, 3, 60, 2, 0, 0x81, 1, 60, 0}
			if withCut {
				rows = append(rows, 0x81, 8, 19, 0x70, 0)
			} else {
				rows = append(rows, 0)
			}

			ref, actual := mptSharedEffectRenderForDuration(t, mptS70Fixture(t, rows), 140)
			for _, win := range [][2]int{{65, 75}, {100, 110}, {120, 130}} {
				r, a := rmsWindow(ref, win[0], win[1]), rmsWindow(actual, win[0], win[1])
				if r < .001 || a < .001 || math.Abs(r-a) > .02*math.Max(r, a) {
					t.Errorf("%d-%d ms RMS: OpenMPT %g tracker %g", win[0], win[1], r, a)
				}
			}
		})
	}
}

func TestMPTS70ChunkingAndAllocation(t *testing.T) {
	rows := []byte{0x81, 3, 60, 2, 0, 0x81, 1, 60, 0, 0x81, 8, 19, 0x70, 0}
	loaded, err := loadMPTM(mptS70Fixture(t, rows))
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
	w.PendingCmds[0] = ModCommand{Note: 62, Instrument: 2}
	VoiceAllocationSystem(w)
	past := VoiceID(0)
	if !w.VoiceLives[past].Active || !w.VoiceLives[past].ITPastNote {
		t.Fatal("expected an active past note")
	}
	if got := testing.AllocsPerRun(100, func() {
		w.VoiceLives[past].Active = true
		w.applyITInstrumentControl(0, w.TrackCursors[0].ActiveVoice, 0x70)
	}); got != 0 {
		t.Fatalf("S70 allocated %g times", got)
	}
}

func TestOpenMPTMS70MemoryAndIsolation(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"memory", []byte{
			0x81, 3, 60, 2, 0,
			0x81, 1, 60, 0,
			0x81, 8, 19, 0x70, 0,
			0x81, 1, 60, 0,
			0x81, 8, 19, 0, 0,
		}},
		{"other channel", []byte{
			0x81, 3, 60, 2, 0,
			0x81, 1, 60, 0,
			0x82, 8, 19, 0x70, 0,
		}},
		{"with new note", []byte{
			0x81, 3, 60, 2, 0,
			0x81, 11, 60, 2, 19, 0x70, 0,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := mptS70Fixture(t, tt.rows)
			b[bytesIndex(t, b, "...C")+6] = 2
			ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
			for _, ms := range []int{25, 65, 105, 145, 185, 225} {
				r, a := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
				if r < .001 || a < .001 || math.Abs(r-a) > .03*math.Max(r, a) {
					t.Errorf("%d ms RMS: OpenMPT %g tracker %g", ms, r, a)
				}
			}
		})
	}
}
