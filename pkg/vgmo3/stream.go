// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vgmo3

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"iter"
	"slices"
	"strings"
	"time"
)

// Stream represents a VGM command stream that can be read and processed.
type Stream struct {
	infos *streamInfos

	cmds []Command
	next func() (Command, bool)
	stop func()
}

type SourceReader interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

// NewStream creates a new Stream from the given SourceReader and size.
// It reads the VGM header and prepares the command stream for processing.
// If the input data is compressed with gzip, it will be decompressed
// automatically.
func NewStream(src SourceReader, size int64) (s Stream, err error) {
	defer func() {
		err = mkError(ErrStream, err)
	}()

	src, size, err = ungzip(src, size)

	if err != nil {
		return
	}

	if size < int64(binary.Size(header{})) {
		err = mkError(errHeader, "file too small")
		return
	}

	si := new(streamInfos)

	err = bread(src, &si.header)

	if err != nil {
		err = mkError(errHeader, err) // io.EOF is untouched here

		if err == io.EOF { // no header data
			err = mkError(errHeader, "no header")
		}

		return
	}

	if string(si.Sig[:]) != "Vgm " {
		err = mkError(errHeader, "invalid signature")
		return
	}

	if si.Clock == 0 {
		err = mkError(errHeader, "not an OPL3 VGM file")
		return
	}

	if si.Loop < 0x1c || si.Loop > si.Eof {
		si.Loop = 0
		si.loop = false
	} else {
		si.Loop -= 0x1c // offset from start of file
		si.loop = true
	}

	if si.Cmd0 == 0 {
		// pre 1.50 @Data is 0x40
		si.Cmd0 = 0x40
	} else {
		si.Cmd0 += 0x34 // offset from start of file
	}

	if si.Loop < si.Cmd0 {
		si.loop = false
		si.Loop = 0
	}

	si.Loop -= si.Cmd0 // from start of command stream

	data := make([]byte, size-int64(si.Cmd0))

	_, err = src.ReadAt(data, int64(si.Cmd0))

	s.init(si, data)

	return
}

func (s *Stream) FrameCount() int64 {
	return s.infos.Length
}

func (s *Stream) Duration() time.Duration {
	return time.Duration(s.FrameCount()) * time.Second / SampleRateHz
}

// Looped returns true if the VGM stream has a loop point.
func (s *Stream) Looped() bool {
	return s.infos.loop
}

// Loop returns the loop offset in the command stream, or 0 if there is no loop.
func (s *Stream) Loop() offset {
	return s.infos.Loop
}

// Clock returns the OPL3 clock frequency in Hz, or 0 if the stream is invalid.
func (s *Stream) Clock() uint32 {
	return s.infos.Clock
}

func (s *Stream) DumpCommands() string {
	var (
		i   int
		sb  strings.Builder
		cmd Command
	)

	for i, cmd = range s.cmds {
		fmt.Fprintf(&sb, "%v\n", cmd)

		if cmd.op == OpEnd {
			break
		}
	}

	var p []byte

	off0 := cmd.addr + 1

	for i++; i < len(s.cmds); i++ {
		cmd = s.cmds[i]

		p = append(p, repack(cmd)...)
	}

	hexdump(&sb, p, off0)

	fmt.Fprintf(&sb, "0x%05x\n", s.infos.Eof-s.infos.Cmd0)

	return sb.String()
}

// String returns a string representation of the Stream.
func (s *Stream) String() string {
	return fmt.Sprintf("VGM{%v}", s.infos)
}

func (s *Stream) init(si *streamInfos, data []byte) {
	s.infos = si
	s.cmds, si.Length = getCmds(data, si.Loop, si.loop)

	cmds := s.cmds

	if len(cmds) == 0 {
		s.next, s.stop = iter.Pull(emptySeq[Command]())
		return
	}

	// seq is a closure that iterates over the commands and handles looping.
	seq := func(yield func(Command) bool) {
		var restart int

		for i := 0; i < len(cmds); i++ { // i is modified in the loop to support restart
			cmd := cmds[i]

			if !yield(cmd) {
				return
			}

			// handle loop restart
			if cmd.op == OpEnd {
				if cmd.nsample == -1 {
					return // end of stream
				}

				// command is an end command with a valid loop offset
				addr := offset(cmd.nsample)

				// lazily find the index of the command with the loop offset address
				if restart == 0 {
					restart = slices.IndexFunc(cmds, func(c Command) bool {
						return c.addr >= addr
					})
				}

				// restart at the loop offset
				if restart >= 0 {
					i = restart - 1
				}
			}
		}
	}

	s.next, s.stop = iter.Pull(seq)
}

