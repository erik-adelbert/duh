package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestModernTempoTickAccumulation(t *testing.T) {
	const ticks = 12000
	rows := make([][]ModCommand, ticks/4+1)
	for i := range rows {
		rows[i] = make([]ModCommand, 1)
	}
	w := NewTracker(SongAssets{
		OrderList: []int{0}, Patterns: []PatternAsset{{Rows: rows}},
		NumTracks: 1, mptRowsPerBeat: 8,
	}, 48000, 1)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 4, 115, -1
	total := 0
	for tick := range ticks {
		if !TickSchedulerSystem(w) {
			t.Fatalf("tick %d did not start", tick)
		}
		PatternFetchSystem(w)
		GlobalFlowSystem(w)
		if w.Clock.SamplesPerTick != 782 && w.Clock.SamplesPerTick != 783 {
			t.Fatalf("tick %d duration = %d", tick, w.Clock.SamplesPerTick)
		}
		total += w.Clock.SamplesPerTick
		w.Clock.SamplesRemaining = 0
	}
	want := ticks * 48000 * 60 / (115 * 4 * 8)
	if total != want {
		t.Fatalf("total duration = %d frames, want %d", total, want)
	}
}

func TestModernTempoRowChanges(t *testing.T) {
	rows := [][]ModCommand{
		{{EffectCmd: 0x0f, EffectArg: 3}, {EffectCmd: 0x14, EffectArg: 150}},
		make([]ModCommand, 2),
	}
	w := NewTracker(SongAssets{
		OrderList: []int{0}, Patterns: []PatternAsset{{Rows: rows}},
		NumTracks: 2, mptRowsPerBeat: 8,
	}, 48000, 1)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 4, 115, -1
	for tick := range 3 {
		if !TickSchedulerSystem(w) {
			t.Fatalf("tick %d did not start", tick)
		}
		PatternFetchSystem(w)
		GlobalFlowSystem(w)
		if w.Clock.Speed != 3 || w.Clock.Tempo != 150 || w.Clock.SamplesPerTick != 800 {
			t.Fatalf("tick %d: speed %d, tempo %g, frames %d", tick, w.Clock.Speed, w.Clock.Tempo, w.Clock.SamplesPerTick)
		}
		w.Clock.SamplesRemaining = 0
	}
}

func TestModernTempoKeepsFractionAcrossTempoChange(t *testing.T) {
	rows := [][]ModCommand{
		make([]ModCommand, 1),
		{{EffectCmd: 0x14, EffectArg: 137}},
		make([]ModCommand, 1),
	}
	w := NewTracker(SongAssets{
		OrderList: []int{0}, Patterns: []PatternAsset{{Rows: rows}},
		NumTracks: 1, mptRowsPerBeat: 8,
	}, 48000, 1)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 4, 115, -1
	total, exact := 0, 0.0
	for tick := range 12 {
		if !TickSchedulerSystem(w) {
			t.Fatalf("tick %d did not start", tick)
		}
		PatternFetchSystem(w)
		GlobalFlowSystem(w)
		exact += 48000.0 * 60 / (w.Clock.Tempo * float64(w.Clock.Speed) * 8)
		total += w.Clock.SamplesPerTick
		if total != int(exact) {
			t.Fatalf("tick %d: total %d, want %d", tick, total, int(exact))
		}
		w.Clock.SamplesRemaining = 0
	}
}

