package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func mptSongResamplingFixture(t *testing.T, song, instrument byte, loop bool) []byte {
	t.Helper()
	b := mptResamplingFixture(t, instrument, 96, loop)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	data := append([]byte(nil), b[:bounds.serializedStart]...)
	data = append(data, mptExtensionField("RSMP", 4, []byte{song, 0, 0, 0})...)
	tail := len(data)
	data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(data, uint32(tail))
}

func TestMPTSongResampling(t *testing.T) {
	for _, tt := range []struct {
		song, instrument byte
		nearest, sinc    bool
	}{
		{0, 5, true, false}, {1, 5, false, false}, {3, 5, false, true}, {5, 5, false, false},
		{0, 1, false, false}, {0, 3, false, true}, {3, 0, true, false},
	} {
		t.Run(fmt.Sprintf("song=%d/instrument=%d", tt.song, tt.instrument), func(t *testing.T) {
			loaded, err := loadMPTM(mptSongResamplingFixture(t, tt.song, tt.instrument, false))
			if err != nil {
				t.Fatal(err)
			}
			inst := loaded.Assets.ITInstruments[1]
			if inst.mptNearestResampling != tt.nearest || inst.mptSincResampling != tt.sinc {
				t.Fatalf("nearest=%t sinc=%t", inst.mptNearestResampling, inst.mptSincResampling)
			}
		})
	}
	cubic, err := loadMPTM(mptSongResamplingFixture(t, 2, 5, false))
	if err != nil {
		t.Fatal(err)
	}
	if !cubic.Assets.mptCubicResampling || !cubic.Assets.ITInstruments[1].mptCubicResampling {
		t.Fatal("cubic song resampling was not applied")
	}
	for _, mode := range []byte{4, 255} {
		if _, err := loadMPTM(mptSongResamplingFixture(t, mode, 5, false)); err == nil || !strings.Contains(err.Error(), "unsupported MPTM song resampling") {
			t.Fatalf("song mode %d: %v", mode, err)
		}
	}
	for _, size := range []uint16{1, 2, 3, 8} {
		data := append([]byte("STPM"), mptExtensionField("RSMP", size, make([]byte, size))...)
		if _, err := parseMPTExtensions(data, 0); err == nil || !strings.Contains(err.Error(), "invalid size") {
			t.Fatalf("width %d: %v", size, err)
		}
	}
	b := mptSongResamplingFixture(t, 3, 5, true)
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	b[sample+18] |= 0x40
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "sinc resampling") {
		t.Fatalf("sinc ping-pong: %v", err)
	}
}

func TestOpenMPTMSongResampling(t *testing.T) {
	for _, tt := range []struct {
		song, instrument byte
		loop             bool
	}{
		{0, 5, false}, {0, 5, true}, {0, 1, false}, {1, 0, true}, {3, 5, true},
		{2, 5, false}, {2, 5, true}, {1, 2, false}, {1, 2, true},
	} {
		t.Run(fmt.Sprintf("song=%d/instrument=%d/loop=%t", tt.song, tt.instrument, tt.loop), func(t *testing.T) {
			ref, actual := mptSharedEffectRenderForDuration(t, mptSongResamplingFixture(t, tt.song, tt.instrument, tt.loop), 100)
			var cross, r2, a2 float64
			for i := 10 * 96; i < 50*96; i++ {
				r, a := float64(ref[i]), float64(actual[i])
				cross, r2, a2 = cross+r*a, r2+r*r, a2+a*a
			}

			correlation := cross / math.Sqrt(r2*a2)
			if r2 < .001 || a2 < .001 || math.IsNaN(correlation) || correlation < .98 {
				t.Fatalf("correlation %g energy %g/%g", correlation, r2, a2)
			}
		})
	}
	refNearest, actualNearest := mptSharedEffectRenderForDuration(t, mptSongResamplingFixture(t, 0, 5, true), 100)
	refLinear, actualLinear := mptSharedEffectRenderForDuration(t, mptSongResamplingFixture(t, 1, 5, true), 100)
	for name, pair := range map[string][2][]float32{"OpenMPT": {refNearest, refLinear}, "tracker": {actualNearest, actualLinear}} {
		var diff float64
		for i := 10 * 96; i < 50*96; i++ {
			d := float64(pair[0][i] - pair[1][i])
			diff += d * d
		}
		if diff < .01 {
			t.Fatalf("%s nearest and linear output did not differ: %g", name, diff)
		}
	}
}
