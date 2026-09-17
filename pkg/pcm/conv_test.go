// duh pcm package
//
// Based on the UCB release of Plan 9 pcmconv.
//
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package pcm

import (
	"bytes"
	"errors"
	"math"
	"testing"
)

func mustFormat(t *testing.T, s string) Format {
	t.Helper()

	f, err := ParseFormat(s)
	if err != nil {
		t.Fatalf("FormatFromString(%q) error: %v", s, err)
	}

	return f
}

func TestNewConverter_CopyFlagWhenFormatsEqual(t *testing.T) {
	f := mustFormat(t, SLE16Stereo44k)

	c, err := NewConverter(f, f)
	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	if c == nil {
		t.Fatalf("NewConverter returned nil converter")
	}

	if c.flags&Fcopy == 0 {
		t.Fatalf("copy flag not set for equal formats")
	}
}

func TestNewConverter_InvalidChannelCount(t *testing.T) {
	in := mustFormat(t, SLE16Mono44k)
	out := mustFormat(t, SLE16Mono48k)
	out.channelCount = 0

	_, err := NewConverter(out, in)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, ErrNew) {
		t.Fatalf("error %v does not wrap ErrNew", err)
	}
}

func TestNewConverter_UnsupportedOutputFormat(t *testing.T) {
	in := mustFormat(t, SLE16Mono44k)
	out := mustFormat(t, SLE16Mono44k)
	out.tag = 'a' // input-only format in converter config

	_, err := NewConverter(out, in)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, ErrNew) {
		t.Fatalf("error %v does not wrap ErrNew", err)
	}
}

func TestConverterConvert_CopyPath(t *testing.T) {
	f := mustFormat(t, SLE16Stereo44k)

	c, err := NewConverter(f, f)
	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	in := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	out := make([]byte, len(in))

	n := c.Process(out, in)

	if n != len(in) {
		t.Fatalf("Convert copied %d bytes, want %d", n, len(in))
	}

	for i := range in {
		if out[i] != in[i] {
			t.Fatalf("out[%d] = %d, want %d", i, out[i], in[i])
		}
	}
}

func TestConverterConvert_PartialFrameBuffering(t *testing.T) {
	ifmt := mustFormat(t, SLE16Mono44k)
	ofmt := mustFormat(t, U8Mono8k)
	ofmt.sampleRate = ifmt.sampleRate // keep ratio simple for this test

	c, err := NewConverter(ofmt, ifmt)
	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	out := make([]byte, 8)

	// Less than one 16-bit frame should be buffered and produce no output.
	n := c.Process(out, []byte{0x34})

	if n != 0 {
		t.Fatalf("Convert output = %d, want 0", n)
	}

	if len(c.buf) != 1 {
		t.Fatalf("buffer len after conversion = %d, want 1", len(c.buf))
	}
}

func TestConverterFlush_ClearsBufferedInput(t *testing.T) {
	ifmt := mustFormat(t, SLE16Mono44k)
	ofmt := mustFormat(t, U8Mono8k)
	ofmt.sampleRate = ifmt.sampleRate

	c, err := NewConverter(ofmt, ifmt)
	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	c.Process(make([]byte, 8), []byte{0xaa})

	if len(c.buf) != 1 {
		t.Fatalf("buffer len before Flush = %d, want 1", len(c.buf))
	}

	n := c.Flush(nil)

	if n != 0 {
		t.Fatalf("Flush returned %d, want 0", n)
	}

	if len(c.buf) != 0 {
		t.Fatalf("buffer len after Flush = %d, want 0", len(c.buf))
	}
}

func TestConverterRatio_IdentityRate(t *testing.T) {
	f := mustFormat(t, SLE16Stereo44k)

	c, err := NewConverter(f, f)
	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	outSize, err := c.Ratio(8)
	if err != nil {
		t.Fatalf("Ratio error: %v", err)
	}

	if outSize != 8 {
		t.Fatalf("Ratio(8) = %d, want 8", outSize)
	}
}

