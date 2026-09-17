package mixer

import (
	"math"

	"github.com/erik-adelbert/duh/internal/help"
)

var wfActive = wfBlackman
var wfLUT [wfHeight * wfWidth]int16

func init() {
	var (
		fracCount  float64 = 1 << wfFracBits
		quantScale float64 = wfQuantScale

		normOff = 1 / (2 * fracCount)

		cutoff = wfCutOff
	)

	for i := range wfHeight {
		var (
			gain  float64
			coefs [wfWidth]float64
		)

		off := (float64(i) + 0.5 - fracCount) * normOff
		base := i << wfLogWidth

		for j := range coefs {
			coefs[j] = coefAt(j, off, cutoff, wfWidth, wfActive)
			gain += coefs[j]
		}

		if math.Abs(gain) < ε {
			if gain >= 0 {
				gain = ε
			} else {
				gain = -ε
			}
		}

		gain = 1 / gain

		for j := range coefs {
			qc := math.Round(quantScale * coefs[j] * gain)
			wfLUT[base+j] = int16(help.Clamp(qc, math.MinInt16, math.MaxInt16))
		}
	}
}

func coefAt(tap int, off, cutoff float64, width int, wftype wfType) float64 {
	w := float64(width - 1)
	up := float64(tap) - off // unshifted position
	p0 := up - w/2           // center around zero

	var wf, si float64 // window function, sinc

	πΔλ := 2 * math.Pi / w // normalized angular frequency difference
	θ := πΔλ * up          // window phase

	// Compute window function
	if wf = 1.0; wftype <= wfKaiser4T {
		wf = 0.0

		for i, c := range wcoefs[wftype] {
			wf += c * math.Cos(float64(i)*θ)
		}
	}

	// Compute sinc
	p0 *= math.Pi // normalized position
	x := cutoff * p0

	if math.Abs(x) < 1e-4 { // use Taylor expansion

		si = cutoff * (1 - x*x/6)
	} else {
		si = math.Sin(x) / p0
	}

	return wf * si
}

var wcoefs = [...][4]float64{
	wfHann:         {0.500000, -0.50000, 0.00000, -0.00000},
	wfHamming:      {0.540000, -0.46000, 0.00000, -0.00000},
	wfBlackman:     {0.420000, -0.50000, 0.08000, -0.00000},
	wfBlackman3T61: {0.449590, -0.49364, 0.05677, -0.00000},
	wfBlackman3T67: {0.423230, -0.49755, 0.07922, -0.00000},
	wfBlackman4T92: {0.358750, -0.48829, 0.14128, -0.01168},
	wfBlackman4T74: {0.402217, -0.49703, 0.09392, -0.00183},
	wfKaiser4T:     {0.402430, -0.49804, 0.09831, -0.00122},
}

const (
	wfQuantBits  = 15
	wfQuantScale = 1 << wfQuantBits
	// wfShift8  = wfQuantBits - 8
	wfShift16  = wfQuantBits
	wfFracBits = 10
	wfLogWidth = 3
	wfHeight   = 1 + 1<<(wfFracBits+1)
	wfWidth    = 1 << wfLogWidth
	// wfSamplesPerSide = (wfWidth - 1) / 2
	wfCutOff = 0.9
)

type wfType int

const (
	wfHann wfType = iota
	wfHamming
	wfBlackman
	wfBlackman3T61
	wfBlackman3T67
	wfBlackman4T92
	wfBlackman4T74
	wfKaiser4T
)
