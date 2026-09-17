// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import (
	"iter"
	"strings"

	ac "github.com/erik-adelbert/duh/internal/tracker/components/voice"
)

type Variant struct {
	Mixer    string
	Sampler  string
	Index    int
	Bit16    bool
	FastMono bool
	Stereo   bool
	Filter   bool
	Linear   bool
	Spline   bool
	FIR      bool
	Ramp     bool
}

func AllVariants() iter.Seq[Variant] {
	return func(yield func(Variant) bool) {
		for _, interp := range []Interp{
			InterpNone,
			InterpLinear,
			InterpSpline,
			InterpFIR,
		} {
			for _, filter := range []Filter{false, true} {
				for _, ramp := range []Ramp{
					RampNone,
					RampSlow,
				} {
					for _, ch := range []Channels{Mono, FastMono, Stereo} {
						for _, bits := range []BitDepth{Bit8, Bit16} {
							var v Variant

							switch ch {
							case Stereo:
								v.Stereo = true
							case FastMono:
								v.FastMono = true
							}

							if bits == Bit16 {
								v.Bit16 = true
							}

							switch interp {
							case InterpLinear:
								v.Linear = true
							case InterpSpline:
								v.Spline = true
							case InterpFIR:
								v.FIR = true
							}

							if ramp != RampNone {
								v.Ramp = true
							}

							if filter {
								v.Filter = true
							}

							if v.Valid() {
								v.Mixer = v.mixerName()
								v.Sampler = v.samplerName()
								v.Index = hash(v)

								if !yield(v) {
									return
								}
							}
						}
					}
				}
			}
		}
	}
}

func (v Variant) Valid() bool {
	if v.Filter && v.FastMono {
		return false
	}

	return true
}

func (v Variant) mixerName() string {
	var b strings.Builder

	if v.Filter {
		b.WriteString("Filter")
	}

	switch {
	case v.Linear:
		b.WriteString("Linear")
	case v.Spline:
		b.WriteString("Spline")
	case v.FIR:
		b.WriteString("FIR")
	default:
		b.WriteString("Nearest")
	}

	if v.Ramp {
		b.WriteString("Ramp")
	}

	switch {
	case v.FastMono:
		b.WriteString("FastMono")
	case v.Stereo:
		b.WriteString("Stereo")
	default:
		b.WriteString("Mono")
	}

	b.WriteString("Mixer")

	if v.Bit16 {
		b.WriteString("16")
	} else {
		b.WriteString("8")
	}

	return b.String()
}

func (v Variant) samplerName() string {
	var b strings.Builder

	b.WriteString("All")

	if v.Filter {
		b.WriteString("Filter")
	}

	switch {
	case v.Linear:
		b.WriteString("Linear")
	case v.Spline:
		b.WriteString("Spline")
	case v.FIR:
		b.WriteString("FIR")
	default:
		b.WriteString("Nearest")
	}

	if v.Stereo {
		b.WriteString("Stereo")
	} else {
		b.WriteString("Mono")
	}

	b.WriteString("Samples")

	if v.Bit16 {
		b.WriteString("16")
	} else {
		b.WriteString("8")
	}

	return b.String()
}

func hash(v Variant) int {
	return ac.HashPatchID(
		v.FastMono,
		v.Stereo,
		v.Bit16,
		v.Ramp,
		v.Filter,
		v.Linear,
		v.Spline,
		v.FIR,
	)
}