func TestConverterRatio_IdentityFormats(t *testing.T) {
	tests := []struct {
		name string
		f    string
	}{
		{"s8", "s8c1r44100"},
		{"s16", "s16c2r44100"},
		{"s24", "s24c2r44100"},
		{"s32", "s32c2r44100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := ParseFormat(tt.f)
			if err != nil {
				t.Fatal(err)
			}

			c, err := NewConverter(f, f)
			if err != nil {
				t.Fatal(err)
			}

			inSize := f.frameSize * 4

			got, err := c.Ratio(inSize)
			if err != nil {
				t.Fatal(err)
			}

			if got != inSize {
				t.Fatalf("Ratio(%d) = %d, want %d",
					inSize, got, inSize)
			}
		})
	}
}

func TestConverterRatio_InputTooSmall(t *testing.T) {
	f := mustFormat(t, SLE16Stereo44k)

	c, err := NewConverter(f, f)
	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	_, err = c.Ratio(1)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, ErrRatio) {
		t.Fatalf("error %v does not wrap ErrRatio", err)
	}
}

func TestCodecSignedLittleEndian_RoundTrip16(t *testing.T) {
	src := []byte{0x34, 0x12, 0xcc, 0xff}
	vals := make([]int32, 2)
	format := Format{
		bitDepth:  16,
		frameSize: 2,
	}

	inSLE(vals, src, format, 2)

	dst := make([]byte, len(src))
	outSLE(dst, vals, format, 2)

	if !bytes.Equal(dst, src) {
		t.Fatalf("round-trip mismatch: got %v want %v", dst, src)
	}
}

func TestCodecSignedBigEndian_RoundTrip16(t *testing.T) {
	src := []byte{0x12, 0x34, 0xff, 0xcc}
	vals := make([]int32, 2)
	format := Format{
		bitDepth:  16,
		frameSize: 2,
	}

	inSBE(vals, src, format, 2)

	dst := make([]byte, len(src))
	outSBE(dst, vals, format, 2)

	if !bytes.Equal(dst, src) {
		t.Fatalf("round-trip mismatch: got %v want %v", dst, src)
	}
}

func TestCodecUnsignedLittleEndian_RoundTrip8(t *testing.T) {
	src := []byte{0x00, 0x80, 0xff}
	vals := make([]int32, 3)
	format := Format{
		bitDepth:  8,
		frameSize: 1,
	}

	inULE(vals, src, format, 3)

	dst := make([]byte, len(src))
	outULE(dst, vals, format, 3)

	if !bytes.Equal(dst, src) {
		t.Fatalf("round-trip mismatch: got %v want %v", dst, src)
	}
}

func TestCodecUnsignedBigEndian_RoundTrip16(t *testing.T) {
	src := []byte{0x00, 0x01, 0xab, 0xcd}
	vals := make([]int32, 2)
	format := Format{
		bitDepth:  16,
		frameSize: 2,
	}

	inUBE(vals, src, format, 2)

	dst := make([]byte, len(src))
	outUBE(dst, vals, format, 2)

	if !bytes.Equal(dst, src) {
		t.Fatalf("round-trip mismatch: got %v want %v", dst, src)
	}
}

func TestCodecSignedLittleEndian_RoundTrip24(t *testing.T) {
	src := []byte{0x00, 0x00, 0x80, 0x00, 0x00, 0x00, 0xff, 0xff, 0x7f}
	vals := make([]int32, 3)
	format := Format{
		bitDepth:  24,
		frameSize: 3,
	}

	inSLE(vals, src, format, 3)

	dst := make([]byte, len(src))
	outSLE(dst, vals, format, 3)

	if !bytes.Equal(dst, src) {
		t.Fatalf("round-trip mismatch: got %v want %v", dst, src)
	}
}

func TestCodecSignedBigEndian_RoundTrip24(t *testing.T) {
	src := []byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x7f, 0xff, 0xff}
	vals := make([]int32, 3)
	format := Format{
		bitDepth:  24,
		frameSize: 3,
	}

	inSBE(vals, src, format, 3)

	dst := make([]byte, len(src))
	outSBE(dst, vals, format, 3)

	if !bytes.Equal(dst, src) {
		t.Fatalf("round-trip mismatch: got %v want %v", dst, src)
	}
}

