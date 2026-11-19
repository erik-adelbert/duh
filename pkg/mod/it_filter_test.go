package mod

import (
	"encoding/binary"
	"math"
	"math/cmplx"
	"testing"
)

func TestOpenMPTITInstrumentFilter(t *testing.T) {
	for _, tt := range []struct {
		name              string
		frequency         float64
		cutoff, resonance byte
		legacy, extended  bool
	}{
		{"low cutoff", 500, 128 | 20, 128, false, false},
		{"cutoff only", 500, 128 | 20, 0, false, false},
		{"resonance only", 5000, 0, 128 | 90, false, false},
		{"default bypass", 2000, 0, 0, false, false},
		{"explicit bypass", 2000, 255, 128, false, false},
		{"high frequency", 5000, 128 | 60, 128, false, false},
		{"resonant", 2000, 128 | 80, 128 | 90, false, false},
		{"extended", 5000, 128 | 80, 128 | 50, false, true},
		{"legacy", 2000, 128 | 80, 128 | 90, true, false},
		{"legacy bypass", 2000, 255, 128 | 90, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := testITInstrument()
			h := b[len(b)-554:]
			h[304] = 0
			h[58], h[59] = tt.cutoff, tt.resonance
			for i := range 4096 {
				value := int16(math.Sin(float64(i)*2*math.Pi*tt.frequency/48000) * 16000)
				binary.LittleEndian.PutUint16(b[512+i*2:], uint16(value))
			}
			if tt.legacy {
				binary.LittleEndian.PutUint16(b[40:], 0x217)
				binary.LittleEndian.PutUint16(b[42:], 0x200)
			}
			if tt.extended {
				b[45] |= 0x10
			}
			baseline := append([]byte(nil), b...)
			baseline[len(baseline)-554+58] = 0
			baseline[len(baseline)-554+59] = 0
			refBase, actualBase := referencePCM(t, baseline)
			ref, actual := referencePCM(t, b)
			refBaseRMS, actualBaseRMS := channelRMS(refBase, 30, 70, 0), channelRMS(actualBase, 30, 70, 0)
			if refBaseRMS < 1e-5 || actualBaseRMS < 1e-5 {
				t.Fatalf("silent baseline: OpenMPT %g, tracker %g", refBaseRMS, actualBaseRMS)
			}
			refRatio := channelRMS(ref, 30, 70, 0) / refBaseRMS
			actualRatio := channelRMS(actual, 30, 70, 0) / actualBaseRMS
			if math.IsNaN(refRatio) || math.IsNaN(actualRatio) || math.Abs(refRatio-actualRatio) > .02 {
				t.Errorf("filter RMS ratio: OpenMPT %.3f, tracker %.3f", refRatio, actualRatio)
			}
			phasor := func(pcm []float32) complex128 {
				var sum complex128
				for frame := 30 * 48; frame < 70*48; frame++ {
					angle := -2 * math.Pi * tt.frequency * float64(frame) / referenceRate
					sum += complex(float64(pcm[frame*2]), 0) * cmplx.Exp(complex(0, angle))
				}
				return sum
			}
			refResponse := phasor(ref) / phasor(refBase)
			actualResponse := phasor(actual) / phasor(actualBase)
			if phase := cmplx.Phase(actualResponse / refResponse); math.IsNaN(phase) || math.Abs(phase) > .05 {
				t.Errorf("filter phase difference = %.3f radians", phase)
			}
		})
	}
}

func TestITFilterCoefficients(t *testing.T) {
	tests := []struct {
		name               string
		cutoff, resonance  uint8
		instrument, legacy bool
		extended           bool
		enabled            bool
		a0, b0, b1         float64
	}{
		{"cutoff", 20, 0, true, false, false, true, .0009025053868576291, 1.9686146835206286, -.9695171889074863},
		{"resonant", 80, 90, true, false, false, true, .028948648524297568, 1.942867307807194, -.9718159563314916},
		{"extended", 80, 50, true, false, true, true, .06066958985160866, 1.7475948340047005, -.8082644238563091},
		{"legacy", 80, 90, true, true, false, true, .02815512329972586, 1.9170219219149636, -.9451770452146893},
		{"default bypass", 127, 0, true, false, false, false, 0, 0, 0},
		{"legacy bypass", 127, 90, true, true, false, false, 0, 0, 0},
		{"resonance at full cutoff", 127, 90, true, false, false, true, 0, 0, 0},
		{"sample mode", 20, 90, false, false, false, false, 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newITFilter(tt.cutoff, tt.resonance, 48000, tt.instrument, tt.legacy, tt.extended)
			if got.Enabled != tt.enabled {
				t.Fatalf("enabled = %v, want %v", got.Enabled, tt.enabled)
			}
			if tt.a0 != 0 && (math.Abs(got.A0-tt.a0) > 1e-12 || math.Abs(got.B0-tt.b0) > 1e-12 || math.Abs(got.B1-tt.b1) > 1e-12) {
				t.Fatalf("coefficients = %+v, want %g, %g, %g", got, tt.a0, tt.b0, tt.b1)
			}
		})
	}
}