func TestModernTempoReadChunking(t *testing.T) {
	_, assets := transitionFixture(false, false, false)
	rows := make([][]ModCommand, 16)
	for i := range rows {
		rows[i] = make([]ModCommand, 1)
	}
	rows[0][0] = ModCommand{Note: 61, Instrument: 1}
	rows[1][0] = ModCommand{EffectCmd: 0x14, EffectArg: 137}
	rows[2][0] = ModCommand{EffectCmd: 0x0f, EffectArg: 3}
	rows[4][0] = ModCommand{Note: 254}
	assets.OrderList, assets.Patterns, assets.mptRowsPerBeat = []int{0}, []PatternAsset{{Rows: rows}}, 8
	const frames = 15000
	newWorld := func() *Tracker {
		w := NewTracker(assets, referenceRate, frames)
		w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 4, 115, -1
		return w
	}
	whole, chunked := newWorld(), newWorld()
	want, got := make([]float32, frames*2), make([]float32, frames*2)
	if n := whole.Read(want, frames); n != frames {
		t.Fatalf("whole render: %d frames", n)
	}
	for pos := 0; pos < frames; {
		n := min(257, frames-pos)
		if read := chunked.Read(got[pos*2:(pos+n)*2], n); read != n {
			t.Fatalf("chunk at %d: %d frames", pos, read)
		}
		pos += n
	}
	if whole.Clock != chunked.Clock || whole.TrackCursors[0].ActiveVoice != -1 || chunked.TrackCursors[0].ActiveVoice != -1 {
		t.Fatal("modern timing or note-cut state changed with read chunk size")
	}
}

func TestOpenMPTModernTempo(t *testing.T) {
	for _, tt := range []struct {
		speed, tempo, rowsPerBeat, effect, argument byte
	}{
		{4, 115, 8, 0, 0},
		{3, 125, 8, 0, 0},
		{6, 150, 4, 0, 0},
		{4, 115, 8, 20, 150},
		{6, 115, 8, 1, 3},
	} {
		t.Run(fmt.Sprintf("speed=%d/tempo=%d/rows-per-beat=%d/effect=%d", tt.speed, tt.tempo, tt.rowsPerBeat, tt.effect), func(t *testing.T) {
			b, assets := transitionFixture(false, false, false)
			b[30], b[31], b[46] = tt.rowsPerBeat, tt.rowsPerBeat*4, b[46]|4
			b[50], b[51] = tt.speed, tt.tempo
			packed := []byte{0x81, 3, 60, 1}
			if tt.effect != 0 {
				packed = []byte{0x81, 11, 60, 1, tt.effect, tt.argument}
			}
			packed = append(packed, []byte{0, 0x81, 1, 254, 0}...)
			packed = append(packed, make([]byte, 62)...)
			binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
			copy(b[344:], packed)
			b = b[:len(b)-11]
			b = append(b, []byte("STPM..MT\x01\x00\x02")...)
			offset := len(b)
			b = append(b, []byte("228mptm")...)
			b = binary.LittleEndian.AppendUint32(b, uint32(offset))

			ref := referenceTransition(t, b)
			rows := make([][]ModCommand, 64)
			for i := range rows {
				rows[i] = make([]ModCommand, 1)
			}
			rows[0][0] = ModCommand{Note: 61, Instrument: 1}
			tempo := tt.tempo
			switch tt.effect {
			case 1:
				rows[0][0].EffectCmd, rows[0][0].EffectArg = 0x0f, tt.argument
			case 20:
				rows[0][0].EffectCmd, rows[0][0].EffectArg = 0x14, tt.argument
				tempo = tt.argument
			}
			rows[1][0] = ModCommand{Note: 254}
			assets.OrderList, assets.Patterns = []int{0}, []PatternAsset{{Rows: rows}}
			assets.mptRowsPerBeat = uint16(tt.rowsPerBeat)
			w := NewTracker(assets, referenceRate, 5760)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = int(tt.speed), float64(tt.tempo), -1
			actual := make([]float32, 5760*2)
			if got := w.Read(actual, 5760); got != 5760 {
				t.Fatalf("rendered %d frames", got)
			}
			lastSound := func(pcm []float32) int {
				for frame := len(pcm)/2 - 1; frame >= 0; frame-- {
					if math.Abs(float64(pcm[frame*2])) > 1e-6 || math.Abs(float64(pcm[frame*2+1])) > 1e-6 {
						return frame
					}
				}
				return -1
			}
			want, got := lastSound(ref), lastSound(actual)
			expected := int(float64(referenceRate) * 60 / (float64(tempo) * float64(tt.rowsPerBeat)))
			if want < expected-4 || want > expected+4 || got < expected-4 || got > expected+4 {
				t.Fatalf("last audible frame: OpenMPT %d, tracker %d, expected about %d", want, got, expected)
			}
		})
	}
}
