package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITFilterEnvelope(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, extended := range []bool{false, true} {
			for _, resonance := range []byte{0, 80} {
				t.Run(fmt.Sprintf("legacy=%v/extended=%v/resonance=%d", legacy, extended, resonance), func(t *testing.T) {
					b := testITInstrument()
					h := b[len(b)-554:]
					h[304] = 0
					h[58], h[59] = 255, 128|resonance
					setTestEnvelope(h, 468, 0x81, []int8{-32, 32, -16}, []uint16{0, 3, 5})
					for i := range 4096 {
						value := int16(math.Sin(float64(i)*2*math.Pi*2000/48000) * 16000)
						binary.LittleEndian.PutUint16(b[512+i*2:], uint16(value))
					}
					if legacy {
						binary.LittleEndian.PutUint16(b[40:], 0x217)
						binary.LittleEndian.PutUint16(b[42:], 0x200)
					}
					if extended {
						b[45] |= 0x10
					}
					baseline := append([]byte(nil), b...)
					baseline[len(baseline)-554+468] = 0
					baseline[len(baseline)-554+59] = 0
					refBase, actualBase := referencePCMWithInterpolation(t, baseline, "2")
					ref, actual := referencePCMWithInterpolation(t, b, "2")
					for _, ms := range []int{6, 26, 46, 66, 86, 106} {
						want := channelRMS(ref, ms, ms+8, 0) / channelRMS(refBase, ms, ms+8, 0)
						got := channelRMS(actual, ms, ms+8, 0) / channelRMS(actualBase, ms, ms+8, 0)
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .035 {
							t.Errorf("%dms: OpenMPT filter ratio %.4f, tracker %.4f", ms, want, got)
						}
					}
				})
			}
		}
	}
}

func TestITFilterEnvelopeVoiceState(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	h[304], h[58], h[59] = 0, 128|80, 128|60
	setTestEnvelope(h, 468, 0x85, []int8{-32, 0, 32}, []uint16{0, 2, 4})
	h[472], h[473] = 1, 1
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}

	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	w.VoiceFilters[v].Y1, w.VoiceRightFilters[v].Y1 = .25, -.25
	w.TrackCursors[0].ITFilterCutoff = 5
	VoiceLifecycleSystem(w)
	if w.VoiceFilters[v].Y1 != .25 || w.VoiceRightFilters[v].Y1 != -.25 ||
		w.VoiceLives[v].ITFilterCutoff != 80 || w.VoiceLives[v].ITPitchEnvelopeTick != 1 {
		t.Fatalf("filter state or tick changed unexpectedly: %+v %+v", w.VoiceLives[v], w.VoiceFilters[v])
	}
	pitch := w.VoicePitches[v].BaseFreqHz
	VoiceKinematicsSystem(w)
	if math.Abs(w.VoicePitches[v].StepIncrement-pitch*float64(w.Assets.Samples[0].C5Speed)/8363/48000) > 1e-12 {
		t.Fatal("filter envelope incorrectly changed pitch")
	}
	VoiceLifecycleSystem(w)
	VoiceLifecycleSystem(w)
	if w.VoiceLives[v].ITPitchEnvelopeTick != 2 {
		t.Fatal("sustain loop did not hold")
	}
	releaseITVoice(w, v)
	VoiceLifecycleSystem(w)
	if w.VoiceLives[v].ITPitchEnvelopeTick != 3 {
		t.Fatal("released envelope did not advance")
	}
}

func TestITFilterEnvelopeStereoAndPastVoices(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	h[304], h[58], h[59] = 0, 128|80, 128|60
	setTestEnvelope(h, 468, 0x81, []int8{-32, 32}, []uint16{0, 4})
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Assets.Samples[0].RightData = append([]float32(nil), loaded.Assets.Samples[0].Data...)
	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	old := w.TrackCursors[0].ActiveVoice
	w.VoiceFilters[old].Y1, w.VoiceRightFilters[old].Y1 = .25, -.25
	w.PendingCmds[0] = ModCommand{Note: 62, Instrument: 1}
	VoiceAllocationSystem(w)
	current := w.TrackCursors[0].ActiveVoice
	if old == current || !w.VoiceLives[old].ITPastNote {
		t.Fatal("new note did not preserve the previous voice")
	}
	VoiceLifecycleSystem(w)
	if w.VoiceFilters[old].Y1 != .25 || w.VoiceRightFilters[old].Y1 != -.25 ||
		w.VoiceFilters[current].Y1 != 0 || w.VoiceRightFilters[current].Y1 != 0 {
		t.Fatal("filter history leaked across channels or voices")
	}
	if w.VoiceFilters[old].A0 != w.VoiceRightFilters[old].A0 || w.VoiceFilters[current].A0 != w.VoiceRightFilters[current].A0 {
		t.Fatal("stereo filter coefficients differ")
	}
}
