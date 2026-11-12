// duh ADL midi driver for OPL3
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

package mid

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

func decoderTestData(t *testing.T, track []byte) []byte {
	t.Helper()
	data, err := os.ReadFile(writeTestSMF(t, track))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDecoderReadSeekAndDuration(t *testing.T) {
	data := decoderTestData(t, []byte{
		0, 0xff, 0x51, 3, 0x07, 0xa1, 0x20,
		0, 0x90, 60, 100,
		96, 0x80, 60, 0,
		0, 0xff, 0x2f, 0,
	})
	d, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if d.Format().String() != "s16c2r48000" {
		t.Fatalf("format %v", d.Format())
	}
	if d.Duration() != 1500*time.Millisecond || d.Size() != 3*PCMRate*2 {
		t.Fatalf("duration %s, size %d", d.Duration(), d.Size())
	}
	output, err := io.ReadAll(d)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(output)) != d.Size() {
		t.Fatalf("rendered %d bytes, expected %d", len(output), d.Size())
	}
	if bytes.Count(output, []byte{0}) == len(output) {
		t.Fatal("rendered audio is silent")
	}
	for _, offset := range []int64{0, 1, 999, 30000, d.Size() - 17, d.Size()} {
		if got, err := d.Seek(offset, io.SeekStart); err != nil || got != offset {
			t.Fatalf("seek %d = %d, %v", offset, got, err)
		}
		got := make([]byte, 17)
		n, err := io.ReadFull(d, got)
		want := output[offset:min(offset+17, int64(len(output)))]
		if !bytes.Equal(got[:n], want) || (len(want) < len(got) && err == nil) {
			t.Fatalf("read at %d: n=%d err=%v", offset, n, err)
		}
	}
	if err := d.Reset(); err != nil {
		t.Fatal(err)
	}
	oldBank := BankSelect
	BankSelect = uint(len(banks) - 1)
	t.Cleanup(func() { BankSelect = oldBank })
	if _, err := d.Seek(999, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if err := d.Reset(); err != nil {
		t.Fatal(err)
	}
	firstBytes := make([]byte, 137)
	if _, err := io.ReadFull(d, firstBytes); err != nil || !bytes.Equal(firstBytes, output[:len(firstBytes)]) {
		t.Fatalf("decoder reset after global bank change: %v", err)
	}
	if err := d.Reset(); err != nil {
		t.Fatal(err)
	}
	p := make([]byte, 31)
	if n, err := d.ReadAt(p, 123); err != nil || n != len(p) || !bytes.Equal(p, output[123:154]) {
		t.Fatalf("ReadAt = %d, %v", n, err)
	}
	if pos, _ := d.Seek(0, io.SeekCurrent); pos != 0 {
		t.Fatalf("ReadAt changed position to %d", pos)
	}
	if _, err := d.Seek(1, io.SeekEnd); err == nil {
		t.Fatal("seek beyond end succeeded")
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Read(p); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("read after close: %v", err)
	}
}

func TestDecoderLoopsAtMarkers(t *testing.T) {
	data := decoderTestData(t, []byte{
		0, 0xff, 0x51, 3, 0x07, 0xa1, 0x20,
		0, 0x90, 60, 100,
		0, 0xff, 0x06, 9, 'l', 'o', 'o', 'p', 'S', 't', 'a', 'r', 't',
		96, 0x80, 60, 0,
		0, 0xff, 0x06, 7, 'l', 'o', 'o', 'p', 'E', 'n', 'd',
		0, 0xff, 0x2f, 0,
	})
	d, err := Decode(bytes.NewReader(data), DecoderOptions{CardCount: 1, Loop: true})
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4*PCMRate*3)
	if n, err := io.ReadFull(d, buf); err != nil || n != len(buf) {
		t.Fatalf("loop read = %d, %v", n, err)
	}
	lastSecond := buf[2*PCMRate*4:]
	if bytes.Count(lastSecond, []byte{0}) == len(lastSecond) {
		t.Fatal("loop stopped producing audio")
	}
}