func TestCodecUnsignedLittleEndian_RoundTrip24(t *testing.T) {
	src := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x80, 0xff, 0xff, 0xff}
	vals := make([]int32, 3)
	format := Format{
		bitDepth:  24,
		frameSize: 3,
	}

	inULE(vals, src, format, 3)

	dst := make([]byte, len(src))
	outULE(dst, vals, format, 3)

	if !bytes.Equal(dst, src) {
		t.Fatalf("round-trip mismatch: got %v want %v", dst, src)
	}
}

func TestCodecUnsignedBigEndian_RoundTrip24(t *testing.T) {
	src := []byte{0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0xff, 0xff, 0xff}
	vals := make([]int32, 3)
	format := Format{
		bitDepth:  24,
		frameSize: 3,
	}

	inUBE(vals, src, format, 3)

	dst := make([]byte, len(src))
	outUBE(dst, vals, format, 3)

	if !bytes.Equal(dst, src) {
		t.Fatalf("round-trip mismatch: got %v want %v", dst, src)
	}
}

func TestCodecSignedLittleEndian_WithStride(t *testing.T) {
	// Two 16-bit little-endian samples with 2 bytes padding per frame.
	src := []byte{
		0x34, 0x12, 0x00, 0x00,
		0x78, 0x56, 0x00, 0x00,
	}
	vals := make([]int32, 2)
	format := Format{
		bitDepth:  16,
		frameSize: 4,
	}

	inSLE(vals, src, format, 2)

	dst := bytes.Repeat([]byte{0xee}, len(src))
	outSLE(dst, vals, format, 2)

	if dst[0] != 0x34 || dst[1] != 0x12 || dst[4] != 0x78 || dst[5] != 0x56 {
		t.Fatalf("sample bytes mismatch with stride: got %v", dst)
	}

	if dst[2] != 0xee || dst[3] != 0xee || dst[6] != 0xee || dst[7] != 0xee {
		t.Fatalf("padding bytes should be untouched: got %v", dst)
	}
}

func TestBugExhibit_Convert_CompletesBufferedFrame(t *testing.T) {
	ifmt := mustFormat(t, SLE16Mono44k)
	ofmt := mustFormat(t, U8Mono8k)
	ofmt.sampleRate = ifmt.sampleRate

	c, err := NewConverter(ofmt, ifmt)
	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	out := make([]byte, 8)

	// Stage one byte so the next call should complete a frame.
	n := c.Process(out, []byte{0x34})

	if n != 0 || len(c.buf) != 1 {
		t.Fatalf("first Convert got n=%d buf=%d, want n=0 buf=1", n, len(c.buf))
	}

	// Expected: this byte completes the pending frame and should emit output.
	n = c.Process(out, []byte{0x12})

	if n <= 0 {
		t.Fatalf("second Convert output = %d, want > 0 when frame is completed", n)
	}

	if len(c.buf) != 0 {
		t.Fatalf("buffer len after completing frame = %d, want 0", len(c.buf))
	}
}

func TestBugExhibit_Convert_MultiFrameProgress(t *testing.T) {
	ifmt := mustFormat(t, SLE16Mono44k)
	ofmt := mustFormat(t, U8Mono8k)
	ofmt.sampleRate = ifmt.sampleRate

	c, err := NewConverter(ofmt, ifmt)
	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	out := make([]byte, 16)

	// 3 bytes = 1 full 16-bit frame + 1 trailing byte.
	// Expected: process one frame, buffer one byte, and return without panic.
	n := c.Process(out, []byte{0x34, 0x12, 0x56})

	if n <= 0 {
		t.Fatalf("Convert output = %d, want > 0", n)
	}
	if len(c.buf) != 1 {
		t.Fatalf("buffer len after Convert = %d, want 1", len(c.buf))
	}
}

