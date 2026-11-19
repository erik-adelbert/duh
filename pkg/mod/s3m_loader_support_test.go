package mod

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestS3MStereoSamples(t *testing.T) {
	for _, tt := range []struct {
		name  string
		flags byte
		data  []byte
		left  []float32
		right []float32
	}{
		{"8-bit", 2, []byte{0, 128, 255, 64}, []float32{-1, 0}, []float32{127.0 / 128, -0.5}},
		{"16-bit", 6, []byte{0, 0, 0, 128, 255, 255, 0, 64}, []float32{-1, 0}, []float32{32767.0 / 32768, -0.5}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := referenceS3M(0, 0, tt.data)
			data[0x100+0x1f] = tt.flags
			binary.LittleEndian.PutUint32(data[0x100+0x10:], 2)
			loaded, err := loadS3M(data)
			if err != nil {
				t.Fatal(err)
			}
			sample := loaded.Assets.Samples[0]
			for i := range tt.left {
				if sample.Data[i] != tt.left[i] || sample.RightData[i] != tt.right[i] {
					t.Fatalf("frame %d: got %v, %v; want %v, %v", i, sample.Data[i], sample.RightData[i], tt.left[i], tt.right[i])
				}
			}
			if _, err := loadS3M(data[:len(data)-1]); err == nil || !strings.Contains(err.Error(), "truncated S3M sample") {
				t.Fatalf("truncated stereo sample: %v", err)
			}
		})
	}
}

func TestOpenMPTS3MStereoSample(t *testing.T) {
	left := make([]byte, 4096)
	right := make([]byte, 4096)
	for i := range left {
		left[i] = 224
		right[i] = 160
	}
	data := referenceS3M(0, 0, append(left, right...))
	data[0x100+0x1f] = 2
	binary.LittleEndian.PutUint32(data[0x100+0x10:], uint32(len(left)))
	data[0x33] = 0x30
	ref, actual := referencePCM(t, data)
	for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
		var leftSum, rightSum float64
		for i := 480; i < 1440; i++ {
			leftSum += math.Abs(float64(pcm[2*i]))
			rightSum += math.Abs(float64(pcm[2*i+1]))
		}
		if leftSum == 0 || rightSum == 0 {
			t.Fatalf("%s: silent stereo channel: %g %g", name, leftSum, rightSum)
		}
		ratio := leftSum / rightSum
		if ratio < 2.8 || ratio > 3.2 {
			t.Errorf("%s: left/right ratio = %g, want approximately 3", name, ratio)
		}
	}
}

func TestS3MUnsupportedInstrumentWarning(t *testing.T) {
	data := referenceS3M(0, 0, nil)
	data[0x100] = 2
	loaded, err := loadS3M(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Warnings) != 1 || !strings.Contains(loaded.Warnings[0], "instrument 1") {
		t.Fatalf("warnings = %v", loaded.Warnings)
	}
}

func TestS3MPatternTruncation(t *testing.T) {
	for _, data := range [][]byte{
		{0, 1, 0, 0x20},
		{0, 2, 0, 0x20, 0x40},
		{0, 2, 0, 0},
		{0, 1, 0, 0},
	} {
		if _, err := unpackS3MPattern(data, 1, 1); err == nil {
			t.Fatalf("accepted truncated pattern %x", data)
		}
	}
}

func TestS3MOrderSkipAndJump(t *testing.T) {
	data := referenceS3M(0, 0, nil)
	data[0x60], data[0x61] = 254, 0
	loaded, err := loadS3M(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded.Assets.OrderList) != 2 || loaded.Assets.OrderList[0] != itOrderSkip || loaded.Assets.OrderList[1] != 0 {
		t.Fatalf("orders = %v", loaded.Assets.OrderList)
	}

	w := NewTracker(SongAssets{
		NumTracks: 1,
		OrderList: []int{0, itOrderSkip, 1},
		Patterns: []PatternAsset{
			{Rows: make([][]ModCommand, 64)},
			{Rows: make([][]ModCommand, 64)},
		},
	}, 48000, 64)
	w.Clock.Speed = 1
	w.Clock.Tempo = 125
	w.Clock.Tick = 0
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x0b, EffectArg: 1}
	GlobalFlowSystem(w)
	if w.Clock.Order != 2 || w.Clock.Row != -1 {
		t.Fatalf("jump to skip order: order=%d row=%d", w.Clock.Order, w.Clock.Row)
	}
	w.Clock.SamplesRemaining = 0
	if !TickSchedulerSystem(w) || w.Clock.Order != 2 || w.Clock.Row != 0 {
		t.Fatalf("jump target: order=%d row=%d finished=%v", w.Clock.Order, w.Clock.Row, w.Clock.SongFinished)
	}
}
