// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package aac

import (
	"fmt"

	"github.com/erik-adelbert/duh/pkg/pcm"
	"github.com/tphakala/go-aac"
	aacpcm "github.com/tphakala/go-aac/pcm"
)

// Format controls encoder output. It is a flat struct mirroring go-flac's
// pcm.Config and go-opus' oggopus.Config: every field's zero value is
// documented, so a literal with only SampleRate, BitDepth and Channels set
// is a complete, valid configuration.
type Format struct {
	aacpcm.Config
}

func ParseFormat(s string) (f Format, err error) {
	var (
		tag     pcm.Tag
		nbit    int
		nchan   int
		smprate int
		bitrate int
	)

	fn, err := fmt.Sscanf(
		s, "%c%dc%dr%db%d", &tag, &nbit, &nchan, &smprate, &bitrate,
	)

	if fn != 5 || err != nil {
		err = mkErrorf(ErrFormat, "invalid format: %q", s)
		return
	}

	if tag != pcm.SignedIntLE {
		err = mkErrorf(ErrFormat, "invalid format: %q", tag)
		return
	}

	// Implementation goes here
	return Format{
		BitDepth:   nbit,
		Channels:   nchan,
		SampleRate: smprate,
		Bitrate:    bitrate,
	}, nil
}

func (f Format) String() string {
	return fmt.Sprintf(
		"s%dc%dr%db%d", f.BitDepth, f.Channels, f.SampleRate, f.Bitrate,
	)
}

// Coder selects the quantizer search strategy. The zero value is CoderNMR,
// upstream's default at the pinned commit. Mirrors enum AACCoder
// (libavcodec/aacenc.h @ d09d5afc3a).
//
// The constants are not ordered by speed, because speed is content dependent:
// CoderTwoLoop is faster than CoderNMR on broadband material but several times
// slower on tonal material. See each constant for the measurements.
//
// The coders also code to different bandwidths when Cutoff is 0, and neither
// is consistently the wider one. CoderNMR takes FFmpeg's tuned
// rate-to-bandwidth table above 32 kb/s per channel, while CoderTwoLoop and
// CoderFast derive the cutoff from the bitrate alone
// (aacenc.c:1592-1614 @ d09d5afc3a). At 48 kHz mono, with the tool defaults,
// that yields, in Hz:
//
//	per channel      32k     64k    128k    192k
//	CoderNMR       14000   16000   18666   20000
//	TwoLoop/Fast   11750   16600   21200   22000
//
// Below 32 kb/s per channel the NMR table does not apply yet, so every coder
// takes the same formula and agrees exactly. From there the NMR table is the
// wider one; the two meet at 14500 Hz at 40 kb/s per channel, and above that
// TwoLoop and Fast are wider. The gap is visible on a spectrogram as a
// different shelf. Set Cutoff explicitly if the coding bandwidth has to stay
// stable across coders.
//
// The tool switches move the bandwidth too, outside the NMR table: the rate
// feeding the cutoff is widened by 15% unless BOTH DisablePNS and DisableIS
// are set (aacenc.c:1609-1610), so disabling the pair narrows the coded
// bandwidth. The NMR table above 32 kb/s per channel is unaffected.
type Coder = aac.Coder
