// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package analyzer

import (
	"math"
	"sync/atomic"
)

// RMSMeterValue represents the RMS and peak values of a signal.
type RMSMeterValue struct {
	RMS  float64
	Peak float64
}

// RMSMeter calculates the RMS and peak values of a signal over time.
type RMSMeter struct {
	value atomic.Pointer[RMSMeterValue]

	sum     float64
	nsample uint32

	peak float64
}

// Addf64 adds a single float64 sample to the RMS meter.
func (m *RMSMeter) Addf64(sample float64) {
	v := math.Abs(sample)

	m.sum += v * v
	m.nsample++

	m.peak = max(m.peak, v)
}

// Publish calculates and stores the current RMS and peak values.
func (m *RMSMeter) Publish() {
	if m.nsample == 0 {
		return
	}

	rmv := new(RMSMeterValue{
		RMS:  math.Sqrt(m.sum / float64(m.nsample)),
		Peak: m.peak,
	})

	m.value.Store(rmv)

	m.sum = 0
	m.peak = 0
	m.nsample = 0
}

// Value returns the current RMS and peak values.
func (m *RMSMeter) Value() RMSMeterValue {
	rmv := m.value.Load()

	if rmv == nil {
		return RMSMeterValue{}
	}

	return *rmv
}
