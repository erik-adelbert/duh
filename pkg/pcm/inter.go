// duh pcm package
//
// Based on the UCB release of Plan 9 pcmconv.
//
// # Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package pcm

import (
	"encoding/binary"
	"math"
)

// encodeIntLE writes nbit-bit signed little-endian samples, interleaved by
// nchan, taking the lower nbit bits from each 32-bit input sample.
func encodeIntLE(out []byte, in []int32, nbit, nchan, nframe int) {
	splsz := (nbit + 7) / 8
	shift := 32 - nbit
	framesz := splsz * nchan

	for i := range nframe {
		froff := i * framesz
		ioff := i * nchan

		for ch := range nchan {
			v := uint32(in[ioff+ch]) >> shift
			b := froff + ch*splsz

			for j := range splsz {
				out[b+j] = byte(v & 0xff)
				v >>= 8
			}
		}
	}
}

// encodeIntBE writes nbit-bit signed big-endian samples, interleaved by
// nchan, taking the lower nbit bits from each 32-bit input sample.
func encodeIntBE(out []byte, in []int32, nbit, nchan, nframe int) {
	splsz := (nbit + 7) / 8
	shift := 32 - nbit
	framesz := splsz * nchan

	for i := range nframe {
		froff := i * framesz
		ioff := i * nchan

		for ch := range nchan {
			v := uint32(in[ioff+ch]) >> shift
			b := froff + ch*splsz

			for j := splsz - 1; j >= 0; j-- {
				out[b+j] = byte(v & 0xff)
				v >>= 8
			}
		}
	}
}

// encodeUintLE writes nbit-bit unsigned little-endian samples, interleaved by
// nchan, taking the lower nbit bits from each 32-bit input sample.
func encodeUintLE(out []byte, in []int32, nbit, nchan, nframe int) {
	splsz := (nbit + 7) / 8
	shift := 32 - nbit
	framesz := splsz * nchan

	bias := uint32(1) << 31

	for i := range nframe {
		froff := i * framesz
		ioff := i * nchan

		for ch := range nchan {
			v := (bias + uint32(in[ioff+ch])) >> shift
			b := froff + ch*splsz

			for j := range splsz {
				out[b+j] = byte(v & 0xff)
				v >>= 8
			}
		}
	}
}

// encodeUintBE writes nbit-bit unsigned big-endian samples, interleaved by
// nchan, taking the lower nbit bits from each 32-bit input sample.
func encodeUintBE(out []byte, in []int32, nbit, nchan, nframe int) {
	splsz := (nbit + 7) / 8
	shift := 32 - nbit
	framesz := splsz * nchan
	bias := uint32(1) << 31

	for i := range nframe {
		froff := i * framesz
		ioff := i * nchan

		for ch := range nchan {
			v := (bias + uint32(in[ioff+ch])) >> shift
			b := froff + ch*splsz

			for j := splsz - 1; j >= 0; j-- {
				out[b+j] = byte(v & 0xff)
				v >>= 8
			}
		}
	}
}

// encodeFloat32 writes 32-bit floating-point samples, interleaved by
// nchan, taking each 32-bit input sample and converting it to float32.
func encodeFloat32(out []byte, in []int32, nchan, nframe int) {
	const scale = float32(1 << 31)
	framesz := 4 * nchan

	putuin32 := binary.LittleEndian.PutUint32

	for i := range nframe {
		froff := i * framesz
		ioff := i * nchan

		for ch := range nchan {
			f := float32(in[ioff+ch]) / scale
			putuin32(out[froff+ch*4:], math.Float32bits(f))
		}
	}
}

// encodeFloat64 writes 64-bit floating-point samples, interleaved by
// nchan, taking each 32-bit input sample and converting it to float64.
func encodeFloat64(out []byte, in []int32, nchan, nframe int) {
	const scale = float64(1 << 31)
	framesz := 8 * nchan

	putuin64 := binary.LittleEndian.PutUint64

	for i := range nframe {
		froff := i * framesz
		ioff := i * nchan

		for ch := range nchan {
			f := float64(in[ioff+ch]) / scale
			putuin64(out[froff+ch*8:], math.Float64bits(f))
		}
	}
}

// encode writes samples in the format specified by f, interleaved by f.channelCount.
// It dispatches to the appropriate encode function based on f.tag and f.bitDepth.
func encode(out []byte, in []int32, f Format, nframe int) {
	nbit := f.bitDepth
	nchan := f.channelCount

	switch f.tag {
	case SignedIntLE:
		encodeIntLE(out, in, nbit, nchan, nframe)
	case SignedIntBE:
		encodeIntBE(out, in, nbit, nchan, nframe)
	case UnsignedIntLE:
		encodeUintLE(out, in, nbit, nchan, nframe)
	case UnsignedIntBE:
		encodeUintBE(out, in, nbit, nchan, nframe)
	case Float:
		if nbit == 32 {
			encodeFloat32(out, in, nchan, nframe)
		} else {
			encodeFloat64(out, in, nchan, nframe)
		}
	}
}

// decodeIntLE reads nbit-bit signed little-endian samples, interleaved by
// f.channelCount, left-aligning each to 32 bits.
func decodeIntLE(out []int32, in []byte, nbit, nchan, nframe int) {
	splsz := (nbit + 7) / 8
	shift := 32 - nbit
	framesz := splsz * nchan

	for i := range nframe {
		froff := i * framesz
		ooff := i * nchan

		for ch := range nchan {
			var v uint32
			b := froff + ch*splsz

			for j := range splsz {
				v |= uint32(in[b+j]) << (8 * j)
			}

			out[ooff+ch] = int32(v << shift)
		}
	}
}

