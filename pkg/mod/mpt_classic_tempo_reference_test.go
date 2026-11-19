package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestMPTClockProjection(t *testing.T) {
	classic, alternative, modern := uint8(0), uint8(1), uint8(2)
	for _, tt := range []struct {
		name    string
		mode    *uint8
		swing   []uint32
		rows    uint16
		want    uint16
		wantErr bool
	}{
		{"absent classic", nil, nil, 8, 0, false},
		{"explicit classic", &classic, nil, 8, 0, false},
		{"classic swing", &classic, []uint32{1 << 24}, 8, 0, true},
		{"alternative", &alternative, nil, 8, 0, true},
		{"modern", &modern, nil, 8, 8, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := projectMPTClock(tt.mode, tt.rows, tt.swing)
			if (err != nil) != tt.wantErr || !tt.wantErr && got != tt.want {
				t.Fatalf("rows per beat %d, error %v", got, err)
			}
		})
	}
}

func TestOpenMPTExtendedClassicTempo(t *testing.T) {
	data := mptmPlaybackFixture()
	data[50], data[51] = 10, 255
	data[bytesIndex(t, data, "..MT")+6] = 0
	patternPos := int(binary.LittleEndian.Uint32(data[202:]))
	length := int(binary.LittleEndian.Uint16(data[patternPos:]))
	packed := data[patternPos+8 : patternPos+8+length]
	clear(packed)
	copy(packed, []byte{0x81, 3, 60, 1, 0, 0x81, 1, 254, 0})

	bounds, err := locateMPTExtensions(data)
	if err != nil {
		t.Fatal(err)
	}
	tail := bounds.serializedStart
	field := mptExtensionField("..TD", 4, binary.LittleEndian.AppendUint32(nil, 980))
	data = append(append(append([]byte(nil), data[:tail]...), field...), data[tail:]...)
	binary.LittleEndian.PutUint32(data[len(data)-4:], uint32(tail+len(field)))

	loaded, err := loadMPTM(data)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.InitialTempo != 980 || loaded.InitialSpeed != 10 || loaded.Assets.mptRowsPerBeat != 0 {
		t.Fatalf("clock = tempo %g speed %d rows per beat %d", loaded.InitialTempo, loaded.InitialSpeed, loaded.Assets.mptRowsPerBeat)
	}
	const expectedCut = 1220
	reference := renderMPTSequenceReference(t, data, "--end-time", "0.1")
	w := NewTracker(loaded.Assets, referenceRate, 4800)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	actual := make([]float32, 4800*2)
	if n := w.Read(actual, 4800); n != 4800 {
		t.Fatalf("short render: %d", n)
	}
	lastAudible := func(pcm []float32) int {
		for i := expectedCut + 20; i >= expectedCut-100; i-- {
			if math.Abs(float64(pcm[2*i])) > 1e-6 || math.Abs(float64(pcm[2*i+1])) > 1e-6 {
				return i
			}
		}
		return -1
	}
	if want, got := lastAudible(reference), lastAudible(actual); absInt(want-expectedCut) > 4 || absInt(got-want) > 4 {
		t.Fatalf("classic tempo cut: OpenMPT %d, tracker %d, expected about %d", want, got, expectedCut)
	}
}
