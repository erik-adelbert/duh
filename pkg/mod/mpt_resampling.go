// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import (
	"fmt"
	"math"
	"sync"
)

const mptSincPhases = 4096

var mptSincTables [3][mptSincPhases][8]float32
var mptSincOnce sync.Once

func mptImprecisePingPongTurn(pos float64, loopStart, loopEnd, diff int) float64 {
	frac := pos - math.Floor(pos)
	inverted := 1 - frac
	invInt := math.Floor(inverted)

	pos = float64(loopEnd) - (math.Floor(pos) - float64(loopEnd)) - invInt + inverted - invInt
	if int(pos) <= loopStart || int(pos) >= loopEnd {
		frac = pos - math.Floor(pos)
		pos = float64(loopEnd-min(loopEnd, diff+1)) + frac
	}

	return pos
}

func applyMPTSongResampling(instruments []ITInstrument, modes []uint32, songMode *uint32) error {
	if songMode != nil && *songMode != 0 && *songMode != 1 && *songMode != 2 && *songMode != 3 && *songMode != 5 {
		return fmt.Errorf("unsupported MPTM song resampling mode %d", *songMode)
	}

	if modes == nil {
		modes = make([]uint32, len(instruments))
		for i := range modes {
			modes[i] = 5
		}
	} else if len(modes) != len(instruments) {
		return fmt.Errorf("MPTM resampling instrument count mismatch")
	}

	for i, mode := range modes {
		if mode < 5 && mode != 0 && mode != 1 && mode != 2 && mode != 3 {
			return fmt.Errorf("MPTM instrument %d uses unsupported resampling mode %d", i+1, mode)
		}
	}

	for i, mode := range modes {
		if mode >= 5 && songMode != nil {
			mode = *songMode
		}

		instruments[i].mptNearestResampling = mode == 0
		instruments[i].mptCubicResampling = mode == 2
		instruments[i].mptSincResampling = mode == 3

		if mode == 3 {
			mptSincOnce.Do(initMPTSincTables)
		}
	}
	return nil
}

func mptCubicSample(data []float32, pos float64, loopStart, loopEnd int, looped bool) float32 {
	center := int(pos)
	phase := float32(int((pos-float64(center))*256)) / 256

	var values [4]float32

	if center > 0 && center+2 < len(data) &&
		(loopEnd <= loopStart || center+2 < loopEnd && (!looped || center-1 >= loopStart)) {
		copy(values[:], data[center-1:center+3])
	} else {
		for i := range values {
			index := center + i - 1

			if loopEnd > loopStart && (index >= loopEnd || looped && index < loopStart) {
				length := loopEnd - loopStart
				index = (index - loopStart) % length
				if index < 0 {
					index += length
				}
				index += loopStart
			}

			values[i] = data[max(0, min(index, len(data)-1))]
		}
	}
	a, b, c, d := values[0], values[1], values[2], values[3]
	return b + phase*(.5*(c-a)+phase*(a-2.5*b+2*c-.5*d+phase*(.5*(d-a)+1.5*(b-c))))
}

func validateMPTInterpolatedLoops(instruments []ITInstrument, samples []SampleAsset) error {
	for i := range instruments {
		if !instruments[i].mptSincResampling && !instruments[i].mptCubicResampling {
			continue
		}

		for _, sample := range instruments[i].Samples {
			if sample != 0 && int(sample) <= len(samples) &&
				(samples[sample-1].LoopEnable && samples[sample-1].LoopPingPong || samples[sample-1].itSustainPingPong) {
				mode := "sinc"
				if instruments[i].mptCubicResampling {
					mode = "cubic"
				}
				return fmt.Errorf("MPTM instrument %d uses %s resampling with an unsupported ping-pong loop", i+1, mode)
			}
		}
	}
	return nil
}

func mptBesselI0(x float64) float64 {
	sum, term := 1.0, 1.0

	for k := 1; ; k++ {
		term *= x * x / (4 * float64(k*k))
		sum += term
		if term < sum*1e-10 {
			return sum
		}
	}
}

func initMPTSincTables() {
	params := [3]struct{ beta, cutoff float64 }{{9.6377, .97}, {8.5, .5}, {7, .425}}

	for table, p := range params {
		normalizer := mptBesselI0(p.beta)
		for phase := range mptSincPhases {
			for tap := range 8 {
				x := float64(tap-3) - float64(phase)/mptSincPhases
				sinc := p.cutoff
				if x != 0 {
					sinc = math.Sin(math.Pi*p.cutoff*x) / (math.Pi * x)
				}

				window := mptBesselI0(p.beta*math.Sqrt(math.Max(0, 1-x*x/16))) / normalizer
				mptSincTables[table][phase][tap] = float32(sinc * window)
			}
		}
	}
}

func mptSincCoefficients(pos float64, table int) *[8]float32 {
	phase := min(int((pos-math.Floor(pos))*mptSincPhases), mptSincPhases-1)
	return &mptSincTables[table][phase]
}

func mptSincSample(data []float32, pos float64, coeff *[8]float32, loopStart, loopEnd int, pingPong, looped bool) float32 {
	center := int(pos)

	var result float32
	for tap, c := range coeff {
		index := center + tap - 3
		if loopEnd > loopStart {
			length := loopEnd - loopStart
			if index >= loopEnd || looped && index < loopStart {
				if pingPong && length > 1 {
					period := 2 * (length - 1)
					index = (index - loopStart) % period
					if index < 0 {
						index += period
					}
					if index >= length {
						index = period - index
					}
					index += loopStart
				} else {
					index = (index - loopStart) % length
					if index < 0 {
						index += length
					}
					index += loopStart
				}
			}
		}
		index = max(0, min(index, len(data)-1))
		result += c * data[index]
	}
	return result
}
