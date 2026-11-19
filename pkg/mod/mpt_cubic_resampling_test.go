package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func mptCubicSampleModeFixture(t *testing.T, loop bool, mode byte) []byte {
	t.Helper()
	b := mptmPlaybackFixture()
	pattern := int(binary.LittleEndian.Uint32(b[202:]))
	clear(b[pattern+8 : pattern+8+int(binary.LittleEndian.Uint16(b[pattern:]))])
	copy(b[pattern+8:], []byte{0x81, 3, 96, 1, 0, 0})
	sample := int(binary.LittleEndian.Uint32(b[194:]))
	if loop {
		b[sample+18] |= 0x10
		binary.LittleEndian.PutUint32(b[sample+52:], 128)
		binary.LittleEndian.PutUint32(b[sample+56:], 2048)
	} else {
		b[sample+18] &^= 0x10
	}
	pcm := int(binary.LittleEndian.Uint32(b[sample+72:]))
	for i := range 4096 {
		value := int16(18000 * math.Sin(2*math.Pi*float64(i)*.29))
		binary.LittleEndian.PutUint16(b[pcm+i*2:], uint16(value))
	}
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	result := append([]byte(nil), b[:bounds.serializedStart]...)
	result = append(result, mptExtensionField("RSMP", 4, []byte{mode, 0, 0, 0})...)
	tail := len(result)
	result = append(result, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(result, uint32(tail))
}

func TestOpenMPTMCubicSampleMode(t *testing.T) {
	for _, loop := range []bool{false, true} {
		b := mptCubicSampleModeFixture(t, loop, 2)
		ref, actual := mptSharedEffectRenderForDuration(t, b, 100)
		assertMPTPCMCorrelation(t, ref, actual, 10, 50)
		var cross, referenceEnergy, actualEnergy float64
		for i := 10 * 96; i < 50*96; i++ {
			r, a := float64(ref[i]), float64(actual[i])
			cross += r * a
			referenceEnergy += r * r
			actualEnergy += a * a
		}
		normalizedError := math.Sqrt(math.Max(0, 1-cross*cross/(referenceEnergy*actualEnergy)))
		if normalizedError > .05 {
			t.Fatalf("loop=%v: normalized cubic error %g", loop, normalizedError)
		}
		linearRef, linearActual := mptSharedEffectRenderForDuration(t, mptCubicSampleModeFixture(t, loop, 1), 100)
		for name, pair := range map[string][2][]float32{"OpenMPT": {ref, linearRef}, "tracker": {actual, linearActual}} {
			var diff float64
			for i := 10 * 96; i < 50*96; i++ {
				d := float64(pair[0][i] - pair[1][i])
				diff += d * d
			}
			if diff < .01 {
				t.Fatalf("%s cubic and linear output did not differ: %g", name, diff)
			}
		}
	}
}

func TestMPTCubicRenderChunking(t *testing.T) {
	for _, stereo := range []bool{false, true} {
		loaded, err := loadMPTM(mptCubicSampleModeFixture(t, true, 2))
		if err != nil {
			t.Fatal(err)
		}
		if stereo {
			sample := &loaded.Assets.Samples[0]
			sample.RightData = make([]float32, len(sample.Data))
			for i, value := range sample.Data {
				sample.RightData[i] = -value
			}
		}
		render := func(chunk int) []float32 {
			w := NewTracker(loaded.Assets, referenceRate, 256)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
			pcm := make([]float32, 9600)
			for frame := 0; frame < len(pcm)/2; frame += chunk {
				n := min(chunk, len(pcm)/2-frame)
				if got := w.Read(pcm[frame*2:(frame+n)*2], n); got != n {
					t.Fatalf("short render at frame %d: %d", frame, got)
				}
			}
			return pcm
		}
		want, got := render(256), render(17)
		var energy float64
		for i := range want {
			if want[i] != got[i] {
				t.Fatalf("stereo=%v: chunk mismatch at sample %d: %g != %g", stereo, i, want[i], got[i])
			}
			energy += float64(want[i]) * float64(want[i])
		}
		if energy < .001 {
			t.Fatalf("stereo=%v: silent output", stereo)
		}
	}
}

func TestMPTCubicSampleNoAllocations(t *testing.T) {
	data := []float32{0, 1, 0, -1, 0, 1}
	for _, pos := range []float64{0.25, 2.25, 4.75} {
		if got := testing.AllocsPerRun(100, func() {
			_ = mptCubicSample(data, pos, 1, 5, true)
		}); got != 0 {
			t.Fatalf("position %g: allocated %g times", pos, got)
		}
	}
}
