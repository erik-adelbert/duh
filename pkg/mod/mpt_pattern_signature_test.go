package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestMPTPatternSignatures(t *testing.T) {
	u32 := func(v uint32) []byte { return binary.LittleEndian.AppendUint32(nil, v) }
	for _, tt := range []struct {
		name    string
		fields  []mptTestEntry
		beat    uint32
		measure uint32
		wantErr string
	}{
		{"normal", []mptTestEntry{{"RPB.", u32(8)}, {"RPM.", u32(32)}}, 8, 32, ""},
		{"maximum", []mptTestEntry{{"RPM.", u32(65536)}, {"RPB.", u32(65536)}}, 65536, 65536, ""},
		{"missing beat", []mptTestEntry{{"RPM.", u32(32)}}, 0, 0, "invalid pattern signature"},
		{"missing measure", []mptTestEntry{{"RPB.", u32(8)}}, 0, 0, "invalid pattern signature"},
		{"zero beat", []mptTestEntry{{"RPB.", u32(0)}, {"RPM.", u32(32)}}, 0, 0, "invalid pattern signature"},
		{"short measure", []mptTestEntry{{"RPB.", u32(8)}, {"RPM.", u32(7)}}, 0, 0, "invalid pattern signature"},
		{"oversize", []mptTestEntry{{"RPB.", u32(1)}, {"RPM.", u32(65537)}}, 0, 0, "invalid pattern signature"},
		{"bad width", []mptTestEntry{{"RPB.", []byte{1}}, {"RPM.", u32(4)}}, 0, 0, "invalid RPB."},
	} {
		t.Run(tt.name, func(t *testing.T) {
			patterns := []PatternAsset{{Rows: [][]ModCommand{{{}}}}}
			block := mptTestBlock("mptPc", []mptTestEntry{
				{"num", []byte{1, 0}}, {"\x00\x00", mptTestBlock("mptP", tt.fields)},
			})
			_, err := applyMPTPatternOverrides(block, patterns, 1)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %s", err, tt.wantErr)
				}
				return
			}
			if err != nil || patterns[0].mptRowsPerBeat != tt.beat || patterns[0].mptRowsPerMeasure != tt.measure {
				t.Fatalf("pattern = %+v, error %v", patterns[0], err)
			}
		})
	}
}

func TestMPTPatternTempoSwing(t *testing.T) {
	encode := func(values ...uint32) []byte {
		data := binary.LittleEndian.AppendUint16(nil, uint16(len(values)))
		for _, value := range values {
			data = binary.LittleEndian.AppendUint32(data, value)
		}
		return data
	}
	const unity = uint32(1 << 24)
	for _, tt := range []struct {
		name string
		data []byte
		rows uint32
		want []uint32
	}{
		{"empty", encode(), 4, nil},
		{"no signature", encode(unity/2, unity*3/2), 0, []uint32{}},
		{"normalized", encode(unity/2, unity*3/2), 2, []uint32{unity / 2, unity * 3 / 2}},
		{"clamped", encode(0, unity*8), 2, []uint32{4162013, 29392419}},
		{"expanded", encode(unity/2, unity*3/2), 4, []uint32{unity / 2, unity * 3 / 2, unity, unity}},
		{"truncated", encode(unity/2, unity*3/2, unity, unity), 2, []uint32{unity / 2, unity * 3 / 2}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mptPatternTempoSwing(tt.data, tt.rows)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("swing = %v, error %v, want %v", got, err, tt.want)
			}
		})
	}
	for _, data := range [][]byte{nil, {0}, {1, 0}, {0, 0, 0}} {
		if _, err := mptPatternTempoSwing(data, 4); err == nil {
			t.Fatalf("accepted malformed swing %v", data)
		}
	}
}

func mptPatternSignatureFixture(t *testing.T, modern, jump bool, overridePattern byte, swing []uint32) []byte {
	t.Helper()
	b := mptRestartPlaybackFixture(t, 0, 0, nil)
	if modern {
		b[bytesIndex(t, b, "..MT")+6] = 2
		b[30], b[31], b[46] = 4, 16, b[46]|4
	}
	if jump {
		pattern := int(binary.LittleEndian.Uint32(b[202:]))
		rows := []byte{0x81, 11, 60, 1, 3, 0x02, 0, 0, 0, 0}
		binary.LittleEndian.PutUint16(b[pattern:], uint16(len(rows)))
		copy(b[pattern+8:], rows)
	}
	tail := int(binary.LittleEndian.Uint32(b[len(b)-4:]))
	seq := mptTestBlock("mptSeq", []mptTestEntry{{"l", []byte{3, 0}}, {"a", []byte{0, 0, 1, 0, 0, 0}}})
	set := mptTestBlock("mptSeqC", []mptTestEntry{{"n", []byte{1}}, {"c", []byte{0}}, {"\x00", seq}})
	fields := []mptTestEntry{
		{"RPB.", binary.LittleEndian.AppendUint32(nil, 8)},
		{"RPM.", binary.LittleEndian.AppendUint32(nil, 24)},
	}
	if swing != nil {
		data := binary.LittleEndian.AppendUint16(nil, uint16(len(swing)))
		for _, value := range swing {
			data = binary.LittleEndian.AppendUint32(data, value)
		}
		fields = append(fields, mptTestEntry{"SWNG", data})
	}
	sig := mptTestBlock("mptP", fields)
	patterns := mptTestBlock("mptPc", []mptTestEntry{{"num", []byte{2, 0}}, {string([]byte{overridePattern, 0}), sig}})
	out := append([]byte(nil), b[:tail]...)
	out = append(out, mptTestBlock("mptm", []mptTestEntry{{"mptSeqC", set}, {"mptPc", patterns}})...)
	return binary.LittleEndian.AppendUint32(out, uint32(tail))
}

func TestOpenMPTPatternSignatureTransitions(t *testing.T) {
	for _, modern := range []bool{false, true} {
		for _, jump := range []bool{false, true} {
			for _, pattern := range []byte{0, 1} {
				for _, swing := range [][]uint32{nil, {1 << 23, 3 << 23}, {0, 1 << 27, 1 << 24}} {
					t.Run(fmt.Sprintf("modern=%t/break=%t/pattern=%d/swing=%v", modern, jump, pattern, swing), func(t *testing.T) {
						b := mptPatternSignatureFixture(t, modern, jump, pattern, swing)
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
						assertMPTPCMCorrelation(t, ref, actual, 5, min(30, len(actual)/96))
						for ms := 35; ms+20 < len(actual)/96; ms += 20 {
							assertMPTPCMCorrelation(t, ref, actual, ms, ms+20)
						}
						assertMPTPatternDelayChunking(t, b, len(actual)/2-1)
					})
				}
			}

		}
	}
}

func TestOpenMPTPatternSwingWithTempoCommands(t *testing.T) {
	for _, effect := range []struct {
		name string
		cmd  byte
		arg  byte
	}{
		{"speed", 1, 3},
		{"tempo", 20, 137},
		{"tempo slide", 20, 0x13},
		{"pattern delay", 19, 0xe2},
	} {
		t.Run(effect.name, func(t *testing.T) {
			b := mptPatternSignatureFixture(t, true, false, 1, []uint32{1 << 23, 3 << 23})
			pattern := int(binary.LittleEndian.Uint32(b[206:]))
			rows := []byte{0x81, 8, effect.cmd, effect.arg, 0, 0, 0, 0}
			copy(b[pattern+8:], rows)
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
