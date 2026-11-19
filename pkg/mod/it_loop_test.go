package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTITPingPongLoop(t *testing.T) {
	for _, tt := range []struct {
		name   string
		speed  uint32
		legacy bool
	}{
		{"unit step", 48000, false},
		{"half step", 24000, false},
		{"double step", 96000, false},
		{"legacy unit step", 48000, true},
		{"legacy half step", 24000, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := testIT()
			b[256+18] |= 0x50
			binary.LittleEndian.PutUint32(b[256+48:], 256)
			binary.LittleEndian.PutUint32(b[256+52:], 17)
			binary.LittleEndian.PutUint32(b[256+56:], 177)
			binary.LittleEndian.PutUint32(b[256+60:], tt.speed)
			for i := range 256 {
				v := math.Sin(float64(i)*.047) + .4*math.Sin(float64(i)*.11)
				binary.LittleEndian.PutUint16(b[512+i*2:], uint16(int16(v*12000)))
			}
			b[336+8+6], b[336+8+7], b[336+8+8] = 0, 0, 0
			if tt.legacy {
				binary.LittleEndian.PutUint16(b[40:], 0x217)
				binary.LittleEndian.PutUint16(b[42:], 0x200)
			}
			ref, actual := referencePCM(t, b)
			best := -1.0
			maxShift := 0
			if tt.legacy {
				// Legacy pitch rounding also shifts forward loops relative to OpenMPT.
				maxShift = 3
			}
			for shift := -maxShift; shift <= maxShift; shift++ {
				var xy, xx, yy float64
				for frame := 30 * 48; frame < 70*48; frame++ {
					x, y := float64(ref[frame*2]), float64(actual[(frame+shift)*2])
					xy += x * y
					xx += x * x
					yy += y * y
				}
				best = math.Max(best, xy/math.Sqrt(xx*yy))
			}
			if math.IsNaN(best) || best < .995 {
				t.Errorf("waveform correlation = %.6f", best)
			}
		})
	}
}

func TestITPingPongResampler(t *testing.T) {
	for _, tt := range []struct {
		name   string
		start  uint32
		end    uint32
		step   float64
		legacy bool
		want   []float32
	}{
		{"IT unit", 2, 6, 1, false, []float32{0, 1, 2, 3, 4, 5, 5, 4, 3, 2, 3, 4, 5, 5}},
		{"legacy unit", 2, 6, 1, true, []float32{0, 1, 2, 3, 4, 5, 5, 5, 4, 3, 2, 3, 4, 5}},
		{"IT fractional", 2, 6, .5, false, []float32{0, .5, 1, 1.5, 2, 2.5, 3, 3.5, 4, 4.5, 5, 4.5, 5, 4.5, 4, 3.5}},
		{"large step", 2, 6, 11, false, []float32{0, 2, 2, 2, 2}},
		{"single sample", 2, 3, 1, false, []float32{0, 1, 2, 2, 2, 2}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assets := SongAssets{IT: true, ITCompatibility: ITCompatibility{LegacyModPlug: tt.legacy}, Samples: []SampleAsset{{
				Data: []float32{0, 1, 2, 3, 4, 5, 6}, LoopEnable: true,
				LoopPingPong: true, LoopStart: tt.start, LoopEnd: tt.end,
			}}}
			makeWorld := func() *Tracker {
				w := NewTracker(assets, 48000, len(tt.want))
				w.VoiceLives[0].Active = true
				w.VoicePitches[0].StepIncrement = tt.step
				return w
			}

			w := makeWorld()
			ResamplerSystem(w, len(tt.want))
			for i, want := range tt.want {
				if got := w.VoiceSamples[0].ScratchBuf[i]; got != want {
					t.Fatalf("frame %d: got %g, want %g", i, got, want)
				}
			}
			split := makeWorld()
			for i, want := range tt.want {
				ResamplerSystem(split, 1)
				if got := split.VoiceSamples[0].ScratchBuf[0]; got != want {
					t.Fatalf("split frame %d: got %g, want %g", i, got, want)
				}
			}
			if w.VoiceSamples[0].Position != split.VoiceSamples[0].Position || w.VoiceSamples[0].Reverse != split.VoiceSamples[0].Reverse {
				t.Fatal("position or direction differs between block sizes")
			}
		})
	}
}

func TestOpenMPTITPingPongNoteWithoutInstrument(t *testing.T) {
	b := testIT()
	b[50] = 1
	b[256+18] |= 0x50
	binary.LittleEndian.PutUint32(b[256+48:], 256)
	binary.LittleEndian.PutUint32(b[256+52:], 17)
	binary.LittleEndian.PutUint32(b[256+56:], 177)
	for i := range 256 {
		v := math.Sin(float64(i)*.047) + .4*math.Sin(float64(i)*.11)
		binary.LittleEndian.PutUint16(b[512+i*2:], uint16(int16(v*12000)))
	}
	packed := append([]byte{0x81, 3, 60, 1, 0, 0x81, 1, 60, 0}, make([]byte, 62)...)
	binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
	copy(b[344:], packed)
	ref, actual := referencePCM(t, b)
	var xy, xx, yy float64
	for frame := 37 * 48; frame < 39*48; frame++ {
		x, y := float64(ref[frame*2]), float64(actual[frame*2])
		xy, xx, yy = xy+x*y, xx+x*x, yy+y*y
	}
	if corr := xy / math.Sqrt(xx*yy); math.IsNaN(corr) || corr < .995 {
		t.Errorf("retriggered waveform correlation = %.6f", corr)
	}
}

