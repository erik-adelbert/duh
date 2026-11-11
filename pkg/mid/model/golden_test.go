// duh midi driver for OPL3
//
// Based on:
//   Wohlstand libADLMIDI
//   bisqwit adlmidi
//
// Uses:
//   sneakernets DMXOPL wopl definitions
//
// Original:
// Copyright (c) 2010-2014 Joel Yliluoma
// Copyright (c) 2015-2026 Vitaly Novichkov
//
// Banks:
// Copyright (c) 2017 Shannon Freeman
//
// Go implementation and modifications:
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package model

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
)

// testdata/golden.txt is produced from libADLMIDI's C models by
// testdata/golden.c; every Go model must reproduce it exactly.

var freqModels = map[string]Volume{
	"generic":  &Generic{},
	"dmx":      &Dmx{},
	"apogee":   &Apogee{},
	"win9x":    &Win9x{},
	"hmi":      &Hmi{},
	"ail":      &Ail{},
	"adlib":    &Adlib{},
	"oconnell": &OConnel{},
}

var volModels = map[string]Volume{
	"generic":     &Generic{},
	"native":      &Native{},
	"rsxx":        &Rsxx{},
	"dmx":         &Dmx{},
	"dmxfixed":    &DmxFixed{},
	"apogee":      &Apogee{},
	"apogeefixed": &ApogeeFixed{},
	"win9x":       &Win9x{},
	"sb16":        &SB16{},
	"hmiold":      &HmiOld{},
	"hmi":         &Hmi{},
	"ail":         &Ail{},
	"adlib":       &Adlib{},
	"oconnell":    &OConnel{},
}

func atoi(t *testing.T, s string) int {
	t.Helper()

	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("bad number %q", s)
	}

	return n
}

func TestGolden(t *testing.T) {
	f, err := os.Open("testdata/golden.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // can't do anything if it fails

	fails := map[string]int{}
	counts := map[string]int{}

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		w := strings.Fields(sc.Text())

		switch w[0] {
		case "F":
			name := w[1]
			tone := float64(atoi(t, w[2])) / 4
			wantOff, want := atoi(t, w[3]), atoi(t, w[4])

			var off uint32

			got := int(freqModels[name].Freq(tone, &off))

			counts["F "+name]++

			if int(off) != wantOff || got != want {
				if fails["F "+name]++; fails["F "+name] <= 3 {
					t.Errorf("Freq %s tone %v: got (%d,%#x) want (%d,%#x)", name, tone, off, got, wantOff, want)
				}
			}

		case "V":
			name := w[1]
			n := make([]int, 0, 14)

			for _, s := range w[2:] {
				n = append(n, atoi(t, s))
			}

			var v OPLModel

			v.Set(OPLVelocity, n[0])
			v.Set(OPLChanVolume, n[1])
			v.Set(OPLChanExpression, n[2])
			v.Set(OPLMasterVolume, n[3])
			v.Set(OPLVoiceMode, n[4])
			v.Set(OPLFbconn, n[5])
			v.Set(OPLModN, n[6])
			v.Set(OPLCarN, n[7])
			v.EnableMod(n[8] != 0)
			v.EnableCar(n[9] != 0)
			v.EnableDrum(n[10] != 0)

			volModels[name].Volume(&v)

			counts["V "+name]++

			gm, gc := v.Get(OPLModN)&63, v.Get(OPLCarN)&63
			if gm != n[11] || gc != n[12] {
				if fails["V "+name]++; fails["V "+name] <= 3 {
					t.Errorf("Volume %s %v: got (%d,%d) want (%d,%d)", name, n[:11], gm, gc, n[11], n[12])
				}
			}

		case "B":
			counts["B"]++

			if got, want := Brightness(atoi(t, w[1])), atoi(t, w[2]); got != want {
				if fails["B"]++; fails["B"] <= 3 {
					t.Errorf("Brightness(%s): got %d want %d", w[1], got, want)
				}
			}
		}
	}

	if sc.Err() != nil {
		t.Fatal(sc.Err())
	}

	for k, n := range fails {
		t.Errorf("%s: %d/%d mismatches", k, n, counts[k])
	}
}
