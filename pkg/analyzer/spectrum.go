// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package analyzer

import (
	"fmt"
	"math"
	"math/bits"
	"os"
	"sync"
	"sync/atomic"

	fft "github.com/erik-adelbert/duh/pkg/tinyfft"
	"golang.org/x/exp/constraints"
)

// Band represents a frequency band with its lower and upper bin indices,
// the bins it covers, and its peak and level values.
type Band struct {
	lo   int
	hi   int
	bins []int

	peak  float64
	level float64
}

// NewLogBands creates logarithmically spaced frequency bands.
//   - count: the number of bands to create
//   - lowHz: the lowest frequency in Hz
//   - highHz: the highest frequency in Hz
//   - sampleRate: the sample rate of the signal
//   - fftSize: the size of the FFT
func NewLogBands(count int, lowHz, highHz float64, sampleRate, fftSize int) []Band {
	if count <= 0 || lowHz <= 0 || highHz <= lowHz ||
		sampleRate <= 0 || fftSize <= 0 {
		return nil
	}

	half := fftSize / 2
	if half < 2 {
		return nil
	}

	lolog := math.Log(lowHz)
	hilog := math.Log(highHz)

	bands := make([]Band, 0, count)

	binAt := func(index int) int {
		frac := float64(index) / float64(count)
		freq := math.Exp(lolog + (hilog-lolog)*frac)

		bin := int(math.Round(
			freq * float64(fftSize) / float64(sampleRate),
		))

		return clamp(bin, 1, half-1)
	}

	for i := range count {
		lo := binAt(i)
		hi := binAt(i+1) - 1

		hi = max(hi, lo)

		bands = append(bands, Band{lo: lo, hi: hi})
	}

	return bands
}

// SpectrumValue represents the magnitude levels and peak values
// for each frequency band in the spectrum.
type SpectrumValue struct {
	Levels []float64
	Peaks  []float64
}

// Free returns the resources associated with the SpectrumValue for reuse.
func (sv *SpectrumValue) Free() {
	svfree(sv)
}

// Spectrum represents a spectrum analyzer that performs FFT-based analysis
// on incoming float64 samples.
type Spectrum struct {
	*fft.TinyFFT

	size int
	hop  int // overlap size

	buf  *wbuf[complex128]
	win  []complex128
	hann []float64

	bands []Band
	bsdB  float64 // bin scale factor for dB normalization

	value atomic.Pointer[SpectrumValue]
}

// NewSpectrum creates a new Spectrum analyzer with the specified bands, overlap, and FFT size.
//   - bands: the frequency bands to analyze
//   - overlap: the fraction of overlap between consecutive FFT windows from 0.0 (no overlap) to 1.0
//   - size: the FFT size
func NewSpectrum(bands []Band, overlap float64, size int) (sp *Spectrum, err error) {
	var wbuf *wbuf[complex128]

	overlap = clamp(overlap, 0.0, 1.0)

	hop := int(float64(size) * (1 - overlap))
	hop = max(hop, 1)

	// Create a windowed buffer with double the size to accommodate overlapping windows.
	// sz = 2048, win = 1024, hop = 512
	wbuf, err = newbuf[complex128](2*size, size, hop)

	if err != nil {
		err = mkError(ErrNew, err)
		return
	}

	tiny, err := fft.NewTinyFFT(floorLog2(size))
	if err != nil {
		err = mkError(ErrNew, err)
		return
	}

	hann := make([]float64, size)
	for i := range hann {
		hann[i] = 0.5 - 0.5*math.Cos(
			2*math.Pi*float64(i)/float64(size-1),
		)
	}

	for i := range bands {
		bands[i].bins = make([]int, bands[i].hi-bands[i].lo+1)

		for j := range bands[i].bins {
			bands[i].bins[j] = bitReverse(bands[i].lo+j, tiny.Log2())
		}
	}

	dc := 2.0 / float64(size-1) // dc scale
	bs := 2 * dc                // bin scale

	sp = &Spectrum{
		TinyFFT: tiny,

		size: size,
		buf:  wbuf,
		hop:  hop,

		bands: bands,
		hann:  hann,
		bsdB:  bs,

		win: make([]complex128, size),
	}

	return
}

// Size returns the size of the FFT used in the spectrum analyzer.
func (sp *Spectrum) Size() int {
	return sp.size
}