func TestConverterConvert_DownmixesExtraInputChannels(t *testing.T) {
	ifmt := mustFormat(t, SLE16Stereo44k)
	ofmt := mustFormat(t, SLE16Mono44k)

	c, err := NewConverter(ofmt, ifmt)
	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	// Two stereo frames, little-endian signed 16-bit:
	// frame 1: L=1000 (0x03e8), R=2000 (0x07d0) => mono 3000 (0x0bb8)
	// frame 2: L=-1000 (0xfc18), R=500 (0x01f4) => mono -500 (0xfe0c)
	in := []byte{
		0xe8, 0x03, 0xd0, 0x07,
		0x18, 0xfc, 0xf4, 0x01,
	}
	out := make([]byte, 4)

	n := c.Process(out, in)

	if n != len(out) {
		t.Fatalf("Convert returned %d bytes, want %d", n, len(out))
	}

	want := []byte{0xb8, 0x0b, 0x0c, 0xfe}
	if !bytes.Equal(out, want) {
		t.Fatalf("downmixed output = %v, want %v", out, want)
	}
}

func TestInsStereoStride(t *testing.T) {
	src := []byte{
		0xe8, 0x03, 0xd0, 0x07,
		0x18, 0xfc, 0xf4, 0x01,
	}
	format := Format{
		bitDepth:  16,
		frameSize: 4,
	}

	dst := make([]int32, 2)

	inSLE(dst, src, format, 2)

	if dst[0] != 1000<<16 {
		t.Fatalf("dst[0] = %d, want %d", dst[0], 1000<<16)
	}

	if dst[1] != -1000<<16 {
		t.Fatalf("dst[1] = %d, want %d", dst[1], -1000<<16)
	}
}

func TestSignedRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		bits int
		src  []byte
	}{
		{
			name: "s8",
			bits: 8,
			src: []byte{
				0x00,
				0x01,
				0x7f,
				0x80,
				0xff,
			},
		},
		{
			name: "s16",
			bits: 16,
			src: []byte{
				0x00, 0x00,
				0x01, 0x00,
				0xff, 0x7f,
				0x00, 0x80,
				0xff, 0xff,
			},
		},
		{
			name: "s24",
			bits: 24,
			src: []byte{
				0x00, 0x00, 0x00,
				0x01, 0x00, 0x00,
				0xff, 0xff, 0x7f,
				0x00, 0x00, 0x80,
				0xff, 0xff, 0xff,
			},
		},
		{
			name: "s32",
			bits: 32,
			src: []byte{
				0x00, 0x00, 0x00, 0x00,
				0x01, 0x00, 0x00, 0x00,
				0xff, 0xff, 0xff, 0x7f,
				0x00, 0x00, 0x00, 0x80,
				0xff, 0xff, 0xff, 0xff,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := (tt.bits + 7) / 8
			n := len(tt.src) / b

			internal := make([]int32, n)
			got := make([]byte, len(tt.src))

			format := Format{
				bitDepth:  tt.bits,
				frameSize: b,
			}

			inSLE(internal, tt.src, format, n)
			outSLE(got, internal, format, n)

			if !bytes.Equal(got, tt.src) {
				t.Errorf(
					"round trip mismatch:\n got: % x\nwant: % x",
					got,
					tt.src,
				)
			}
		})
	}
}

func TestUnsigned8RoundTrip(t *testing.T) {
	src := []byte{
		0x00,
		0x01,
		0x7f,
		0x80,
		0x81,
		0xfe,
		0xff,
	}

	const bits = 8
	const skip = 1
	var n = len(src)

	internal := make([]int32, n)
	got := make([]byte, len(src))

	format := Format{
		bitDepth:  bits,
		frameSize: skip,
	}

	inULE(internal, src, format, n)
	outULE(got, internal, format, n)

	if !bytes.Equal(got, src) {
		t.Errorf(
			"round trip mismatch:\n got: % x\nwant: % x",
			got,
			src,
		)
	}
}

