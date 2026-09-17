// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backend

import "github.com/erik-adelbert/duh/pkg/analyzer"

type MeterValues struct {
	left  analyzer.RMSMeterValue
	right analyzer.RMSMeterValue
}

func (m *MeterValues) Set(left, right analyzer.RMSMeterValue) {
	m.left = left
	m.right = right
}

func (m MeterValues) Left() analyzer.RMSMeterValue {
	return m.left
}

func (m MeterValues) Right() analyzer.RMSMeterValue {
	return m.right
}

type SpectrumValue = analyzer.SpectrumValue
