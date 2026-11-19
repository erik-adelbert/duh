package mod

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func cueOffsetFixture(t *testing.T, volume byte, cues []uint32, itOffset, loop, oldEffects bool) []byte {
	t.Helper()
	b := mptS70Fixture(t, []byte{0x81, 7, 60, 2, volume, 0, 0, 0})
	b[50] = 6
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	pcm := int(binary.LittleEndian.Uint32(b[sample+72:]))
	for i := range 4096 {
		period := 16.0
		if i >= 512 {
			period = 28
		}
		if i >= 2048 {
			period = 42
		}
		value := int16(14000 * math.Sin(2*math.Pi*float64(i)/period))
		binary.LittleEndian.PutUint16(b[pcm+i*2:], uint16(value))
	}
	if !loop {
		b[sample+18] &^= 0x10
	}
	flags := bytesIndex(t, b, ".FSM") + 6
	if itOffset {
		b[flags+3] |= 8
	} else {
		b[flags+3] &^= 8
	}
	if oldEffects {
		b[44] |= 0x10
	}
	if cues != nil {
		bounds, err := locateMPTExtensions(b)
		if err != nil {
			t.Fatal(err)
		}
		data := append([]byte(nil), b[:bounds.serializedStart]...)
		data = append(data, mptTestCue(1, cues...)...)
		tail := len(data)
		data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
		b = binary.LittleEndian.AppendUint32(data, uint32(tail))
	}
	return b
}

func TestOpenMPTCueOffset(t *testing.T) {
	for _, tt := range []struct {
		name       string
		volume     byte
		cues       []uint32
		itOffset   bool
		loop       bool
		oldEffects bool
	}{
		{"custom first", 224, []uint32{512, 2048}, true, false, false},
		{"custom second", 225, []uint32{512, 2048}, true, false, false},
		{"default first", 224, nil, true, false, false},
		{"missing cue IT", 225, []uint32{512}, true, false, false},
		{"out of range IT loop", 224, []uint32{3000}, true, true, false},
		{"out of range legacy loop", 224, []uint32{3000}, false, true, false},
		{"out of range old effects", 224, []uint32{5000}, true, false, true},
		{"out of range old loop", 224, []uint32{3000}, true, true, true},
		{"out of range legacy old loop", 224, []uint32{3000}, false, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := cueOffsetFixture(t, tt.volume, tt.cues, tt.itOffset, tt.loop, tt.oldEffects)
			ref, actual := mptSharedEffectRenderForDuration(t, b, 100)
			if tt.oldEffects && tt.itOffset && !tt.loop {
				if rmsWindow(ref, 10, 50) > .0001 || rmsWindow(actual, 10, 50) > .0001 {
					t.Fatal("out-of-range old-effects offset should be silent")
				}
			} else {
				assertMPTPCMCorrelation(t, ref, actual, 10, 50)
			}
			assertMPTPatternDelayChunking(t, b, 4000)
		})
	}
}

func TestMPTCueOffsetValidation(t *testing.T) {
	for _, volume := range []byte{223, 233} {
		if _, err := unpackITPatternMode([]byte{0x81, 4, volume, 0}, 1, true); err == nil {
			t.Errorf("unsupported volume %d accepted", volume)
		}
	}
	if _, err := unpackITPatternMode([]byte{0x81, 4, 224, 0}, 1, false); err == nil {
		t.Error("ordinary IT cue offset accepted")
	}
	for _, tt := range []struct {
		name string
		rows []byte
		want string
	}{
		{"no note", []byte{0x81, 4, 224, 0}, "cue offset requires"},
		{"special note", []byte{0x81, 5, 255, 224, 0}, "cue offset requires"},
		{"effect", []byte{0x81, 15, 60, 2, 224, 15, 1, 0}, "cue offset requires"},
		{"same channel Oxx", []byte{0x81, 7, 60, 2, 224, 0, 0x81, 8, 15, 0, 0}, "Oxx memory"},
		{"prior Oxx", []byte{0x81, 8, 15, 1, 0, 0x81, 7, 60, 2, 224, 0}, "Oxx memory"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadMPTM(mptS70Fixture(t, tt.rows))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
	b := mptS70Fixture(t, []byte{0x81, 7, 60, 2, 224, 0x82, 8, 15, 1, 0})
	b[bytesIndex(t, b, "...C")+6] = 2
	if _, err := loadMPTM(b); err != nil {
		t.Fatalf("Oxx on a different channel: %v", err)
	}
	legacy := mptS70Fixture(t, []byte{0x81, 7, 60, 2, 224, 0})
	binary.LittleEndian.PutUint32(legacy[bytesIndex(t, legacy, "VWSL")+6:], 0x01250000)
	if _, err := loadMPTM(legacy); err == nil || !strings.Contains(err.Error(), "1.26+ profile") {
		t.Fatalf("legacy profile: %v", err)
	}
	pattern, err := unpackITPatternMode([]byte{0x81, 7, 60, 2, 225, 0, 0x81, 0x70, 0}, 2, true)
	if err != nil || !pattern.Rows[1][0].HasVolume || pattern.Rows[1][0].Volume != 225 {
		t.Fatalf("packed cue reuse: %+v, %v", pattern, err)
	}
}
