package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestMPTModernRowsPerBeat(t *testing.T) {
	modern, classic := uint8(2), uint8(0)
	for _, tt := range []struct {
		name  string
		mode  *uint8
		rows  uint16
		valid bool
	}{
		{"target", &modern, 8, true},
		{"no swing", &modern, 4, true},
		{"missing mode", nil, 4, false},
		{"classic mode", &classic, 4, false},
		{"zero rows", &modern, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := projectMPTModernRowsPerBeat(tt.mode, tt.rows)
			if (err == nil) != tt.valid || tt.valid && got != tt.rows {
				t.Fatalf("rows per beat %d, error %v", got, err)
			}
		})
	}
}

func TestOpenMPTModernNeutralSwing(t *testing.T) {
	for _, swing := range []bool{false, true} {
		for _, effect := range []byte{0, 20, 1} {
			for _, cutRow := range []int{1, 16} {
				t.Run(fmt.Sprintf("swing=%v/effect=%d/cut-row=%d", swing, effect, cutRow), func(t *testing.T) {
					b, assets := transitionFixture(false, false, false)
					b = b[:len(b)-11]
					b[30], b[31], b[46], b[50], b[51] = 8, 32, b[46]|4, 4, 115
					packed := []byte{0x81, 3, 60, 1}
					cmd := ModCommand{Note: 61, Instrument: 1}
					tempo := 115
					if effect != 0 {
						arg := byte(150)
						if effect == 1 {
							arg = 3
							cmd.EffectCmd = 0x0f
						} else {
							cmd.EffectCmd = 0x14
							tempo = int(arg)
						}
						cmd.EffectArg = arg
						packed = []byte{0x81, 11, 60, 1, effect, arg}
					}
					packed = append(packed, make([]byte, cutRow)...)
					packed = append(packed, 0x81, 1, 254, 0)
					packed = append(packed, make([]byte, 63-cutRow)...)
					binary.LittleEndian.PutUint32(b[202:], uint32(len(b)))
					b = binary.LittleEndian.AppendUint16(b, uint16(len(packed)))
					b = binary.LittleEndian.AppendUint16(b, 64)
					b = append(b, 0, 0, 0, 0)
					b = append(b, packed...)
					b = append(b, []byte("STPM")...)
					b = append(b, mptExtensionField("..MT", 1, []byte{2})...)
					if swing {
						field := binary.LittleEndian.AppendUint16(nil, 8)
						for range 8 {
							field = binary.LittleEndian.AppendUint32(field, 1<<24)
						}
						b = append(b, mptExtensionField("SWNG", uint16(len(field)), field)...)
					}
					tail := len(b)
					b = append(b, mptTestSequences([]mptTestEntry{{"l", []byte{1, 0}}, {"a", []byte{0, 0}}})...)
					b = binary.LittleEndian.AppendUint32(b, uint32(tail))
					bounds, err := locateMPTExtensions(b)
					if err != nil {
						t.Fatal(err)
					}
					ext, err := parseMPTExtensions(b[bounds.propertiesStart:bounds.serializedStart], 0)
					if err != nil {
						t.Fatal(err)
					}
					if swing != (len(ext.tempoSwing) == 8) {
						t.Fatalf("unexpected serialized swing: %v", ext.tempoSwing)
					}
					if _, err := parseMPTSequences(b[bounds.serializedStart:bounds.serializedEnd], 1); err != nil {
						t.Fatal(err)
					}
					rowsPerBeat, err := projectMPTModernRowsPerBeat(ext.tempoMode, uint16(b[30]))
					if err != nil {
						t.Fatal(err)
					}
					rows := make([][]ModCommand, 64)
					for i := range rows {
						rows[i] = make([]ModCommand, 1)
					}
					rows[0][0], rows[cutRow][0] = cmd, ModCommand{Note: 254}
					assets.OrderList, assets.Patterns, assets.mptRowsPerBeat = []int{0}, []PatternAsset{{Rows: rows}}, rowsPerBeat
					const frames = referenceRate * 5 / 4
					w := NewTracker(assets, referenceRate, 960)
					w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 4, 115, -1
					actual := make([]float32, frames*2)
					for pos := 0; pos < frames; {
						n := min(960, frames-pos)
						if got := w.Read(actual[pos*2:(pos+n)*2], n); got != n {
							t.Fatalf("short render: %d", got)
						}
						pos += n
					}
					ref := renderMPTSequenceReference(t, b, "--repeat", "-1", "--end-time", "1.250")
					if len(ref) < frames*2 {
						t.Fatalf("short reference: %d samples", len(ref))
					}
					last := func(pcm []float32) int {
						for frame := len(pcm)/2 - 1; frame >= 0; frame-- {
							if math.Abs(float64(pcm[frame*2])) > 1e-6 || math.Abs(float64(pcm[frame*2+1])) > 1e-6 {
								return frame
							}
						}
						return -1
					}
					expected := referenceRate * 60 * cutRow / (tempo * 8)
					if want, got := last(ref), last(actual); absInt(want-expected) > 4 || absInt(got-expected) > 4 {
						t.Fatalf("last audible frame: OpenMPT %d, tracker %d, expected about %d", want, got, expected)
					}
				})
			}
		}
	}
}
