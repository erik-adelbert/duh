package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTModernTickAfterTimingChanges(t *testing.T) {
	const initialTempo = 1151250
	for _, tt := range []struct {
		speed, cutTick byte
	}{
		{2, 1}, {3, 2}, {7, 6},
	} {
		t.Run(fmt.Sprintf("speed=%d/cut=%d", tt.speed, tt.cutTick), func(t *testing.T) {
			b, assets := transitionFixture(false, false, false)
			b = b[:len(b)-11]
			b[30], b[31], b[46], b[50], b[51] = 8, 32, b[46]|4, 4, 115
			packed := []byte{
				0x81, 3, 60, 1, 0,
				0x81, 8, 19, 0xc0 | tt.cutTick,
				0x82, 8, 1, tt.speed,
				0x83, 8, 20, 137, 0,
			}
			packed = append(packed, make([]byte, 62)...)
			binary.LittleEndian.PutUint32(b[202:], uint32(len(b)))
			b = binary.LittleEndian.AppendUint16(b, uint16(len(packed)))
			b = binary.LittleEndian.AppendUint16(b, 64)
			b = append(b, 0, 0, 0, 0)
			b = append(b, packed...)
			b = append(b, []byte("STPM")...)
			b = append(b, mptExtensionField("..MT", 1, []byte{2})...)
			flags := []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}
			b = append(b, mptExtensionField(".FSM", uint16(len(flags)), flags)...)
			tail := len(b)
			b = append(b, mptTestSequences([]mptTestEntry{
				{"l", []byte{1, 0}}, {"a", []byte{0, 0}},
				{"t", binary.LittleEndian.AppendUint32(nil, initialTempo)},
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
			if saved, err := parseMPTSavedPlayBehaviours(ext.playbackFlags); err != nil || !saved.enabled(mptITSCxStopsSample) {
				t.Fatalf("reference flags=%#x, error=%v", saved, err)
			}
			sequences, err := parseMPTSequences(b[bounds.serializedStart:bounds.serializedEnd], 1)
			if err != nil {
				t.Fatal(err)
			}
			defaults, err := resolveMPTSequenceDefaults(sequences.sequences[0], mptSequenceDefaults{tempo: 1150000, speed: 4})
			if err != nil || defaults.tempo != initialTempo || defaults.speed != 4 {
				t.Fatalf("sequence defaults: %+v, error %v", defaults, err)
			}
			rowsPerBeat, err := projectMPTModernRowsPerBeat(ext.tempoMode, uint16(b[30]))
			if err != nil {
				t.Fatal(err)
			}
			pattern, err := unpackITPattern(packed, 64)
			if err != nil {
				t.Fatal(err)
			}
			assets.NumTracks, assets.OrderList, assets.Patterns, assets.mptRowsPerBeat = 3, []int{0}, []PatternAsset{pattern}, rowsPerBeat
			const frames = referenceRate / 5
			reference := renderMPTSequenceReference(t, b, "--repeat", "-1", "--end-time", "0.200")
			if len(reference) < frames*2 {
				t.Fatal("short OpenMPT render")
			}
			exact := float64(referenceRate)*60/(float64(initialTempo)/10000*8) +
				float64(referenceRate)*60*float64(tt.cutTick)/(137*8*float64(tt.speed))
			expected := int(exact)
			findLast := func(pcm []float32) int {
				for frame := expected + 10; frame >= expected-50; frame-- {
					if math.Abs(float64(pcm[frame*2])) > 1e-6 || math.Abs(float64(pcm[frame*2+1])) > 1e-6 {
						return frame
					}
				}
				return -1
			}
			checkCut := func(name string, pcm []float32) int {
				t.Helper()
				var before float64
				for frame := expected - 100; frame < expected-20; frame++ {
					before = math.Max(before, math.Abs(float64(pcm[frame*2])))
				}
				if before < .001 {
					t.Errorf("%s: no audible signal before cut", name)
				}
				for frame := expected + 20; frame < frames; frame++ {
					if math.Abs(float64(pcm[frame*2])) > 1e-6 || math.Abs(float64(pcm[frame*2+1])) > 1e-6 {
						t.Errorf("%s: signal remains after cut at frame %d", name, frame)
						break
					}
				}
				return findLast(pcm)
			}
			want := checkCut("OpenMPT", reference)
			if absInt(want-expected) > 4 {
				t.Fatalf("OpenMPT cut=%d, expected about %d", want, expected)
			}
			for _, chunkSize := range []int{127, 960} {
				w := NewTracker(assets, referenceRate, 960)
				w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = int(defaults.speed), float64(defaults.tempo)/10000, -1
				actual := make([]float32, frames*2)
				for pos := 0; pos < frames; {
					n := min(chunkSize, frames-pos)
					if got := w.Read(actual[2*pos:2*(pos+n)], n); got != n {
						t.Fatalf("short render at frame %d: %d", pos, got)
					}
					pos += n
				}
				if got := checkCut(fmt.Sprintf("tracker chunk %d", chunkSize), actual); absInt(got-want) > 4 {
					t.Errorf("chunk %d: tracker cut=%d, OpenMPT=%d", chunkSize, got, want)
				}
			}
		})
	}
}
