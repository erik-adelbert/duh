// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tui

import (
	"math"
	"strings"
	"time"

	"github.com/erik-adelbert/duh/internal/help"
	"github.com/erik-adelbert/duh/pkg/analyzer"
)

type VuBar struct {
	*analyzer.RMSMeter

	last time.Time
	dt   time.Duration

	levelDB float64
	hold    float64
}

func NewVuBar() VuBar {
	return VuBar{
		RMSMeter: new(analyzer.RMSMeter),
		last:     time.Now(),
	}
}

func (v *VuBar) Tick(now time.Time) {
	v.dt = now.Sub(v.last)
	v.last = now

	v.Publish()
}

func (v *VuBar) Frender(sb *strings.Builder, w int) {
	const (
		ε       = 1e-9
		rangeDB = 36.0 // dB range for the VU meter
	)

	dt := v.dt

	m := v.Value()

	db := 20 * math.Log10(m.RMS+ε)

	const atk = 20 * time.Millisecond // attack
	const rel = 50 * time.Millisecond // release

	if db > v.levelDB {
		v.levelDB = smooth(v.levelDB, db, dt.Seconds(), atk.Seconds())
	} else {
		v.levelDB = smooth(v.levelDB, db, dt.Seconds(), rel.Seconds())
	}

	level := help.Clamp((v.levelDB+rangeDB)/rangeDB, 0.0, 1.0)

	peakDB := 20 * math.Log10(m.Peak+ε)

	const peakRelease = 80 * time.Millisecond

	decay := math.Exp(-dt.Seconds() / peakRelease.Seconds())
	peakLvl := help.Clamp((peakDB+rangeDB)/rangeDB, 0.0, 1.0)

	v.hold = max(v.hold*decay, peakLvl)
	v.hold = max(v.hold, level)

	units := help.Clamp(level*float64(w), 0, float64(w))

	eighths := int(units * 8)
	full, frac := eighths/8, eighths%8

	peaki := help.Clamp(v.hold*float64(w), 0, float64(w-1))

	for i := range w {
		if i == int(peaki) {
			sb.WriteString(redPeak)

			continue
		}

		color := barColorVu(i, w)

		switch {
		case i < full:
			sb.WriteString(color("█"))
		case i == full && frac > 0:
			sb.WriteString(color(blocks[frac]))
		default:
			sb.WriteString(" ")
		}
	}

	clip := RedClip
	noclip := InvGreyClip

	if NO_COLOR {
		clip = Clip
		noclip = InvClip
	}

	if int(peaki) == w-1 {
		sb.WriteString(clip)
	} else {
		sb.WriteString(noclip)
	}
}

func smooth(current, target, seconds, tau float64) float64 {
	a := 1 - math.Exp(-seconds/tau)
	return current + (target-current)*a
}

var redPeak = Red("│")

func init() {
	if NO_COLOR {
		redPeak = "│"
	}
}

var blocks = [...]string{
	"▏",
	"▎",
	"▍",
	"▌",
	"▋",
	"▊",
	"▉",
	"█",
}

var (
	BoldRed0dB = Bold(Red("0dB"))
	Bold0dB    = Bold("0dB")
	RedClip    = Invert(Bold(Red("CLIP")))
	Clip       = Invert(Bold("CLIP"))

	InvGreyClip = InvDarkGrey("CLIP")
	InvClip     = Invert("CLIP")
)
