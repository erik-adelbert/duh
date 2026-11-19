package mod

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// decodeITCompressed decodes one channel of IT 2.14 or 2.15 sample data.
func decodeITCompressed(data []byte, length int, sixteenBit, it215 bool) ([]float32, int, error) {
	return readITCompressed(data, length, sixteenBit, it215, true)
}

func readITCompressed(data []byte, length int, sixteenBit, it215, decode bool) ([]float32, int, error) {
	bits, blockSamples, fetch := 8, 0x8000, 3
	if sixteenBit {
		bits, blockSamples, fetch = 16, 0x4000, 4
	}
	if length < 0 || uint64(length) > uint64(len(data))*8 {
		return nil, 0, errors.New("compressed sample length exceeds available data")
	}

	var output []float32
	if decode {
		output = make([]float32, 0, length)
	}

	offset := 0
	produced := 0
	for produced < length {
		if len(data)-offset < 2 {
			return nil, 0, errors.New("truncated compressed sample block header")
		}

		size := int(binary.LittleEndian.Uint16(data[offset:]))
		offset += 2
		if size == 0 || size > len(data)-offset {
			return nil, 0, errors.New("invalid or truncated compressed sample block")
		}

		block := data[offset : offset+size]
		offset += size
		bitPos, width := 0, bits+1
		first, second := 0, 0
		end := min(produced+blockSamples, length)

		readBits := func(n int) (int, error) {
			if n > len(block)*8-bitPos {
				return 0, errors.New("truncated compressed sample bitstream")
			}
			value := 0
			for i := range n {
				value |= int((block[bitPos/8]>>uint(bitPos%8))&1) << uint(i)
				bitPos++
			}
			return value, nil
		}

		for produced < end {
			if width < 1 || width > bits+1 {
				return nil, 0, fmt.Errorf("invalid compressed sample bit width %d", width)
			}

			value, err := readBits(width)
			if err != nil {
				return nil, 0, err
			}

			top := 1 << uint(width-1)

			if width <= 6 && value == top {
				next, err := readBits(fetch)
				if err != nil {
					return nil, 0, err
				}

				next++
				if next >= width {
					next++
				}

				width = next
				continue
			}

			if width > 6 && width <= bits {
				lower := top - (1 << uint(fetch-1))

				if value >= lower && value < lower+(1<<uint(fetch)) {
					next := value - lower + 1
					if next >= width {
						next++
					}

					width = next
					continue
				}
			}

			if width == bits+1 {
				if value&top != 0 {
					width = (value & (top - 1)) + 1
					continue
				}
			} else if value&top != 0 {
				value -= top * 2
			}

			mask := (1 << uint(bits)) - 1
			first = (first + value) & mask
			second = (second + first) & mask
			sample := first
			if it215 {
				sample = second
			}

			if sample&(1<<uint(bits-1)) != 0 {
				sample -= 1 << uint(bits)
			}

			if decode {
				output = append(output, float32(sample)/float32(int(1)<<uint(bits-1)))
			}

			produced++
		}
	}

	return output, offset, nil
}
