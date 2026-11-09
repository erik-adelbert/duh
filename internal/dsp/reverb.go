package dsp

import (
	"math"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Delay: a plain circular delay line.
// Step writes one sample and returns the sample from `len(buf)` steps ago.
// ---------------------------------------------------------------------------

type Delay struct {
	buf []float32
	pos int
}

func (d *Delay) Create(size int) {
	if size < 1 {
		size = 1
	}
	d.buf = make([]float32, size)
	d.pos = 0
}

// Step writes in and returns the delayed sample (read-before-write, so the
// delay is exactly len(buf) samples).
func (d *Delay) Step(in float32) float32 {
	if len(d.buf) == 0 {
		return in
	}

	out := d.buf[d.pos]
	d.buf[d.pos] = in
	d.pos++

	if d.pos == len(d.buf) {
		d.pos = 0
	}

	return out
}

// Process fills dst with delayed copies of src (src and dst may alias).
func (d *Delay) Process(src, dst []float32) {
	for i := range dst {
		dst[i] = d.Step(src[i])
	}
}

// ---------------------------------------------------------------------------
// Filter: a single delay line with an HF-damping one-pole state.
// ---------------------------------------------------------------------------

type Filter struct {
	Ptr   []float32
	Pos   int
	Store float32
}

func (f *Filter) Create(size int) {
	if size < 1 {
		size = 1
	}
	f.Ptr = make([]float32, size)
	f.Pos = 0
	f.Store = 0
}

// update writes a into the buffer at Pos, advances Pos (backwards), returns b.
func (f *Filter) update(a, b float32) float32 {
	f.Ptr[f.Pos] = a
	if f.Pos == 0 {
		f.Pos = len(f.Ptr) - 1
	} else {
		f.Pos--
	}
	return b
}

func (f *Filter) ProcessComb(input, feedback, hfDamping float32) float32 {
	f.Store = f.Ptr[f.Pos] + (f.Store-f.Ptr[f.Pos])*hfDamping
	return f.update(input+feedback*f.Store, f.Ptr[f.Pos])
}

func (f *Filter) ProcessAllPass(input float32) float32 {
	return f.update(input+f.Ptr[f.Pos]*0.5, f.Ptr[f.Pos]-input)
}

// ---------------------------------------------------------------------------
// FilterArray: one channel — 8 combs in parallel, 4 allpasses in series.
// ---------------------------------------------------------------------------

type FilterArray struct {
	Comb    [8]Filter
	Allpass [4]Filter
}

// Filter delay lengths in samples at 44100 Hz (from Freeverb / SoX).
var combLengths = [8]int{1116, 1188, 1277, 1356, 1422, 1491, 1557, 1617}
var allpassLengths = [4]int{225, 341, 441, 556}

func (fa *FilterArray) Create(rate, scale, offset float64) {
	const stereoAdjust = 12

	r := rate / 44100.0

	// Use offset as a continuous value (not truncated to int before scaling)
	// so fractional stereo depth actually spreads the channels.
	off := offset
	for i := range combLengths {
		lenSamples := float64(combLengths[i]) + float64(stereoAdjust)*off
		if lenSamples < 1 {
			lenSamples = 1
		}
		size := int(scale*r*lenSamples + 0.5)
		fa.Comb[i].Create(size)
		off = -off
	}

	off = offset
	for i := range allpassLengths {
		lenSamples := float64(allpassLengths[i]) + float64(stereoAdjust)*off
		if lenSamples < 1 {
			lenSamples = 1
		}
		size := int(r*lenSamples + 0.5)
		fa.Allpass[i].Create(size)
		off = -off
	}
}

func (fa *FilterArray) Process(
	input []float32,
	output []float32,
	feedback, hfDamping, gain float32,
) {
	for i := range input {
		var out float32
		in := input[i]

		// Combs in parallel: order doesn't matter for addition.
		for j := range fa.Comb {
			out += fa.Comb[j].ProcessComb(in, feedback, hfDamping)
		}

		// Allpasses in series: order matters, keep forward index order.
		for j := range fa.Allpass {
			out = fa.Allpass[j].ProcessAllPass(out)
		}

		output[i] = out * gain
	}
}

// ---------------------------------------------------------------------------
// Reverb: top-level stereo reverb with a true circular pre-delay buffer.
// ---------------------------------------------------------------------------

type Reverb struct {
	Feedback  float32
	HfDamping float32
	Gain      float32

	Chan [2]FilterArray
	Out  [2][]float32

	preDelay Delay

	// Reusable scratch so Process doesn't allocate.
	work []float32
}

func (r *Reverb) Create(
	sampleRate float64,
	wetGainDB, roomScale, reverberance, hfDamping float32,
	preDelay, stereoDepth float64,
	bufferSize int,
) {
	scale := float64(roomScale)*0.9 + 0.1

	a := -1.0 / math.Log(1-0.3)
	b := 100.0 / (math.Log(1-0.98)*a + 1.0)

	r.Feedback = float32(1 - math.Exp((float64(reverberance)*100.0-b)/(a*b)))
	r.HfDamping = hfDamping*0.3 + 0.2
	r.Gain = float32(math.Exp(float64(wetGainDB)*(math.Log(10.0)*0.05)) * 0.015)

	delaySamples := int(preDelay*sampleRate + 0.5)
	r.preDelay.Create(delaySamples)

	// Number of channels actually supported by the Chan array.
	nchan := clamp(int(math.Ceil(stereoDepth)+1), 2, len(r.Chan))

	for i := range nchan {
		r.Chan[i].Create(sampleRate, scale, float64(i)*stereoDepth)
		r.Out[i] = make([]float32, bufferSize)
	}

	r.work = make([]float32, bufferSize)
}

func clamp(n, min, max int) int {
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

// Process consumes `length` samples from input and fills r.Out.
func (r *Reverb) Process(input []float32) {
	length := len(input)

	if length > len(r.work) {
		r.work = make([]float32, length)
	}
	work := r.work[:length]

	r.preDelay.Process(input[:length], work)

	for i := range r.Out {
		if r.Out[i] == nil {
			continue
		}

		if length > len(r.Out[i]) {
			r.Out[i] = make([]float32, length)
		}

		r.Chan[i].Process(work, r.Out[i][:length], r.Feedback, r.HfDamping, r.Gain)
	}
}

// ---------------------------------------------------------------------------
// Specs: the 7 reverb parameters.
// ---------------------------------------------------------------------------

type Specs struct {
	DoReverb     float32 // 0 or 1
	WetGainDB    float32 // -10..10
	RoomScale    float32 // 0..1
	Reverberance float32 // 0..1
	HfDamping    float32 // 0..1
	PreDelayS    float32 // 0..0.5
	StereoDepth  float32 // 0..1
}

func DefaultSpecs() Specs {
	return Specs{
		DoReverb:     1,
		WetGainDB:    6,
		RoomScale:    0.7,
		Reverberance: 0.6,
		HfDamping:    0.8,
		PreDelayS:    0,
		StereoDepth:  1,
	}
}

func (s Specs) asArray() [7]float32 {
	return [7]float32{
		s.DoReverb,
		s.WetGainDB,
		s.RoomScale,
		s.Reverberance,
		s.HfDamping,
		s.PreDelayS,
		s.StereoDepth,
	}
}

func (s *Specs) fromArray(a [7]float32) {
	s.DoReverb = a[0]
	s.WetGainDB = a[1]
	s.RoomScale = a[2]
	s.Reverberance = a[3]
	s.HfDamping = a[4]
	s.PreDelayS = a[5]
	s.StereoDepth = a[6]
}

// ---------------------------------------------------------------------------
// ParseReverb: "room=0.8:delay=0.05:wet=4" style config strings.
// ---------------------------------------------------------------------------

var alias = map[string]int{
	"no": 0, "none": 0, "off": 0, "false": 0,

	"gain": 1, "wet_gain": 1, "wetgain": 1, "wet": 1, "wet-gain": 1, "g": 1,

	"room_scale": 2, "scale": 2, "room-scale": 2, "roomscale": 2,
	"room": 2, "r": 2,

	"reverberance": 3, "reverb": 3, "amount": 3, "factor": 3, "f": 3,

	"hf": 4, "hf_damping": 4, "hf-damping": 4, "damping": 4,
	"dampening": 4, "damp": 4, "d": 4,

	"delay": 5, "pre_delay": 5, "pre-delay": 5, "predelay": 5,
	"wait": 5, "seconds": 5, "w": 5,

	"stereo": 6, "depth": 6, "stereo_depth": 6, "stereo-depth": 6,
	"stereodepth": 6, "width": 6, "wide": 6, "s": 6,
}

func ParseReverb(specs string, into *Specs) {
	arr := into.asArray()

	for term := range strings.SplitSeq(specs, ":") {
		if term == "" {
			continue
		}

		key, value, _ := strings.Cut(term, "=")

		var fv float32
		if value != "" {
			if v, err := strconv.ParseFloat(value, 32); err == nil {
				fv = float32(v)
			}
		}

		if idx, ok := alias[key]; ok {
			arr[idx] = fv
		}
	}

	into.fromArray(arr)
}

// ---------------------------------------------------------------------------
// ReverbData: wrapper matching MyReverbData in the C++ file.
// ---------------------------------------------------------------------------

type ReverbData struct {
	WetOnly float32
	Chan    [2]Reverb
	Specs   Specs
}

func NewReverbData(specs Specs, sampleRate float64, bufferSize int) *ReverbData {
	d := &ReverbData{Specs: specs}
	d.ReInit(sampleRate, bufferSize)
	return d
}

func (d *ReverbData) ReInit(sampleRate float64, bufferSize int) {
	d.WetOnly = d.Specs.DoReverb

	for i := range 2 {
		d.Chan[i].Create(
			sampleRate,
			d.Specs.WetGainDB,
			d.Specs.RoomScale,
			d.Specs.Reverberance,
			d.Specs.HfDamping,
			float64(d.Specs.PreDelayS),
			float64(d.Specs.StereoDepth),
			bufferSize,
		)
	}
}
