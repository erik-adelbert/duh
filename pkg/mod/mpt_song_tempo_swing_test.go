package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestMPTSongTempoSwing(t *testing.T) {
	const unity = uint32(1 << 24)
	for _, tt := range []struct {
		name   string
		input  []uint32
		rows   uint16
		expect []uint32
	}{
		{"empty", nil, 4, nil},
		{"normalized", []uint32{unity / 2, unity * 3 / 2}, 2, []uint32{unity / 2, unity * 3 / 2}},
		{"expanded", []uint32{unity / 2, unity * 3 / 2}, 4, []uint32{unity / 2, unity * 3 / 2, unity, unity}},
		{"truncated", []uint32{unity / 2, unity * 3 / 2, unity, unity}, 2, []uint32{unity / 2, unity * 3 / 2}},
		{"uniform", []uint32{unity / 2, unity / 2}, 2, []uint32{unity, unity}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			original := append([]uint32(nil), tt.input...)
			got := mptSongTempoSwing(tt.input, tt.rows)
			if !reflect.DeepEqual(got, tt.expect) || !reflect.DeepEqual(tt.input, original) {
				t.Fatalf("swing = %v, input = %v, want %v", got, tt.input, tt.expect)
			}
		})
	}
}

func mptWithSongTempoSwing(t *testing.T, data []byte, swing []uint32) []byte {
	t.Helper()
	bounds, err := locateMPTExtensions(data)
	if err != nil {
		t.Fatal(err)
	}
	field := binary.LittleEndian.AppendUint16(nil, uint16(len(swing)))
	for _, value := range swing {
		field = binary.LittleEndian.AppendUint32(field, value)
	}
	out := append([]byte(nil), data[:bounds.serializedStart]...)
	out = append(out, mptExtensionField("SWNG", uint16(len(field)), field)...)
	tail := len(out)
	out = append(out, data[bounds.serializedStart:len(data)-4]...)
	return binary.LittleEndian.AppendUint32(out, uint32(tail))
}

func TestOpenMPTSongTempoSwing(t *testing.T) {
	for _, songSwing := range [][]uint32{
		{1 << 23, 3 << 23, 1 << 24, 1 << 24},
		{1 << 23, 3 << 23},
		{0, 1 << 27, 1 << 24, 1 << 24, 1 << 24},
	} {
		for _, patternSwing := range [][]uint32{nil, {3 << 23, 1 << 23}} {
			for _, jump := range []bool{false, true} {
				t.Run(fmt.Sprintf("song=%v/pattern=%v/break=%t", songSwing, patternSwing, jump), func(t *testing.T) {
					b := mptWithSongTempoSwing(t, mptPatternSignatureFixture(t, true, jump, 1, patternSwing), songSwing)
					loaded, err := loadMPTM(b)
					if err != nil {
						t.Fatal(err)
					}

					ref := renderMPTSequenceReference(t, b, "--subsong", "0")
					w := NewTracker(loaded.Assets, referenceRate, 256)
					w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
					buf := make([]float32, 512)
					var actual []float32
					for !w.Clock.SongFinished && len(actual) < referenceRate*4 {
						n := w.Read(buf, 256)
						actual = append(actual, buf[:n*2]...)
					}
					if !w.Clock.SongFinished || math.Abs(float64(len(ref)-9600-len(actual))) > 2 {
						t.Fatalf("duration samples reference=%d actual=%d finished=%v", len(ref), len(actual), w.Clock.SongFinished)
					}
					for ms := 5; ms+20 < len(actual)/96; ms += 20 {
						assertMPTPCMCorrelation(t, ref, actual, ms, ms+20)
					}
					assertMPTPatternDelayChunking(t, b, len(actual)/2-1)
				})
			}
		}
	}
}

func TestOpenMPTSongSwingWithTempoCommands(t *testing.T) {
	for _, effect := range []struct {
		name string
		cmd  byte
		arg  byte
	}{
		{"speed", 1, 3},
		{"tempo", 20, 137},
		{"tempo slide", 20, 0x13},
		{"pattern delay", 19, 0xe2},
		{"tick delay", 19, 0x62},
	} {
		t.Run(effect.name, func(t *testing.T) {
			b := mptPatternSignatureFixture(t, true, false, 1, nil)
			pattern := int(binary.LittleEndian.Uint32(b[206:]))
			copy(b[pattern+8:], []byte{0x81, 8, effect.cmd, effect.arg, 0, 0, 0, 0})
			b = mptWithSongTempoSwing(t, b, []uint32{1 << 23, 3 << 23, 1 << 24, 1 << 24})
			loaded, err := loadMPTM(b)
			if err != nil {
				t.Fatal(err)
			}
			ref := renderMPTSequenceReference(t, b, "--subsong", "0")
			w := NewTracker(loaded.Assets, referenceRate, 256)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
			buf := make([]float32, 512)
			var actual []float32
			for !w.Clock.SongFinished && len(actual) < referenceRate*4 {
				n := w.Read(buf, 256)
				actual = append(actual, buf[:n*2]...)
			}
			if !w.Clock.SongFinished || math.Abs(float64(len(ref)-9600-len(actual))) > 2 {
				t.Fatalf("duration samples reference=%d actual=%d finished=%v", len(ref), len(actual), w.Clock.SongFinished)
			}
			assertMPTPatternDelayChunking(t, b, len(actual)/2-1)
		})
	}
}
