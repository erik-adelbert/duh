package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTModernSequenceTimingChanges(t *testing.T) {
	const tempoRaw = 1151250
	b, assets := transitionFixture(false, false, false)
	b = b[:len(b)-11]
	b[30], b[31], b[46], b[50], b[51] = 8, 32, b[46]|4, 4, 115
	rows := make([][]ModCommand, 64)
	var packed []byte
	for row := range rows {
		rows[row] = make([]ModCommand, 1)
		switch row {
		case 0, 4, 8:
			packed = append(packed, 0x81, 3, 60, 1)
			rows[row][0] = ModCommand{Note: 61, Instrument: 1}
		case 3, 7, 12:
			packed = append(packed, 0x81, 1, 254)
			rows[row][0] = ModCommand{Note: 254}
		case 2:
			packed = append(packed, 0x81, 8, 20, 150)
			rows[row][0] = ModCommand{EffectCmd: 0x14, EffectArg: 150}
		case 5:
			packed = append(packed, 0x81, 8, 1, 3)
			rows[row][0] = ModCommand{EffectCmd: 0x0f, EffectArg: 3}
		case 9:
			packed = append(packed, 0x81, 8, 20, 137)
			rows[row][0] = ModCommand{EffectCmd: 0x14, EffectArg: 137}
		}
		packed = append(packed, 0)
	}
	binary.LittleEndian.PutUint32(b[202:], uint32(len(b)))
	b = binary.LittleEndian.AppendUint16(b, uint16(len(packed)))
	b = binary.LittleEndian.AppendUint16(b, 64)
	b = append(b, 0, 0, 0, 0)
	b = append(b, packed...)
	b = append(b, []byte("STPM")...)
	b = append(b, mptExtensionField("..MT", 1, []byte{2})...)
	field := binary.LittleEndian.AppendUint16(nil, 8)
	for _, value := range []uint32{1 << 23, 3 << 23, 1 << 24, 1 << 24, 1 << 23, 3 << 23, 1 << 24, 1 << 24} {
		field = binary.LittleEndian.AppendUint32(field, value)
	}
	b = append(b, mptExtensionField("SWNG", uint16(len(field)), field)...)
	tail := len(b)
	b = append(b, mptTestSequences([]mptTestEntry{
		{"l", []byte{1, 0}}, {"a", []byte{0, 0}},
		{"t", binary.LittleEndian.AppendUint32(nil, tempoRaw)},
		{"s", binary.LittleEndian.AppendUint32(nil, 4)},
	})...)
	b = binary.LittleEndian.AppendUint32(b, uint32(tail))
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	ext, err := parseMPTExtensions(b[bounds.propertiesStart:bounds.serializedStart], 0)
	if err != nil {
		t.Fatal(err)
	}
	sets, err := parseMPTSequences(b[bounds.serializedStart:bounds.serializedEnd], 1)
	if err != nil {
		t.Fatal(err)
	}
	defaults, err := resolveMPTSequenceDefaults(sets.sequences[0], mptSequenceDefaults{tempo: 1150000, speed: 4})
	if err != nil || defaults.tempo != tempoRaw || defaults.speed != 4 {
		t.Fatalf("sequence defaults: %+v, error %v", defaults, err)
	}
	rowsPerBeat, err := projectMPTModernRowsPerBeat(ext.tempoMode, uint16(b[30]))
	if err != nil {
		t.Fatal(err)
	}
	assets.OrderList, assets.Patterns, assets.mptRowsPerBeat = []int{0}, []PatternAsset{{Rows: rows}}, rowsPerBeat
	assets.mptTempoSwing = mptSongTempoSwing(ext.tempoSwing, rowsPerBeat)
	const frames = referenceRate * 9 / 10
	w := NewTracker(assets, referenceRate, 960)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = int(defaults.speed), float64(defaults.tempo)/10000, -1
	actual := make([]float32, frames*2)
	for pos := 0; pos < frames; {
		n := min(257, frames-pos)
		if got := w.Read(actual[2*pos:2*(pos+n)], n); got != n {
			t.Fatalf("short render at frame %d: %d", pos, got)
		}
		pos += n
	}
	ref := renderMPTSequenceReference(t, b, "--repeat", "-1", "--end-time", "0.900")
	if len(ref) < len(actual) {
		t.Fatal("short OpenMPT render")
	}
	// Each cut is checked independently, including fractional tick carry across changes.
	tempo, exact := float64(tempoRaw)/10000, 0.0
	for row := 0; row <= 12; row++ {
		switch row {
		case 2:
			tempo = 150
		case 9:
			tempo = 137
		}
		if row == 3 || row == 7 || row == 12 {
			expected := int(exact)
			last := func(pcm []float32) int {
				for frame := expected + 10; frame >= expected-50; frame-- {
					if math.Abs(float64(pcm[frame*2])) > 1e-6 || math.Abs(float64(pcm[frame*2+1])) > 1e-6 {
						return frame
					}
				}
				return -1
			}
			want, got := last(ref), last(actual)
			if absInt(want-expected) > 4 || absInt(got-expected) > 4 {
				t.Errorf("row %d: OpenMPT cut=%d tracker=%d expected about %d", row, want, got, expected)
			}
		}
		exact += float64(referenceRate) * 60 / (tempo * 8) * float64(assets.mptTempoSwing[row%len(assets.mptTempoSwing)]) / (1 << 24)
	}
}