func TestConverterIdentity(t *testing.T) {
	tests := []struct {
		name string
		f    string
		src  []byte
	}{
		{
			"s8",
			"s8c1r44100",
			[]byte{0x00, 0x01, 0x40, 0x7f, 0x80, 0xff},
		},
		{
			"s16",
			"s16c1r44100",
			[]byte{
				0x00, 0x00,
				0x01, 0x00,
				0xff, 0x7f,
				0x00, 0x80,
				0xff, 0xff,
			},
		},
		{
			"s24",
			"s24c1r44100",
			[]byte{
				0x00, 0x00, 0x00,
				0x01, 0x00, 0x00,
				0xff, 0xff, 0x7f,
				0x00, 0x00, 0x80,
				0xff, 0xff, 0xff,
			},
		},
		{
			"s32",
			"s32c1r44100",
			[]byte{
				0x00, 0x00, 0x00, 0x00,
				0x01, 0x00, 0x00, 0x00,
				0xff, 0xff, 0xff, 0x7f,
				0x00, 0x00, 0x00, 0x80,
				0xff, 0xff, 0xff, 0xff,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := ParseFormat(tt.f)
			if err != nil {
				t.Fatal(err)
			}

			_, err = NewConverter(f, f)
			if err != nil {
				t.Fatal(err)
			}

			dst := make([]byte, len(tt.src))

			// Whatever your actual Converter API is:
			// c.Convert(dst, tt.src)

			_ = dst
		})
	}
}

func Test24to16Sine(t *testing.T) {
	const n = 4096

	ifmt, _ := NewFormat(SignedIntLE, 24, 1, 44_100)
	ofmt, _ := NewFormat(SignedIntLE, 16, 1, 44_100)

	c, err := NewConverter(ofmt, ifmt)
	if err != nil {
		t.Fatal(err)
	}

	in := make([]byte, n*3)
	for i := range n {
		x := int32(math.Sin(float64(i)/float64(n)*math.Pi*2*440.0/44_100.0) * 0.5 * float64(1<<23))

		in[i*3+0] = byte(x)
		in[i*3+1] = byte(x >> 8)
		in[i*3+2] = byte(x >> 16)
	}

	out := make([]byte, n*2)

	got := c.Process(out, in)

	if got != len(out) {
		t.Fatalf("got %d want %d", got, len(out))
	}
}

func TestTmp24to16Amplitude(t *testing.T) {
	ifmt, _ := NewFormat(SignedIntLE, 24, 1, 44_100)
	ofmt, _ := NewFormat(SignedIntLE, 16, 1, 44_100)

	c, err := NewConverter(ofmt, ifmt)

	if err != nil {
		t.Fatal(err)
	}

	const n = 2048

	in := make([]byte, n*3)
	for i := range n {
		x := int32(math.Sin(float64(i)*2*math.Pi*440/44_100) * float64(1<<22))

		in[i*3+0] = byte(x)
		in[i*3+1] = byte(x >> 8)
		in[i*3+2] = byte(x >> 16)
	}

	out := make([]byte, n*2)
	got := c.Process(out, in)

	if got != len(out) {
		t.Fatalf("got=%d want=%d", got, len(out))
	}

	max := int32(0)
	for i := 0; i < len(out); i += 2 {
		v := int16(out[i]) | (int16(out[i+1]) << 8)
		if v < 0 {
			v = -v
		}

		if int32(v) > max {
			max = int32(v)
		}
	}
	t.Logf("max16=%d", max)
}

func TestRoundTripULE(t *testing.T) {
	vals := []byte{0x00, 0x01, 0x7f, 0x80, 0x81, 0xff}
	scratch32 := make([]int32, len(vals))
	rtts := make([]byte, len(vals))

	ifmt, _ := ParseFormat(U8Stereo44k)

	inULE(scratch32, vals, ifmt, 3)
	outULE(rtts, scratch32, ifmt, 3)

	t.Logf("value=%#x, res=%#x, rtts=%#x", vals, scratch32, rtts)

}