func TestITInstrumentFilterMemoryAndVoiceIsolation(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	h[58], h[59] = 128|40, 128|70
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	secondInstrument := w.Assets.ITInstruments[0]
	secondInstrument.HasFilterCutoff = false
	secondInstrument.HasFilterResonance = false
	w.Assets.ITInstruments = append(w.Assets.ITInstruments, secondInstrument)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	old := w.TrackCursors[0].ActiveVoice
	if !w.VoiceFilters[old].Enabled {
		t.Fatal("filter was not enabled")
	}
	w.VoiceFilters[old].Y1 = .5
	w.PendingCmds[0] = ModCommand{Note: 62, Instrument: 2}
	VoiceAllocationSystem(w)
	next := w.TrackCursors[0].ActiveVoice
	if old == next || w.TrackCursors[0].ITFilterCutoff != 40 || w.VoiceFilters[next].Y1 != 0 || w.VoiceFilters[old].Y1 != .5 || w.VoiceFilters[next].A0 != w.VoiceFilters[old].A0 {
		t.Fatalf("filter state: old=%+v next=%+v", w.VoiceFilters[old], w.VoiceFilters[next])
	}
	w.PendingCmds[0] = ModCommand{}
	w.PendingCmds[1] = ModCommand{Note: 61, Instrument: 2}
	VoiceAllocationSystem(w)
	other := w.TrackCursors[1].ActiveVoice
	if w.VoiceFilters[other].Enabled || w.TrackCursors[1].ITFilterCutoff != 127 || w.TrackCursors[1].ITFilterResonance != 0 {
		t.Fatalf("filter settings leaked across tracks: %+v", w.TrackCursors[1])
	}
	w.Assets.ITInstruments[1].HasFilterResonance = true
	w.Assets.ITInstruments[1].FilterResonance = 0
	w.PendingCmds[1] = ModCommand{}
	w.PendingCmds[0] = ModCommand{Note: 63, Instrument: 2}
	VoiceAllocationSystem(w)
	if w.TrackCursors[0].ITFilterCutoff != 40 || w.TrackCursors[0].ITFilterResonance != 0 || !w.VoiceFilters[w.TrackCursors[0].ActiveVoice].Enabled {
		t.Fatalf("filter memory not updated independently: %+v", w.TrackCursors[0])
	}
}

func TestITFilterStateAcrossBlocks(t *testing.T) {
	sample := make([]float32, 65)
	sample[0] = 1
	assets := SongAssets{Samples: []SampleAsset{{Data: sample}}}
	makeWorld := func() *Tracker {
		w := NewTracker(assets, 48000, 64)
		w.VoiceLives[0].Active = true
		w.VoicePitches[0].StepIncrement = 1
		w.VoiceFilters[0] = newITFilter(80, 90, 48000, true, false, false)
		return w
	}
	whole, split := makeWorld(), makeWorld()
	ResamplerSystem(whole, 64)
	ResamplerSystem(split, 32)
	first := append([]float32(nil), split.VoiceSamples[0].ScratchBuf[:32]...)
	ResamplerSystem(split, 32)
	for i := range 64 {
		got := split.VoiceSamples[0].ScratchBuf[i%32]
		if i < 32 {
			got = first[i]
		}
		if got != whole.VoiceSamples[0].ScratchBuf[i] {
			t.Fatalf("frame %d: split %g, whole %g", i, got, whole.VoiceSamples[0].ScratchBuf[i])
		}
	}
	if split.VoiceFilters[0] != whole.VoiceFilters[0] {
		t.Fatal("filter history differs across block boundaries")
	}
}
