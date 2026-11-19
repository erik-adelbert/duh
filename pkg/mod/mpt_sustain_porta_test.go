package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"slices"
	"testing"
)

func mptSustainPortaFixture(t *testing.T, enabled, compatible, swap bool, instrument byte) []byte {
	return mptSustainPortaFixtureWithRelease(t, enabled, compatible, swap, instrument, true)
}

func mptSustainPortaFixtureWithRelease(t *testing.T, enabled, compatible, swap bool, instrument byte, release bool) []byte {
	return mptSustainPortaFixtureWithTail(t, enabled, compatible, swap, instrument, release, nil)
}

func mptSustainPortaFixtureWithTail(t *testing.T, enabled, compatible, swap bool, instrument byte, release bool, tail []byte) []byte {
	t.Helper()
	rows := []byte{0x81, 3, 60, 2, 0}
	if release {
		rows = append(rows, 0x81, 1, 255, 0)
	} else {
		rows = append(rows, 0)
	}
	if instrument != 0 {
		rows = append(rows, 0x81, 11, 64, instrument, 7, 4, 0)
	} else {
		rows = append(rows, 0x81, 9, 64, 7, 4, 0)
	}
	rows = append(rows, tail...)
	b := mptSustainLoopFixture(t, 1, 0)
	if swap {
		b = mptTwoPCMSamplesFixture(t, rows, true, false)
		for _, offset := range []int{202, 206} {
			sample := int(binary.LittleEndian.Uint32(b[offset:]))
			b[sample+18] |= 0x20
			binary.LittleEndian.PutUint32(b[sample+64:], 128)
			binary.LittleEndian.PutUint32(b[sample+68:], 193)
			data := int(binary.LittleEndian.Uint32(b[sample+72:]))
			for i := range 4096 {
				freq := .05
				if i >= 193 {
					freq = .27
				}
				binary.LittleEndian.PutUint16(b[data+i*2:], uint16(int16(18000*math.Sin(float64(i)*2*math.Pi*freq))))
			}
		}
	}
	first := int(binary.LittleEndian.Uint32(b[194:]))
	for note := range 120 {
		b[first+65+note*2] = 1
		if swap {
			b[first+65+note*2] = 2
		}
	}
	setTestEnvelope(b[first:first+554], 304, 1, []int8{64}, []uint16{0})
	second := int(binary.LittleEndian.Uint32(b[198:]))
	setTestEnvelope(b[second:second+554], 304, 1, []int8{64}, []uint16{0})
	if !swap {
		b = mptPatternRowsFixture(t, b, rows)
	}
	b[50] = 3
	if compatible {
		b[44] |= 0x20
	} else {
		b[44] &^= 0x20
	}
	binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01320400)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	field := bytesIndex(t, b, ".FSM")
	length := int(binary.LittleEndian.Uint16(b[field+4:]))
	flags := make([]byte, 16)
	copy(flags, b[field+6:field+6+length])
	if enabled {
		flags[mptITNoSustainOnPortamentoBit/8] |= 1 << (mptITNoSustainOnPortamentoBit % 8)
	}
	out := append([]byte(nil), b[:field]...)
	out = append(out, mptExtensionField(".FSM", uint16(len(flags)), flags)...)
	out = append(out, b[field+6+length:bounds.serializedStart]...)
	serializedStart := len(out)
	out = append(out, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(out, uint32(serializedStart))
}

func TestOpenMPTSustainPortamento(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, compatible := range []bool{false, true} {
			for _, instrument := range []byte{0, 1, 2} {
				for _, swap := range []bool{false, true} {
					t.Run(fmt.Sprintf("enabled=%t/compatible=%t/instrument=%d/swap=%t", enabled, compatible, instrument, swap), func(t *testing.T) {
						b := mptSustainPortaFixture(t, enabled, compatible, swap, instrument)
						ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
						assertMPTPCMCorrelation(t, ref, actual, 140, 180)
					})
				}
			}
		}
	}
}

func TestOpenMPTSustainPortamentoWithoutSwapReset(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, compatible := range []bool{false, true} {
			t.Run(fmt.Sprintf("enabled=%t/compatible=%t", enabled, compatible), func(t *testing.T) {
				b := mptSustainPortaFixture(t, enabled, compatible, true, 1)
				flags := bytesIndex(t, b, ".FSM") + 6
				b[flags+int(mptITPortamentoSwapResetsPos)/8] &^= 1 << (mptITPortamentoSwapResetsPos % 8)
				ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
				assertMPTPCMCorrelation(t, ref, actual, 140, 180)
			})
		}
	}
}

func TestOpenMPTSustainPortamentoBeforeRelease(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, compatible := range []bool{false, true} {
			for _, swap := range []bool{false, true} {
				t.Run(fmt.Sprintf("enabled=%t/compatible=%t/swap=%t", enabled, compatible, swap), func(t *testing.T) {
					b := mptSustainPortaFixtureWithRelease(t, enabled, compatible, swap, 1, false)
					ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
					assertMPTPCMCorrelation(t, ref, actual, 140, 180)
				})
			}
		}
	}
}

func TestMPTSustainPortamentoFlagVersionAndChunking(t *testing.T) {
	for _, tt := range []struct {
		version uint32
		enabled bool
	}{
		{0x01310000, false}, {0x01320000, true}, {0x01320012, false},
		{0x01320013, true}, {0x01320400, true},
	} {
		b := mptSustainPortaFixture(t, true, false, true, 1)
		binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], tt.version)
		loaded, err := loadMPTM(b)
		if err != nil {
			t.Fatal(err)
		}
		if got := loaded.Assets.ITCompatibility.mptNoSustainOnPortamento; got != tt.enabled {
			t.Errorf("version %#x: enabled=%t, want %t", tt.version, got, tt.enabled)
		}
		render := func(chunk int) []float32 {
			w := NewTracker(loaded.Assets, referenceRate, 256)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
			pcm := make([]float32, 9600*2)
			for frame := 0; frame < len(pcm)/2; frame += chunk {
				n := min(chunk, len(pcm)/2-frame)
				if got := w.Read(pcm[frame*2:(frame+n)*2], n); got != n {
					t.Fatalf("short render at frame %d: %d", frame, got)
				}
			}
			return pcm
		}
		if !slices.Equal(render(37), render(256)) {
			t.Errorf("version %#x: output depends on chunk size", tt.version)
		}
	}
}
