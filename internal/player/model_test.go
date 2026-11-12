// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package player

import (
	"reflect"
	"testing"
)

func TestSampleCallbackChannels(t *testing.T) {
	for _, tt := range []struct {
		name    string
		samples []float64
		nchan   int
		want    [][2]float64
	}{
		{"mono", []float64{0.25, -0.5}, 1, [][2]float64{{0.25, 0.25}, {-0.5, -0.5}}},
		{"stereo", []float64{0.25, -0.5, 0.75, -1}, 2, [][2]float64{{0.25, -0.5}, {0.75, -1}}},
		{"multichannel", []float64{0.25, -0.5, 1, 0.75, -1, 0}, 3, [][2]float64{{0.25, -0.5}, {0.75, -1}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var got [][2]float64
			cb := sampleCallback([]func(float64, float64){func(left, right float64) {
				got = append(got, [2]float64{left, right})
			}})
			cb(tt.samples, tt.nchan)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("samples = %v, want %v", got, tt.want)
			}
		})
	}
}
