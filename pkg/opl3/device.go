// duh OPL3 emulator package
//
// Based on Nuked OPL3 by Nuke.YKT.
//
// Original:
// Copyright (C) 2013-2020 Nuke.YKT
// Copyright (C) 2026 Tony Gies (Nuked-OPL3-fast modifications)
//
// Go implementation and modifications:
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package opl3

import (
	"math"
	"sync/atomic"
	"time"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

type Device struct {
	Counter

	sampleRate int

	chip *Chip

	regs Registers

	state atomic.Pointer[State]

	addrs [2]uint8

	timers [2]*timer

	sampleFrac float64

	μsAcc time.Duration
}

const (
	Bank0 = iota
	Bank1
)

func NewDevice(sampleRate int) *Device {
	return NewDeviceWithClock(sampleRate, YMF262ClockHz)
}

func NewDeviceWithClock(sampleRate, clockHz int) *Device {
	return new(Device{
		sampleRate: sampleRate,

		timers: [2]*timer{
			newTimer(timerRate0),
			newTimer(timerRate1),
		},

		chip: NewChipWithClock(sampleRate, clockHz),
	})
}

func (d *Device) Format() pcm.Format {
	f, _ := pcm.ParseFormat(pcm.OPL3)
	return f
}

func (d *Device) SetChannelCallback(cb ChannelCallback) {
	d.chip.SetChannelCallback(cb)
}

// Run runs the device for the given duration in microseconds.
func (d *Device) Run(δt time.Duration) (sampleCount int) {
	if δt <= 0 {
		return 0
	}

	// Tick the timers for every device tick rate that has passed.
	d.μsAcc += δt
	for d.μsAcc >= tickRate0 {

		d.timers[0].tick(tickRate0)
		d.timers[1].tick(tickRate1)

		d.μsAcc -= tickRate0
	}

	t := d.μsAcc.Seconds()

	// Calculate the number of samples generated and accumulate the fractional.
	samples := t*float64(d.sampleRate) + d.sampleFrac
	d.sampleFrac = samples - math.Floor(samples)

	return int(samples)
}

func (d *Device) RunN(sampleCount int) int {
	n, sr := sampleCount, d.sampleRate

	if n <= 0 {
		return 0
	}

	// Calculate the duration for the given number of samples.
	δt := time.Duration(
		float64(n) / float64(sr) * float64(time.Second),
	)

	return d.Run(δt)
}

func (d *Device) Resample2Ch(out []int16) {
	d.chip.Resample2Ch(out)

	d.Snapshot()
	d.SampleCount++
}

func (d *Device) Stream2Ch(out []int16, size int) {
	d.chip.Stream2Ch(out, size)
}

func (d *Device) WriteAddress(bank, addr uint8) (err error) {
	defer func() {
		err = mkError(ErrDevice, err)
	}()

	if bank > Bank1 {
		return mkError(errWriteAddress, "invalid bank")
	}

	d.AddrWrite++

	d.addrs[bank] = addr

	return
}

func (d *Device) WriteData(bank, data uint8) (err error) {
	defer func() {
		err = mkError(ErrDevice, err)
	}()

	if bank > Bank1 {
		return mkError(errWriteData, "invalid bank")
	}

	d.DataWrite++

	d.regs[bank][d.addrs[bank]] = data

	d.WriteRegister(bank, d.addrs[bank], data, false)

	return
}

func (d *Device) WriteRegister(bank, reg, data uint8, buffered bool) {
	if bank > Bank1 {
		return
	}

	d.regs[bank][reg] = data

	// Handle timer registers
	if bank == Bank0 {
		switch reg {
		case timer0:
			d.timers[0].reset(data)
		case timer1:
			d.timers[1].reset(data)
		case timerCtrl:
			if data&timerIRQ != 0 {
				d.timers[0].stop()
				d.timers[1].stop()
			} else {
				d.timers[0].mask(has(data, timerMask0))
				d.timers[0].enable(has(data, timerStart0))

				d.timers[1].mask(has(data, timerMask1))
				d.timers[1].enable(has(data, timerStart1))
			}
		}
	}

	d.RegisterWrite++

	reg16 := uint16(bank)<<8 | uint16(reg)

	if buffered {
		d.chip.QueueReg(reg16, data)
	} else {
		d.chip.WriteReg(reg16, data)
	}
}

func (d *Device) Reset(sampleRate int) (err error) {
	return d.ResetWithClock(sampleRate, YMF262ClockHz)
}

func (d *Device) ResetWithClock(sampleRate, clockHz int) (err error) {
	defer func() {
		err = mkError(ErrDevice, err)
	}()

	if sampleRate <= 0 {
		return mkError(errSampleRate, "sample rate must be positive")
	}

	d.sampleRate = sampleRate

	d.chip.ResetWithClock(d.sampleRate, clockHz)

	clear(d.regs[:])
	d.Counter = Counter{}

	return nil
}

func (d *Device) SampleRate() int {
	return d.sampleRate
}

func (d *Device) Snapshot() {
	cur := d.state.Swap(nil)

	if cur == nil {
		cur = getState()
	}

	cur.TimerStatus = d.timerStatus()
	cur.Registers = d.regs

	d.state.Store(cur)
}

func (d *Device) State() (State, bool) {
	var zero State

	cur := d.state.Swap(nil)

	if cur != nil {
		defer cur.Free()

		return *cur, true
	}

	return zero, false
}

func (d *Device) timerStatus() (status uint8) {
	ok0 := d.timers[0].isElapsed()
	ok1 := d.timers[1].isElapsed()

	if ok0 {
		status |= timerMask0 | timerIRQ
	}

	if ok1 {
		status |= timerMask1 | timerIRQ
	}

	return
}

type Counter struct {
	DataWrite     uint64
	AddrWrite     uint64
	RegisterWrite uint64
	SampleCount   uint64
}

func (dc *Counter) Counter() Counter {
	return *dc
}

func (dc *Counter) ResetCounter() {
	*dc = Counter{}
}

const (
	tickRate0 = 80 * μs
	tickRate1 = 320 * μs

	timerMask0 = 0b0100_0000
	timerMask1 = 0b0010_0000
	timerIRQ   = 0b1000_0000

	timer0    = 0x02
	timer1    = 0x03
	timerCtrl = 0x04

	timerRate0 = 80 * μs
	timerRate1 = 320 * μs

	timerStart0 = 0b0000_0001
	timerStart1 = 0b0000_0010
)

const μs = time.Microsecond