func getCmds(raw []byte, loop offset, looped bool) (cmds []Command, n int64) {
	cmds = make([]Command, 0, len(raw)/3)

	var (
		reg     uint8
		data    byte
		nsample int

		loopLength int64
	)

	for i := 0; i < len(raw); i++ { // i is modified in the loop to support restart
		nop := false

		addr := offset(i)

		op := raw[i]

		switch op {
		case OpWrite0, OpWrite1, OpWaitN:
			if i+2 >= len(raw) {
				return
			}

			nsample = 0

			reg, data = raw[i+1], raw[i+2]

			if op == OpWaitN {
				reg, data = 0, 0

				nsample = int(ble.Uint16(raw[i+1 : i+3]))
			}

			i += 2 // Advance past reg and data
		case OpWait735, OpWait882:
			nsample = 735

			if op == OpWait882 {
				nsample = 882
			}
		case OpWaitShort:
			nsample = int(lo4(op))

		case OpEnd:
			nsample = -1 // Signal end of stream

			if looped {
				// Store loop offset
				nsample = int(loop) // /!\ usually -1 offset from the start of the stream
			}
		default:
			nop = true // Consume unsupported commands
		}

		if !nop {
			cmd := Command{
				op:      op,
				reg:     reg,
				data:    data,
				addr:    addr,
				nsample: nsample,
			}

			cmds = append(cmds, cmd)
			loopLength++ // count the command itself as part of the length

			switch op {
			case OpEnd:
				if n == 0 {
					// Store the total length of the command stream up to the
					// first end command (should be the only one though)
					n = loopLength
				}
			default:
				// Accumulate the sample count for wait commands
				loopLength += int64(nsample)
			}
		}
	}

	return
}

func hexdump(sb *strings.Builder, data []byte, baseOffset offset) {
	const ncol = 8

	for i := 0; i < len(data); i += ncol {
		end := min(i+ncol, len(data))

		fmt.Fprintf(sb, "0x%05x ", baseOffset+offset(i))

		for j := i; j < i+ncol; j++ {
			if j < len(data) {
				fmt.Fprintf(sb, "%02x ", data[j])
			} else {
				sb.WriteString("   ")
			}
		}

		sb.WriteString(" |")

		for j := i; j < end; j++ {
			b := data[j]

			if !isprint(b) {
				b = '.'
			}

			sb.WriteByte(b)
		}

		fmt.Fprintln(sb, "|")
	}
}

func isprint(b byte) bool {
	return b >= 32 && b <= 126
}

func repack(c Command) (p []byte) {
	switch c.op {
	case OpWrite0, OpWrite1:
		p = []byte{c.op, c.reg, c.data}
	case OpWaitN:
		p = []byte{c.op, lo8(c.nsample), hi8(c.nsample)}
	default:
		p = []byte{c.op}
	}

	return
}

// bread reads binary data using little-endian encoding.
func bread(r io.Reader, data any) error {
	return binary.Read(r, ble, data)
}

// ungzip checks if the input data is compressed with gzip and decompresses it if necessary.
// the maximum uncompressed size is limited to 9 MB.
func ungzip(r SourceReader, n int64) (sr SourceReader, size int64, err error) {
	defer func() {
		err = mkError(errUngzip, err)
	}()

	const maxUnpackedSize = 9 * MB

	sr, size = r, n

	br := bufio.NewReader(r)

	magic, err := br.Peek(2)

	if err != nil {
		return
	}

	// gzip magic: 1F 8B
	if magic[0] == 0x1f && magic[1] == 0x8b {
		var gz *gzip.Reader

		gz, err = gzip.NewReader(br)

		if err != nil {
			return
		}

		var unzipped []byte

		unzipped, err = io.ReadAll(
			io.LimitReader(gz, maxUnpackedSize), // max 9 MB
		)

		if err != nil {
			return
		}

		sr = bytes.NewReader(unzipped)

		size = int64(len(unzipped))

		return
	}

	return
}

// emptySeq returns an empty iterator of the specified type.
func emptySeq[T any]() iter.Seq[T] {
	return func(yield func(T) bool) {}
}

// 1 MB in bytes
const MB = 1024 * 1024