// decodeIntBE reads nbit-bit signed big-endian samples, interleaved by
// nchan, left-aligning each to 32 bits.
func decodeIntBE(out []int32, in []byte, nbit, nchan, nframe int) {
	splsz := (nbit + 7) / 8
	shift := 32 - nbit
	framesz := splsz * nchan

	for i := range nframe {
		froff := i * framesz
		ooff := i * nchan
		for ch := range nchan {
			var v uint32
			b := froff + ch*splsz
			for j := range splsz {
				v = (v << 8) | uint32(in[b+j])
			}
			out[ooff+ch] = int32(v << shift)
		}
	}
}

// decodeUintLE reads nbit-bit unsigned little-endian samples, interleaved by
// nchan, left-aligning each to 32 bits with a bias subtraction.
func decodeUintLE(out []int32, in []byte, nbit, nchan, nframe int) {
	splsz := (nbit + 7) / 8
	shift := 32 - nbit
	framesz := splsz * nchan
	bias := uint32(1) << 31

	for i := range nframe {
		froff := i * framesz
		ooff := i * nchan
		for ch := range nchan {
			var v uint32
			b := froff + ch*splsz
			for j := range splsz {
				v |= uint32(in[b+j]) << (8 * j)
			}
			out[ooff+ch] = int32((v << shift) - bias)
		}
	}
}

// decodeUintBE reads nbit-bit unsigned big-endian samples, interleaved by
// nchan, left-aligning each to 32 bits with a bias subtraction.
func decodeUintBE(out []int32, in []byte, nbit, nchan, nframe int) {
	splsz := (nbit + 7) / 8
	shift := 32 - nbit
	framesz := splsz * nchan
	bias := uint32(1) << 31

	for i := range nframe {
		froff := i * framesz
		ooff := i * nchan
		for ch := range nchan {
			var v uint32
			b := froff + ch*splsz
			for j := range splsz {
				v = (v << 8) | uint32(in[b+j])
			}
			out[ooff+ch] = int32((v << shift) - bias)
		}
	}
}

// decodeFloat32 reads 32-bit floating-point samples, interleaved by
// nchan, converting each to a 32-bit integer with left alignment.
func decodeFloat32(out []int32, in []byte, nchan, nframe int) {
	const scale = float32(1 << 31)
	framesz := 4 * nchan

	for i := range nframe {
		froff := i * framesz
		ooff := i * nchan
		for ch := range nchan {
			f := math.Float32frombits(binary.LittleEndian.Uint32(in[froff+ch*4:]))
			switch {
			case f >= 1:
				out[ooff+ch] = math.MaxInt32
			case f <= -1:
				out[ooff+ch] = math.MinInt32
			default:
				out[ooff+ch] = int32(f * scale)
			}
		}
	}
}

// decodeFloat64 reads 64-bit floating-point samples, interleaved by
// nchan, converting each to a 32-bit integer with left alignment.
func decodeFloat64(out []int32, in []byte, nchan, nframe int) {
	const scale = float64(1 << 31)
	framesz := 8 * nchan

	for i := range nframe {
		froff := i * framesz
		ooff := i * nchan
		for ch := range nchan {
			f := math.Float64frombits(binary.LittleEndian.Uint64(in[froff+ch*8:]))
			switch {
			case f >= 1:
				out[ooff+ch] = math.MaxInt32
			case f <= -1:
				out[ooff+ch] = math.MinInt32
			default:
				out[ooff+ch] = int32(f * scale)
			}
		}
	}
}

// decode reads samples in the format specified by f, interleaved by f.channelCount,
// converting each to a 32-bit integer and storing them in out.
func decode(out []int32, in []byte, f Format, nframe int) {
	nbit := f.bitDepth
	nchan := f.channelCount

	switch f.tag {
	case SignedIntLE:
		decodeIntLE(out, in, nbit, nchan, nframe)
	case SignedIntBE:
		decodeIntBE(out, in, nbit, nchan, nframe)
	case UnsignedIntLE:
		decodeUintLE(out, in, nbit, nchan, nframe)
	case UnsignedIntBE:
		decodeUintBE(out, in, nbit, nchan, nframe)
	case Float:
		if nbit == 32 {
			decodeFloat32(out, in, nchan, nframe)
		} else {
			decodeFloat64(out, in, nchan, nframe)
		}
	case ALaw:
		decodeAlaw(out, in, nchan, nframe)
	case MuLaw:
		decodeMulaw(out, in, nchan, nframe)
	}
}

// decodeAlaw decodes 8-bit A-law samples, interleaved by nchan,
// converting each to a 32-bit integer with left alignment.
func decodeAlaw(out []int32, in []byte, nchan, nframe int) {
	for i := range nframe {
		froff := i * nchan
		ooff := i * nchan

		for ch := range nchan {
			a := in[froff+ch] ^ 0x55
			t := int32(a&0xf) << 4

			seg := int32((a & 0x70) >> 4)

			switch seg {
			case 0:
				t += 8
			case 1:
				t += 0x108
			default:
				t += 0x108
				t <<= seg - 1
			}

			if a&0x80 == 0 {
				t = -t
			}

			out[ooff+ch] = t << 16
		}
	}
}

// decodeMulaw decodes 8-bit μ-law samples, interleaved by nchan,
// converting each to a 32-bit integer with left alignment.
func decodeMulaw(out []int32, in []byte, nchan, nframe int) {
	for i := range nframe {
		froff := i * nchan
		ooff := i * nchan

		for ch := range nchan {
			μ := ^in[froff+ch]

			t := (int32(μ&0xf) << 3) + 0x84
			t <<= (μ & 0x70) >> 4
			t = 0x84 - t

			if μ&0x80 == 0 {
				t = -t
			}

			out[ooff+ch] = t << 16
		}
	}
}