// Addf64 adds a single float64 sample to the spectrum analyzer.
func (sp *Spectrum) Addf64(sample float64) {
	sp.buf.write(complex(sample, 0))

	if sp.buf.available() >= sp.TinyFFT.Size() {
		_, err := sp.buf.readWindow(sp.win)

		if err != nil {
			fmt.Printf("readWindow error: %v (avail: %d, needed: %d)\n",
				err, sp.buf.available(), sp.TinyFFT.Size())
			os.Exit(0)
			return
		}

		sp.process()
	}
}

// Value retrieves the current SpectrumValue.
// Caller must eventually call Free() on the returned SpectrumValue.
func (sp *Spectrum) Value() (sv *SpectrumValue) {
	sv = sp.value.Swap(nil)

	return sv
}

// process applies the window, performs the FFT, calculates band magnitudes,
// and publishes the results.
// It is at the heart of the hot path for spectrum analysis.
func (sp *Spectrum) process() {
	// apply the hann window
	for i := range sp.win {
		sp.win[i] *= complex(sp.hann[i], 0)
	}

	err := sp.FFT(sp.win)

	if err != nil { // hot path callback, just return
		return
	}

	// display bands
	for i := range sp.bands {
		sp.bandMagnitude(i)
	}

	sp.publish()
}

// bandMagnitude calculates the magnitude of the specified band
// and updates its level and peak.
// It is at the heart of the hot path for spectrum analysis.
func (sp *Spectrum) bandMagnitude(i int) {
	bands, win := sp.bands, sp.win

	var mag2 float64

	for _, bin := range bands[i].bins {
		z := win[bin]
		mag2 = max(mag2, real(z)*real(z)+imag(z)*imag(z))
	}

	magdB := dbNorm2(mag2, sp.bsdB*sp.bsdB)

	bands[i].level = smooth(bands[i].level, magdB)
	bands[i].peak = decay(bands[i].peak, bands[i].level)
}

func (sp *Spectrum) publish() {
	// reuse the SpectrumValue storage
	sv := sp.value.Swap(nil)

	if sv == nil {
		sv = svnew(len(sp.bands)) // none available, allocate!
	}

	for i, bd := range sp.bands {
		sv.Levels[i] = bd.level
		sv.Peaks[i] = bd.peak
	}

	sp.value.Store(sv)
}

func dbNorm2(mag2, scale float64) float64 {
	const (
		ε     = 1e-9
		minDB = -60.0
	)

	db := 10 * math.Log10(mag2*scale+ε)
	return clamp((db-minDB)/-minDB, 0.0, 1.0)
}

func smooth(level, norm float64) float64 {
	const Smoothing = 0.6

	return Smoothing*level + (1-Smoothing)*norm
}

func decay(peak, level float64) float64 {
	const PeakDecay = 0.85

	return max(peak*PeakDecay, level)
}

// SpectrumValue pool

const svHint = 64

var svPool = sync.Pool{
	New: func() any {
		return &SpectrumValue{
			Levels: make([]float64, svHint),
			Peaks:  make([]float64, svHint),
		}
	},
}

// svnew retrieves a SpectrumValue from the pool or allocates one if necessary.
func svnew(n int) *SpectrumValue {
	sv := svPool.Get().(*SpectrumValue)

	if n > cap(sv.Levels) {
		sv.Levels = make([]float64, n)
		sv.Peaks = make([]float64, n)
	}

	sv.Levels = sv.Levels[:n]
	sv.Peaks = sv.Peaks[:n]

	return sv
}

// svfree returns the SpectrumValue to the pool for reuse.
func svfree(sv *SpectrumValue) {
	if sv == nil {
		return
	}

	// Reset the slices before putting back to the pool.
	sv.Levels = sv.Levels[:0]
	sv.Peaks = sv.Peaks[:0]

	if cap(sv.Levels) > 2*svHint { // discard!
		return
	}

	svPool.Put(sv)
}

// bitReverse returns the bit-reversed value of x with the given width.
func bitReverse(x, width int) int {
	return int(bits.Reverse32(uint32(x)) >> (32 - width))
}

// floorLog2 returns the largest integer k such that 2^k <= x. If x <= 0, it returns -1.
func floorLog2(x int) int {
	if x <= 0 {
		return -1
	}

	return bits.Len(uint(x)) - 1
}

type number interface {
	constraints.Integer | constraints.Float
}

// clamp clamps the value v to the range [l, r].
func clamp[T number](v, l, r T) T {
	if v < l {
		return l
	}
	if v > r {
		return r
	}
	return v
}
