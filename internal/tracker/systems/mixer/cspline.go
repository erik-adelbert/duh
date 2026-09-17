package mixer

import (
	"math"

	"github.com/erik-adelbert/duh/internal/help"
)

var csLUT [4 * csLUTSize]int16

func init() {
	fraqStep := 1.0 / float64(csLUTSize)
	quantScale := float64(csQuantScale)

	for i := range csLUTSize {
		x := float64(i) * fraqStep
		j := i << 2

		// Catmull–Rom spline coefficients
		cm1 := math.Round(quantScale * (-0.5*x*x*x + 1.0*x*x - 0.5*x))
		c0 := math.Round(quantScale * (+1.5*x*x*x - 2.5*x*x + 1.0))
		c1 := math.Round(quantScale * (-1.5*x*x*x + 2.0*x*x + 0.5*x))
		c2 := math.Round(quantScale * (+0.5*x*x*x - 0.5*x*x))

		csLUT[j] = clamp16(cm1)
		csLUT[j+1] = clamp16(c0)
		csLUT[j+2] = clamp16(c1)
		csLUT[j+3] = clamp16(c2)

		var sum int32

		for k := range 4 {
			sum += int32(csLUT[j+k])
		}

		if sum != csQuantScale {
			jmax := j

			for k := 1; k < 4; k++ {
				if abs32(csLUT[j+k]) > abs32(csLUT[jmax]) {
					jmax = j + k
				}
			}

			corr := csQuantScale - sum
			v := int32(csLUT[jmax]) + corr
			csLUT[jmax] = clamp16(v)
		}
	}
}

const (
	csQuantBits  = 14
	csQuantScale = (1 << csQuantBits)
	csShift8     = (csQuantBits - 8)
	csShift16    = csQuantBits
	csFracBits   = 10
	csLUTSize    = (1 << csFracBits)
)

type clampy interface{ float64 | int32 }

func clamp16[T clampy](x T) int16 {
	return int16(help.Clamp(x, math.MinInt16, math.MaxInt16))
}

func abs32(x int16) int32 {
	if x < 0 {
		return -int32(x)
	}

	return int32(x)
}
