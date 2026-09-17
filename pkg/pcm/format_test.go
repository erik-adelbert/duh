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
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestFormatDecodef64Stereo(t *testing.T) {
	fmt := DefaultFormat()
	buf32 := make([]int32, 4)
	out := make([]float64, 4)
	in := []byte{
		0x00, 0x40, 0x00, 0xc0,
		0x00, 0x20, 0x00, 0xe0,
	}

	nsample := fmt.DecodeF64(out, in, buf32)
	if nsample != len(out) {
		t.Fatalf("Decodef64() samples = %d, want %d", nsample, len(out))
	}

	want := []float64{0.5, -0.5, 0.25, -0.25}
	for i := range want {
		if math.Abs(out[i]-want[i]) > 1e-9 {
			t.Errorf("Decodef64() sample %d = %f, want %f", i, out[i], want[i])
		}
	}
}

func TestDefaultFormat(t *testing.T) {
	got := DefaultFormat()

	if got.sampleRate != 44_100 {
		t.Fatalf("rate = %d, want 44_100", got.sampleRate)
	}

	if got.channelCount != 2 {
		t.Fatalf("channelCount = %d, want 2", got.channelCount)
	}

	if got.frameSize != 4 {
		t.Fatalf("frameSize = %d, want 4", got.frameSize)
	}

	if got.abits != 16 || got.bitDepth != 16 || got.tag != 's' {
		t.Fatalf("default format fields = %+v, want s16/16", got)
	}
}

func TestDefaultFormat_MatchesConstant(t *testing.T) {
	got := DefaultFormat()

	want, err := ParseFormat(SLE16Stereo44k)
	if err != nil {
		t.Fatalf("FormatFromString(%q) error = %v", SLE16Stereo44k, err)
	}

	if got != want {
		t.Fatalf("DefaultFormat() = %+v, want %+v", got, want)
	}
}

func TestCommonFormatStrings_Parse(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		tag    rune
		bits   int
		ch     int
		rate   int
		abits  int
		frames int64
	}{
		{name: "SLE16Mono8k", in: SLE16Mono8k, tag: 's', bits: 16, ch: 1, rate: 8000, abits: 16, frames: 2},
		{name: "SLE16Mono44k", in: SLE16Mono44k, tag: 's', bits: 16, ch: 1, rate: 44_100, abits: 16, frames: 2},
		{name: "SLE16Mono48k", in: SLE16Mono48k, tag: 's', bits: 16, ch: 1, rate: 48_000, abits: 16, frames: 2},
		{name: "SLE16Stereo44k", in: SLE16Stereo44k, tag: 's', bits: 16, ch: 2, rate: 44_100, abits: 16, frames: 4},
		{name: "SLE16Stereo48k", in: SLE16Stereo48k, tag: 's', bits: 16, ch: 2, rate: 48_000, abits: 16, frames: 4},
		{name: "ULE8Mono8k", in: U8Mono8k, tag: 'u', bits: 8, ch: 1, rate: 8000, abits: 8, frames: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseFormat(tc.in)
			if err != nil {
				t.Fatalf("FormatFromString(%q) error = %v", tc.in, err)
			}

			if got.Tag() != tc.tag {
				t.Fatalf("verb = %q, want %q", got.Tag(), tc.tag)
			}

			if got.BitDepth() != tc.bits {
				t.Fatalf("bits = %d, want %d", got.BitDepth(), tc.bits)
			}

			if got.ChannelCount() != tc.ch {
				t.Fatalf("channelCount = %d, want %d", got.ChannelCount(), tc.ch)
			}

			if got.SampleRate() != tc.rate {
				t.Fatalf("rate = %d, want %d", got.SampleRate(), tc.rate)
			}

			if got.abits != tc.abits {
				t.Fatalf("abits = %d, want %d", got.abits, tc.abits)
			}

			if got.FrameSize() != tc.frames {
				t.Fatalf("frameSize = %d, want %d", got.FrameSize(), tc.frames)
			}
		})
	}
}

func TestFormatFromString_Valid(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Format
	}{
		{
			name: "signed 16 stereo 44k",
			in:   "s16c2r44100",
			want: Format{sampleRate: 44_100, channelCount: 2, frameSize: 4, abits: 16, bitDepth: 16, tag: 's'},
		},
		{
			name: "field order can vary",
			in:   "r48000c1u8",
			want: Format{sampleRate: 48_000, channelCount: 1, frameSize: 1, abits: 8, bitDepth: 8, tag: 'u'},
		},
		{
			name: "mulaw shorthand m8",
			in:   "m8c1r8000",
			want: Format{sampleRate: 8000, channelCount: 1, frameSize: 1, abits: 16, bitDepth: 8, tag: 'μ'},
		},
		{
			name: "alaw 8bit",
			in:   "a8c1r8000",
			want: Format{sampleRate: 8000, channelCount: 1, frameSize: 1, abits: 16, bitDepth: 8, tag: 'a'},
		},
		{
			name: "float32",
			in:   "f32c2r96000",
			want: Format{sampleRate: 96_000, channelCount: 2, frameSize: 8, abits: 32, bitDepth: 32, tag: 'f'},
		},
		{
			name: "float64",
			in:   "f64c2r96000",
			want: Format{sampleRate: 96_000, channelCount: 2, frameSize: 16, abits: strconv.IntSize, bitDepth: 64, tag: 'f'},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseFormat(tc.in)
			if err != nil {
				t.Fatalf("FormatFromString(%q) error = %v", tc.in, err)
			}

			if got != tc.want {
				t.Fatalf("FormatFromString(%q) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

func TestFormatFromString_Invalid(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		wantContains string
	}{
		{name: "empty", in: "", wantContains: "empty format"},
		{name: "unsupported fmt", in: "x16c2r44100", wantContains: "unknown format character"},
		{name: "missing fields", in: "s16", wantContains: "rate and channel count must be positive"},
		{name: "bad bits", in: "s0c2r44100", wantContains: "unsupported bit depth"},
		{name: "bits too high", in: "s33c2r44100", wantContains: "unsupported bit depth"},
		{name: "mulaw not 8", in: "m16c1r8000", wantContains: "must be 8 bits"},
		{name: "alaw not 8", in: "a16c1r8000", wantContains: "must be 8 bits"},
		{name: "float invalid size", in: "f16c2r44100", wantContains: "float must be 32 or 64 bits"},
		{name: "rate zero", in: "s16c2r0", wantContains: "rate and channel count must be positive"},
		{name: "channels zero", in: "s16c0r44100", wantContains: "rate and channel count must be positive"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseFormat(tc.in)
			if err == nil {
				t.Fatalf("FormatFromString(%q) expected error, got nil", tc.in)
			}

			if !errors.Is(err, ErrFormat) {
				t.Fatalf("FormatFromString(%q) error %v does not wrap ErrPCMFormat", tc.in, err)
			}

			if !errors.Is(err, ErrPCM) {
				t.Fatalf("FormatFromString(%q) error %v does not wrap ErrPCM", tc.in, err)
			}

			if !strings.Contains(err.Error(), tc.wantContains) {
				t.Fatalf("FormatFromString(%q) error %q does not contain %q", tc.in, err.Error(), tc.wantContains)
			}
		})
	}
}

func TestPCMFormat_String(t *testing.T) {
	pf := Format{sampleRate: 22_050, channelCount: 1, frameSize: 2, abits: 16, bitDepth: 8, tag: 'μ'}

	got := pf.String()
	want := "μ8c1r22050"

	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
