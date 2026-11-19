package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestMPTResamplingExtensions(t *testing.T) {
	for _, width := range []int{1, 2, 4} {
		values := make([]byte, width*2)
		values[0], values[width] = 1, 3
		data := append([]byte("XTPM"), mptExtensionField("...R", uint16(width), values)...)
		ext, err := parseMPTExtensions(data, 2)
		if err != nil || len(ext.instrumentResampling) != 2 || ext.instrumentResampling[0] != 1 || ext.instrumentResampling[1] != 3 {
			t.Fatalf("width %d: %+v, %v", width, ext, err)
		}
		instruments := make([]ITInstrument, 2)
		if err := applyMPTResampling(instruments, ext.instrumentResampling); err != nil ||
			instruments[0].mptSincResampling || !instruments[1].mptSincResampling {
			t.Fatalf("width %d: instruments %+v, %v", width, instruments, err)
		}
	}
	for _, mode := range []uint32{4} {
		if err := applyMPTResampling(make([]ITInstrument, 1), []uint32{mode}); err == nil {
			t.Errorf("accepted unsupported mode %d", mode)
		}
		instruments := make([]ITInstrument, 1)
		instruments[0].mptSincResampling = true
		instruments[0].Samples[0] = 1
		if err := validateMPTInterpolatedLoops(instruments, []SampleAsset{{LoopEnable: true, LoopPingPong: true}}); err == nil {
			t.Fatal("accepted sinc resampling with a ping-pong loop")
		}
		if err := validateMPTInterpolatedLoops(instruments, []SampleAsset{{LoopEnable: true}}); err != nil {
			t.Fatalf("rejected sinc resampling with a forward loop: %v", err)
		}
	}
	bad := append([]byte("XTPM"), mptExtensionField("...R", 3, make([]byte, 6))...)
	if _, err := parseMPTExtensions(bad, 2); err == nil {
		t.Fatal("accepted invalid resampling field width")
	}
	for _, size := range []int{0, 4, 28} {
		data := append([]byte("STPM"), mptExtensionField("CCOL", uint16(size), make([]byte, size))...)
		if _, err := parseMPTExtensions(data, 0); err != nil {
			t.Fatalf("channel colors size %d: %v", size, err)
		}
	}
	bad = append([]byte("STPM"), mptExtensionField("CCOL", 3, make([]byte, 3))...)
	if _, err := parseMPTExtensions(bad, 0); err == nil || !strings.Contains(err.Error(), "invalid size") {
		t.Fatalf("invalid channel colors: %v", err)
	}
}

func mptResamplingFixture(t *testing.T, mode byte, note byte, loop bool) []byte {
	t.Helper()
	b := oplEnvelopeEndFixture(t, 0x01280100, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}, []byte{0})
	b[30], b[44], b[50] = 8, b[44]|8, 2
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	copy(b[pattern+8:], []byte{0x81, 3, note, 2, 0})
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	data := int(binary.LittleEndian.Uint32(b[sample+72:]))
	for i := range 4096 {
		v := int16(18000 * math.Sin(2*math.Pi*float64(i)*.29))
		binary.LittleEndian.PutUint16(b[data+i*2:], uint16(v))
	}
	if loop {
		b[sample+18] |= 0x10
		binary.LittleEndian.PutUint32(b[sample+52:], 128)
		binary.LittleEndian.PutUint32(b[sample+56:], 2048)
	} else {
		b[sample+18] &^= 0x10
	}
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	properties := append([]byte("XTPM"), mptExtensionField("...R", 1, []byte{1, mode})...)
	properties = append(properties, b[bounds.propertiesStart:bounds.serializedStart]...)
	for _, f := range []struct {
		code string
		data []byte
	}{{"...C", []byte{1, 0}}, {"..MT", []byte{0}}, {".MMP", []byte{4, 0, 0, 0}},
		{".APS", []byte{64, 0, 0, 0}}, {"VTSV", []byte{64, 0, 0, 0}}} {
		properties = append(properties, mptExtensionField(f.code, uint16(len(f.data)), f.data)...)
	}
	result := append([]byte(nil), b[:bounds.propertiesStart]...)
	result = append(result, properties...)
	tail := len(result)
	result = append(result, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(result, uint32(tail))
}

func TestOpenMPTMResampling(t *testing.T) {
	for _, note := range []byte{60, 96, 108} {
		for _, loop := range []bool{false, true} {
			t.Run(fmt.Sprintf("note=%d/loop=%t", note, loop), func(t *testing.T) {
				ref, actual := mptSharedEffectRenderForDuration(t, mptResamplingFixture(t, 3, note, loop), 120)
				baseRef, baseActual := mptSharedEffectRenderForDuration(t, mptResamplingFixture(t, 1, note, loop), 120)
				refBase, actualBase := rmsWindow(baseRef, 5, 15), rmsWindow(baseActual, 5, 15)
				if refBase < .00001 || actualBase < .00001 {
					t.Fatal("silent comparison baseline")
				}
				want, got := rmsWindow(ref, 5, 15)/refBase, rmsWindow(actual, 5, 15)/actualBase
				if math.IsNaN(want) || math.IsNaN(got) || math.Abs(want-got) > .05 || math.Abs(want-1) < .1 {
					t.Fatalf("sinc/linear RMS: OpenMPT %.5f, tracker %.5f", want, got)
				}

			})
		}
	}
}

func TestOpenMPTMSincShortLoops(t *testing.T) {
	for _, mode := range []byte{1, 3} {
		t.Run(fmt.Sprintf("mode=%d", mode), func(t *testing.T) {
			fixture := func(mode byte) []byte {
				b := mptResamplingFixture(t, mode, 96, true)
				sample := int(binary.LittleEndian.Uint32(b[202:]))
				binary.LittleEndian.PutUint32(b[sample+52:], 128)
				binary.LittleEndian.PutUint32(b[sample+56:], 193)
				return b
			}

			ref, actual := mptSharedEffectRenderForDuration(t, fixture(mode), 120)
			var cross, energyRef, energyActual float64
			for i := 30 * 48 * 2; i < 80*48*2; i++ {
				r, a := float64(ref[i]), float64(actual[i])
				cross += r * a
				energyRef += r * r
				energyActual += a * a
			}
			if energyRef < .00001 || energyActual < .00001 {
				t.Fatal("silent comparison")
			}
			correlation := cross / math.Sqrt(energyRef*energyActual)
			if math.IsNaN(correlation) || correlation < .99 {
				t.Fatalf("loop PCM correlation %.5f", correlation)
			}
		})
	}
}
