// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package a440

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Frequency float64 // Frequency in Hz.

const (
	Hz      Frequency = 1
	MilliHz Frequency = 0.001 * Hz
	KiloHz  Frequency = 1000 * Hz
)

func (f Frequency) Hz() float64 {
	return float64(f)
}

func ParseFrequency(s string) (fq Frequency, err error) {
	defer func() {
		err = mkError(ErrParse, err)
	}()

	s = strings.TrimSpace(s)

	if s == "" {
		return
	}

	fq, err = parseSPN(s)

	if err == nil {
		// Successfully parsed as SPN
		return
	}

	return parseHz(s)
}

func (f *Frequency) Set(s string) (err error) {
	*f, err = ParseFrequency(s)

	err = mkError(ErrSet, err)

	return
}

func (f Frequency) String() string {
	return fmt.Sprintf("%gHz", float64(f))
}

func (f Frequency) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		fmt.Fprintf(s, "%gHz", float64(f))
	case 's':
		fmt.Fprintf(s, "%s", f.SPN())
	case 'f':
		fmt.Fprintf(s, "%.2fHz", float64(f))
	default:
		fmt.Fprintf(s, "%%!%c(Frequency=%g)", verb, float64(f))
	}
}

func (f Frequency) SPN() string {
	var scale = [...]string{
		"C", "C#", "D", "D#", "E", "F",
		"F#", "G", "G#", "A", "A#", "B",
	}

	n := int(math.Round(
		69 + 12*math.Log2(float64(f)/440),
	))

	octave := n/12 - 1
	note := scale[n%12]

	return fmt.Sprintf("%s%d", note, octave)
}

func parseHz(s string) (f Frequency, err error) {
	var f64 float64

	var units = []struct {
		suffix string
		factor Frequency
	}{
		{"kHz", KiloHz},
		{"mHz", MilliHz},
		{"Hz", Hz},
	}

	for _, u := range units {
		if n, ok := strings.CutSuffix(s, u.suffix); ok {
			n = strings.TrimSpace(n)

			f64, err = strconv.ParseFloat(n, 64)

			if err != nil {
				return
			}

			f = Frequency(f64) * u.factor
			return
		}
	}

	// No suffix! It should be interpreted in Hz
	f64, err = strconv.ParseFloat(s, 64)

	if err != nil {
		return
	}

	f = Frequency(f64)

	return
}

func parseSPN(s string) (f Frequency, err error) {
	s = strings.ToUpper(strings.TrimSpace(s))

	if s == "" {
		err = mkError(errSPN, "empty SPN string")
		return
	}

	var octave int

	var noteNames = [...]string{
		"C", "C#", "D", "D#", "E", "F",
		"F#", "G", "G#", "A", "A#", "B",
	}

	var n int

	for i, name := range noteNames {
		d, ok := strings.CutPrefix(s, name)

		if ok {
			octave, err = strconv.Atoi(d)
			if err == nil {
				n = (octave+1)*12 + i

				f = Frequency(440 * math.Pow(2, float64(n-69)/12))
				return
			}
		}
	}

	err = mkError(errSPN, fmt.Sprintf("invalid SPN string: %q", s))

	return
}
