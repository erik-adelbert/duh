package mod

import (
	"encoding/binary"
	"math"
	"os"
	"reflect"
	"strconv"
	"testing"
)

var percussiveOPLPatch = oplPatch{0x01, 0x02, 0x0c, 0x01, 0xf8, 0xf5, 0x1c, 0x1b, 0x03, 0, 0x0e, 0}

func TestOPLImmediateRetrigger(t *testing.T) {
	for _, rate := range []int{22050, 48000, 96000, 192000} {
		t.Run(strconv.Itoa(rate), func(t *testing.T) {
			render := func(chunk int, cancel bool) []int16 {
				s := newOPLSynth(rate)
				s.setPatch(0, percussiveOPLPatch)
				s.setPanning(0, 128)
				s.setVolume(0, 63)
				s.setFrequency(0, 440, true)
				s.render(make([]int16, rate*2))
				s.noteOff(0)
				s.setVolume(0, 0)
				s.setPatch(0, percussiveOPLPatch)
				s.setPanning(0, 128)
				s.setVolume(0, 63)
				s.setFrequency(0, 440, true)
				if cancel {
					s.noteOff(0)
					s.setVolume(0, 0)
				}
				out := make([]int16, rate/10*2)
				for pos := 0; pos < len(out); {
					n := min(chunk*2, len(out)-pos)
					s.render(out[pos : pos+n])
					pos += n
				}
				return out
			}
			whole, chunks := render(rate, false), render(1, false)
			if !reflect.DeepEqual(whole, chunks) {
				t.Fatal("retrigger changes with render chunk size")
			}
			var energy, cancelledEnergy float64
			cancelled := render(1, true)
			for i, v := range whole {
				energy += float64(v) * float64(v)
				cancelledEnergy += float64(cancelled[i]) * float64(cancelled[i])
			}
			if rms := math.Sqrt(energy / float64(len(whole))); rms < 100 {
				t.Fatalf("retrigger is silent: RMS %g", rms)
			}
			if cancelledEnergy > energy*.001 {
				t.Fatalf("cancelled key-on still sounded: energy %g vs %g", cancelledEnergy, energy)
			}
		})
	}
}

func TestOpenMPTPiguOPLRetrigger(t *testing.T) {
	path := os.Getenv("MPTECS_MPTM_TARGET")
	if path == "" {
		t.Skip("set MPTECS_MPTM_TARGET to the Triangel Island fixture")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for channel := 1; channel < 16; channel++ {
		data[64+channel] |= 128
	}
	bounds, err := locateMPTExtensions(data)
	if err != nil {
		t.Fatal(err)
	}
	ext := data[bounds.propertiesStart:bounds.serializedStart]
	index := -1
	for i := 0; i+10 <= len(ext); i++ {
		if string(ext[i:i+4]) == ".APS" && binary.LittleEndian.Uint16(ext[i+4:]) == 4 {
			index = i + 6
			break
		}
	}
	if index < 0 {
		t.Fatal("target sample preamp not found")
	}
	binary.LittleEndian.PutUint32(ext[index:], 0)
	loaded, err := loadMPTM(data)
	if err != nil {
		t.Fatal(err)
	}
	ref := renderMPTSequenceReference(t, data, "--end-time", "35")
	w := NewTracker(loaded.Assets, referenceRate, 4096)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	buf := make([]float32, 2048)
	var energy float64
	for frames := 0; frames < 35*referenceRate; {
		n := min(len(buf)/2, 35*referenceRate-frames)
		if w.Read(buf, n) != n {
			t.Fatal("short target render")
		}
		for i := max(0, 30*referenceRate-frames) * 2; i < n*2; i++ {
			energy += float64(buf[i]) * float64(buf[i])
		}
		frames += n
	}
	want := rmsWindow(ref, 30000, 35000)
	got := math.Sqrt(energy / float64(5*referenceRate*2))
	if want < .01 || math.Abs(got/want-1) > .05 {
		t.Fatalf("isolated channel RMS: tracker %g, OpenMPT %g", got, want)
	}
}