func TestDecoderTailKeepsHeldNotesAndReadChunking(t *testing.T) {
	data := decoderTestData(t, []byte{0, 0x90, 60, 100, 0, 0xff, 0x2f, 0})
	first, err := Decode(bytes.NewReader(data), DecoderOptions{CardCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(first)
	if err != nil {
		t.Fatal(err)
	}
	if first.Duration() != time.Second || len(output) != PCMRate*4 {
		t.Fatalf("tail duration %s, bytes %d", first.Duration(), len(output))
	}
	last := output[len(output)-4096:]
	if bytes.Count(last, []byte{0}) == len(last) {
		t.Fatal("held note did not continue through the tail")
	}
	second, err := Decode(bytes.NewReader(data), DecoderOptions{CardCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	var result []byte
	buf := make([]byte, 137)
	for {
		n, err := second.Read(buf)
		result = append(result, buf[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(output, result) {
		t.Fatal("output depends on read buffer size")
	}
}

func TestDecoderRejectsInvalidInputAndNormalizesOptions(t *testing.T) {
	if _, err := Decode(bytes.NewReader([]byte("bad"))); err == nil {
		t.Fatal("invalid input accepted")
	}

	data := decoderTestData(t, []byte{0, 0xff, 0x2f, 0})
	invalidDivision := bytes.Clone(data)
	invalidDivision[12], invalidDivision[13] = 0, 0
	if _, err := Decode(bytes.NewReader(invalidDivision)); err == nil {
		t.Fatal("zero MIDI time division accepted")
	}

	for _, tt := range []struct {
		options DecoderOptions
		want    DecoderOptions
	}{
		{options: DecoderOptions{}, want: DecoderOptions{CardCount: 1}},
		{options: DecoderOptions{CardCount: 0, FourOps: -1}, want: DecoderOptions{CardCount: 1}},
		{options: DecoderOptions{CardCount: 1, Bank: -1}, want: DecoderOptions{CardCount: 1}},
		{options: DecoderOptions{CardCount: 1, VolumeModel: VolModelCount}, want: DecoderOptions{CardCount: 1, VolumeModel: VolModelCount - 1}},
	} {
		d, err := Decode(bytes.NewReader(data), tt.options)
		if err != nil {
			t.Errorf("Decode(%+v): %v", tt.options, err)
			continue
		}
		if d.opts != tt.want {
			t.Errorf("Decode(%+v) normalized options to %+v, want %+v", tt.options, d.opts, tt.want)
		}
		_ = d.Close()
	}
}

func TestDecoderSupportsCMFAndIMF(t *testing.T) {
	cmf := make([]byte, 84)
	copy(cmf, "CTMF")
	binary.LittleEndian.PutUint16(cmf[6:], 62)
	binary.LittleEndian.PutUint16(cmf[8:], 78)
	binary.LittleEndian.PutUint16(cmf[12:], 96)
	binary.LittleEndian.PutUint16(cmf[58:], 1)
	cmf[62], cmf[63], cmf[72] = 0x21, 0x22, 3
	copy(cmf[78:], []byte{0, 0xff, 0x2f, 0})

	imf := make([]byte, 170)
	binary.LittleEndian.PutUint16(imf, 168)
	for i := 2; i+3 < len(imf); i += 4 {
		imf[i], imf[i+1], imf[i+2] = 0xc0, 1, 1
	}
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{"CMF", cmf}, {"IMF", imf},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d, err := Decode(bytes.NewReader(tt.data))
			if err != nil {
				t.Fatal(err)
			}
			output, err := io.ReadAll(d)
			if err != nil || int64(len(output)) != d.Size() || d.Duration() < time.Second {
				t.Fatalf("output size %d, expected %d; duration %s; error %v",
					len(output), d.Size(), d.Duration(), err)
			}
		})
	}
}
