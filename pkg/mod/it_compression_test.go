package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

type itTestBitWriter struct {
	data []byte
	bits int
}

func (w *itTestBitWriter) write(value, width int) {
	for i := range width {
		if w.bits%8 == 0 {
			w.data = append(w.data, 0)
		}
		w.data[w.bits/8] |= byte((value>>uint(i))&1) << uint(w.bits%8)
		w.bits++
	}
}

func testITCompressed(values []int, bits int, it215 bool) []byte {
	blockSamples := 0x8000
	if bits == 16 {
		blockSamples = 0x4000
	}
	var result []byte
	mask := (1 << uint(bits)) - 1
	for start := 0; start < len(values); start += blockSamples {
		var writer itTestBitWriter
		previous, previousDelta := 0, 0
		for _, value := range values[start:min(start+blockSamples, len(values))] {
			delta := (value - previous) & mask
			encoded := delta
			if it215 {
				encoded = (delta - previousDelta) & mask
			}
			writer.write(encoded, bits+1)
			previous, previousDelta = value, delta
		}
		result = binary.LittleEndian.AppendUint16(result, uint16(len(writer.data)))
		result = append(result, writer.data...)
	}
	return result
}

func TestITCompressedSample(t *testing.T) {
	for _, bits := range []int{8, 16} {
		for _, it215 := range []bool{false, true} {
			values := make([]int, 33000)
			for i := range values {
				values[i] = (i*7919 + i*i*7) & ((1 << uint(bits)) - 1)
			}
			packed := testITCompressed(values, bits, it215)
			decoded, consumed, err := decodeITCompressed(packed, len(values), bits == 16, it215)
			if err != nil {
				t.Fatal(err)
			}
			if consumed != len(packed) {
				t.Fatalf("consumed %d bytes, want %d", consumed, len(packed))
			}
			for i, value := range values {
				if value >= 1<<uint(bits-1) {
					value -= 1 << uint(bits)
				}
				want := float32(value) / float32(int(1)<<uint(bits-1))
				if decoded[i] != want {
					t.Fatalf("bits=%d IT215=%v sample %d = %g, want %g", bits, it215, i, decoded[i], want)
				}
			}
		}
	}
}

func TestITCompressedWidthChanges(t *testing.T) {
	for _, bits := range []int{8, 16} {
		var writer itTestBitWriter
		writer.write((1<<uint(bits))|2, bits+1) // width 3
		writer.write(1, 3)
		writer.write(4, 3)
		fetch, lower := 3, 124
		if bits == 16 {
			fetch, lower = 4, 120
		}
		writer.write(6, fetch) // width 8
		writer.write(1, 8)
		writer.write(lower, 8) // width 1
		writer.write(0, 1)
		packed := binary.LittleEndian.AppendUint16(nil, uint16(len(writer.data)))
		packed = append(packed, writer.data...)
		got, _, err := decodeITCompressed(packed, 3, bits == 16, false)
		if err != nil {
			t.Fatal(err)
		}
		scale := float32(int(1) << uint(bits-1))
		for i, want := range []float32{1 / scale, 2 / scale, 2 / scale} {
			if got[i] != want {
				t.Fatalf("bits=%d sample %d = %g, want %g", bits, i, got[i], want)
			}
		}
	}
}

func TestITCompressedMalformed(t *testing.T) {
	for _, data := range [][]byte{
		nil, {0, 0}, {2, 0, 0}, {1, 0, 0}, {2, 0, 0xff, 0x01},
	} {
		if _, _, err := decodeITCompressed(data, 1, false, false); err == nil {
			t.Errorf("accepted malformed compressed sample %v", data)
		}
	}
}

func TestOpenMPTITCompressedWidthChanges(t *testing.T) {
	for _, bits := range []int{8, 16} {
		var writer itTestBitWriter
		writer.write((1<<uint(bits))|2, bits+1)
		writer.write(1, 3)
		writer.write(4, 3)
		fetch, lower := 3, 124
		if bits == 16 {
			fetch, lower = 4, 120
		}
		writer.write(6, fetch)
		writer.write(1, 8)
		writer.write(lower, 8)
		for i := 2; i < 4096; i++ {
			writer.write(0, 1)
		}
		packedData := binary.LittleEndian.AppendUint16(nil, uint16(len(writer.data)))
		packedData = append(packedData, writer.data...)
		packed := append(testIT()[:512], packedData...)
		plain := testIT()[:512]
		packed[256+18], plain[256+18] = 9, 1
		if bits == 16 {
			packed[256+18] |= 2
			plain[256+18] |= 2
		}
		for i := range 4096 {
			value := uint16(2)
			if i == 0 {
				value = 1
			}
			if bits == 16 {
				plain = binary.LittleEndian.AppendUint16(plain, value)
			} else {
				plain = append(plain, byte(value))
			}
		}
		refPacked, actualPacked := referencePCMWithInterpolation(t, packed, "2")
		refPlain, actualPlain := referencePCMWithInterpolation(t, plain, "2")
		var power float64
		for i := range refPlain {
			power += float64(refPlain[i]) * float64(refPlain[i])
			if refPacked[i] != refPlain[i] || actualPacked[i] != actualPlain[i] {
				t.Fatalf("bits=%d: width-changing sample differs at %d: OpenMPT %g/%g, tracker %g/%g",
					bits, i, refPacked[i], refPlain[i], actualPacked[i], actualPlain[i])
			}
		}
		if power == 0 {
			t.Fatal("reference output is silent")
		}
	}
}

func TestOpenMPTITCompressedSample(t *testing.T) {
	for _, bits := range []int{8, 16} {
		for _, it215 := range []bool{false, true} {
			values := make([]int, 4096)
			for i := range values {
				values[i] = int(math.Sin(float64(i)*2*math.Pi*500/48000)*float64(int(1)<<uint(bits-2))) & ((1 << uint(bits)) - 1)
			}
			packed := testITCompressed(values, bits, it215)
			b := append(testIT()[:512], packed...)
			b[256+18] = 9
			if bits == 16 {
				b[256+18] |= 2
			}
			b[256+46] = 1
			if it215 {
				b[256+46] |= 4
			}
			plain := testIT()[:512]
			plain[256+18] = 1
			if bits == 16 {
				plain[256+18] |= 2
			}
			for _, value := range values {
				if bits == 16 {
					plain = binary.LittleEndian.AppendUint16(plain, uint16(value))
				} else {
					plain = append(plain, byte(value))
				}
			}
			packedRef, packedActual := referencePCMWithInterpolation(t, b, "2")
			plainRef, plainActual := referencePCMWithInterpolation(t, plain, "2")
			var power float64
			for i := range packedRef {
				power += float64(plainRef[i]) * float64(plainRef[i])
				if packedRef[i] != plainRef[i] || packedActual[i] != plainActual[i] {
					t.Fatalf("bits=%d IT215=%v: packed and plain playback differ at sample %d: OpenMPT %g/%g, tracker %g/%g",
						bits, it215, i, packedRef[i], plainRef[i], packedActual[i], plainActual[i])
				}
			}
			if power == 0 {
				t.Fatal("reference output is silent")
			}
		}
	}
}
