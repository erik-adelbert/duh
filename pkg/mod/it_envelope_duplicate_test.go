package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestITEnvelopeRepeatedTicks(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	setTestEnvelope(h, 468, 3, []int8{32, 0, 0, -29, -29, -5, -5, 22, 22}, []uint16{0, 0, 1, 1, 2, 2, 3, 3, 5})
	h[470], h[471] = 2, 7
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	env := &loaded.Assets.ITInstruments[0].PitchEnvelope
	for tick, want := range []float32{32, 0, -29, -5, 22, 22} {
		if got := itEnvelopeValue(env, tick); got != want {
			t.Errorf("tick %d = %g, want %g", tick, got, want)
		}
	}
	if next := itNextEnvelopeTick(env, 3, false); next != 1 {
		t.Fatalf("loop after duplicate point advanced to %d, want 1", next)
	}
	h[468+7+2*3] = 0
	h[468+8+2*3] = 0
	h[468+7+3*3] = 1
	h[468+8+3*3] = 0
	h[468+7+4*3] = 0
	h[468+8+4*3] = 0
	if _, err := loadIT(b); err == nil {
		t.Fatal("decreasing envelope ticks were accepted")
	}
}

func TestOpenMPTITRepeatedEnvelopeTicks(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, kind := range []string{"volume", "panning", "pitch", "filter"} {
			t.Run(fmt.Sprintf("legacy=%v/%s", legacy, kind), func(t *testing.T) {
				b := testITInstrument()
				b[50] = 6
				h := b[len(b)-554:]
				h[304] = 0
				offset, flags := 468, byte(1)
				values := []int8{32, -32, 0, 24, -16, 16, -24}
				switch kind {
				case "volume":
					offset = 304
					values = []int8{64, 16, 32, 60, 16, 48, 32}
				case "panning":
					offset = 386
				case "filter":
					flags = 0x81
					h[58], h[59] = 255, 128|40
				}
				setTestEnvelope(h, offset, flags, values, []uint16{0, 0, 1, 1, 2, 2, 5})
				if legacy {
					binary.LittleEndian.PutUint16(b[40:], 0x217)
					binary.LittleEndian.PutUint16(b[42:], 0x200)
				}
				ref, actual := referencePCMWithInterpolation(t, b, "2")
				if kind == "pitch" {
					for tick := range 6 {
						want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
						if math.Abs(got/want-1) > .015 {
							t.Errorf("tick %d frequency: OpenMPT %.2f, tracker %.2f", tick, want, got)
						}
					}
					return
				}
				baseline := append([]byte(nil), b...)
				baseline[len(baseline)-554+offset] = 0
				if kind == "filter" {
					baseline[len(baseline)-554+59] = 0
				}
				refBase, actualBase := referencePCMWithInterpolation(t, baseline, "2")
				for tick := range 6 {
					start := tick*20 + 6
					for channel := range 2 {
						want := channelRMS(ref, start, start+8, channel) / channelRMS(refBase, start, start+8, channel)
						got := channelRMS(actual, start, start+8, channel) / channelRMS(actualBase, start, start+8, channel)
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .07 {
							t.Errorf("tick %d channel %d: OpenMPT %.3f, tracker %.3f", tick, channel, want, got)
						}
					}
				}
			})
		}
	}
}