func TestOpenMPTITPingPongBoundary(t *testing.T) {
	for _, tt := range []struct {
		name   string
		legacy bool
		want   []int
	}{
		{"IT", false, []int{172, 173, 174, 175, 176, 176, 175, 174, 173}},
		{"ModPlug", true, []int{172, 173, 174, 175, 176, 176, 176, 175, 174}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := testIT()
			b[256+18] |= 0x50
			binary.LittleEndian.PutUint32(b[256+48:], 256)
			binary.LittleEndian.PutUint32(b[256+52:], 17)
			binary.LittleEndian.PutUint32(b[256+56:], 177)
			for i := range 256 {
				binary.LittleEndian.PutUint16(b[512+i*2:], uint16(1000+i*100))
			}
			b[336+8+6], b[336+8+7], b[336+8+8] = 0, 0, 0
			if tt.legacy {
				binary.LittleEndian.PutUint16(b[40:], 0x217)
				binary.LittleEndian.PutUint16(b[42:], 0x200)
			}
			ref, _ := referencePCM(t, b)
			scale := float64(ref[160*2]) / 17000
			if scale == 0 {
				t.Fatal("silent reference")
			}
			for i, index := range tt.want {
				value := float64(ref[(172+i)*2]) / scale
				if math.Abs(value-float64(1000+index*100)) > 20 {
					t.Fatalf("frame %d: sample value %.1f, want %d", 172+i, value, 1000+index*100)
				}
			}
		})
	}
}

func TestITPingPongLoader(t *testing.T) {
	b := testIT()
	b[256+18] |= 0x50
	binary.LittleEndian.PutUint32(b[256+52:], 10)
	binary.LittleEndian.PutUint32(b[256+56:], 30)
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	s := loaded.Assets.Samples[0]
	if !s.LoopEnable || !s.LoopPingPong || s.LoopStart != 10 || s.LoopEnd != 30 {
		t.Fatalf("incorrect loop: %+v", s)
	}
	b[256+18] |= 0x80
	loaded, err = loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	if s := loaded.Assets.Samples[0]; s.mptSustainLoop || s.itSustainPingPong || !s.LoopPingPong {
		t.Fatal("inactive sustain direction flag changed the normal loop")
	}
	b[256+18] &^= 0x10
	loaded, err = loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	if s := loaded.Assets.Samples[0]; s.LoopEnable || s.LoopPingPong || s.mptSustainLoop || s.itSustainPingPong {
		t.Fatal("inactive direction flags enabled a loop")
	}
}

func TestOpenMPTITSustainLoop(t *testing.T) {
	for _, tt := range []struct {
		name   string
		flags  byte
		legacy bool
	}{
		{"forward", 0x20, false},
		{"ping-pong to tail", 0xa0, false},
		{"ping-pong to forward loop", 0xb0, false},
		{"ping-pong to ping-pong loop", 0xf0, false},
		{"legacy ping-pong to tail", 0xa0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := testITInstrument()
			b[50] = 2
			b[256+18] = 3 | tt.flags
			binary.LittleEndian.PutUint32(b[256+52:], 512)
			binary.LittleEndian.PutUint32(b[256+56:], 4096)
			binary.LittleEndian.PutUint32(b[256+64:], 17)
			binary.LittleEndian.PutUint32(b[256+68:], 177)
			setTestEnvelope(b[len(b)-554:], 304, 1, []int8{64}, []uint16{0})
			for i := range 4096 {
				v := math.Sin(float64(i)*.047) + .4*math.Sin(float64(i)*.11)
				binary.LittleEndian.PutUint16(b[512+i*2:], uint16(int16(v*12000)))
			}
			clear(b[344 : 344+int(binary.LittleEndian.Uint16(b[336:]))])
			copy(b[344:], []byte{0x81, 3, 60, 1, 0, 0x81, 1, 255, 0})
			if tt.legacy {
				binary.LittleEndian.PutUint16(b[40:], 0x217)
				binary.LittleEndian.PutUint16(b[42:], 0x200)
			}
			ref, actual := referencePCMWithChunks(t, b, "2", 120, 97)
			for _, win := range [][2]int{{10, 38}, {41, 70}, {95, 110}} {
				best := -1.0
				for shift := -3; shift <= 3; shift++ {
					var xy, xx, yy float64
					for i := win[0] * 48; i < win[1]*48; i++ {
						x, y := float64(ref[i*2]), float64(actual[(i+shift)*2])
						xy, xx, yy = xy+x*y, xx+x*x, yy+y*y
					}
					if xx > 0 && yy > 0 {
						best = math.Max(best, xy/math.Sqrt(xx*yy))
					}
				}
				if best < .98 {
					t.Errorf("%d-%d ms: waveform correlation %.5f", win[0], win[1], best)
				}
			}
		})
	}
}
