// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vgmo3

import (
	"fmt"
	"io"

	"github.com/erik-adelbert/duh/pkg/opl3"
	"github.com/erik-adelbert/duh/pkg/pcm"
)

const (
	SampleRateHz     = 44_100 // 44.1 kHz
	DefaultLoopCount = 2      // default loop count for VGM streams
)

const OPL3ChannelCount = 18

const (
	Bank0 = iota
	Bank1
)

// Decoder reads VGM data from a seekable source, synthesizes audio using the OPL3 device
// and provides the audio samples.
type Decoder struct {
	*opl3.Device
	Stream

	wait  int
	nloop int
}

func Decode(sr SourceReader, size int64) (r *Decoder, err error) {
	defer func() {
		err = mkError(ErrNew, err)
	}()

	var vgms Stream

	vgms, err = NewStream(sr, size)

	if err != nil {
		return
	}

	clockHz := int(vgms.infos.Clock)

	r = &Decoder{
		Stream: vgms,
		Device: opl3.NewDeviceWithClock(SampleRateHz, clockHz),
		nloop:  DefaultLoopCount,
	}

	return
}

// Read synthesizes and reads signed 16-bit audio samples. It is an error
// to call Read with a buffer whose length is not a multiple of 4 or to
// call Read after Close has been called. Read is not safe for concurrent
// use by multiple goroutines.
func (r *Decoder) Read(p []byte) (n int, err error) {
	defer func() {
		err = mkError(ErrRead, err)
	}()

	if len(p)%4 != 0 {
		return 0, mkError(ErrRead, "buffer length not multiple of 4")
	}

	for i := 0; i < len(p); i += 4 {
		switch {
		case r.wait > 0:
			// wait for the specified number of samples before processing
			// the next command
			r.wait--
		default:
			// get the next command from the VGM stream
			cmd, ok := r.next()

			if !ok {
				err = io.EOF
				return
			}

			if stop := r.exec(cmd); stop {
				err = io.EOF
				return
			}
		}

		var buf [2]int16

		r.Resample2Ch(buf[:])

		ble.PutUint16(p[i+0:], uint16(buf[0]))
		ble.PutUint16(p[i+2:], uint16(buf[1]))

		n += 4
	}

	return
}

// Close closes the Reader and releases any resources associated with it.
// Calling Close multiple times is safe and has no effect after the first call.
func (r *Decoder) Close() error {
	r.stop()

	return nil
}

func (r *Decoder) Format() pcm.Format {
	f, _ := pcm.ParseFormat(pcm.OPL3)
	return f
}

func (r *Decoder) Version() uint32 {
	return uint32(r.infos.Ver)
}

func (r *Decoder) LoopCount() int {
	return r.nloop
}

func (r *Decoder) String() string {
	nchan := int64(r.Format().ChannelCount())
	nsample := r.Format().SampleCount(r.Duration())
	samplesz := int64(r.Format().SampleSize())

	size := nsample * nchan * samplesz / kB

	return fmt.Sprintf("OPL3{%s %s %dkB %s}", r.Device.Format(), r.Duration(), size, r.Stream.String())
}

func (r *Decoder) SetLoopCount(n int) {
	r.nloop = n

	if r.nloop == 0 {
		r.nloop = 1
	}
}

func (r *Decoder) exec(c Command) (stop bool) {
	// fmt.Printf("VGM reader: exec cmd=%v\n", c)
	op := c.Op()

	reg, data := c.Reg(), c.Data()
	nsample := c.SampleCount()

	switch op {
	case OpWrite0:
		r.WriteRegister(Bank0, reg, data, false)
	case OpWrite1:
		r.WriteRegister(Bank1, reg, data, false)
	case OpEnd:
		if r.nloop > 0 {
			r.nloop--
		}

		if nsample == -1 || r.nloop == 0 { // end of stream
			stop = true
		}
	default: // wait commands
		r.wait += nsample
	}

	return
}

func (r *Decoder) next() (c Command, ok bool) {
	return r.Stream.next()
}

const kB = 1024
