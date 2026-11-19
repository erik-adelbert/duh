// Package testfixtures provides synthetic tracker modules for tests.
package testfixtures

import (
	"encoding/binary"
	"math"
)

// IT returns a small IT module with one sample and one pattern.
func IT() []byte {
	put16 := binary.LittleEndian.PutUint16
	put32 := binary.LittleEndian.PutUint32

	const sampleHeader, pattern, sampleData = 256, 336, 512
	b := make([]byte, sampleData+8192)
	copy(b, "IMPM")
	put16(b[32:], 2)
	put16(b[36:], 1)
	put16(b[38:], 1)
	put16(b[40:], 0x214)
	put16(b[42:], 0x214)
	put16(b[44:], 1)
	b[48], b[49], b[50], b[51] = 128, 48, 6, 125
	for i := range 64 {
		b[64+i], b[128+i] = 32, 64
	}
	b[192], b[193] = 0, 255
	put32(b[194:], sampleHeader)
	put32(b[198:], pattern)
	copy(b[sampleHeader:], "IMPS")
	b[sampleHeader+17], b[sampleHeader+18], b[sampleHeader+19], b[sampleHeader+46] = 64, 3, 64, 1
	put32(b[sampleHeader+48:], 4096)
	put32(b[sampleHeader+60:], 48000)
	put32(b[sampleHeader+72:], sampleData)
	for i := range 4096 {
		value := int16(math.Sin(float64(i)*2*math.Pi*500/48000) * 16000)
		put16(b[sampleData+i*2:], uint16(value))
	}
	packed := []byte{0x81, 7, 60, 1, 32, 0, 0x81, 0x70, 0}
	packed = append(packed, make([]byte, 62)...)
	put16(b[pattern:], uint16(len(packed)))
	put16(b[pattern+2:], 64)
	copy(b[pattern+8:], packed)
	return b
}

// S3M returns a single-channel S3M with a sample and one effect.
func S3M(effect, argument byte, sample []byte) []byte {
	put16 := binary.LittleEndian.PutUint16
	put32 := binary.LittleEndian.PutUint32

	const instrument = 0x100
	const pattern = 0x150
	const sampleOffset = 0x200
	data := make([]byte, sampleOffset+len(sample))
	copy(data, "reference fixture")
	data[0x1c], data[0x1d] = 0x1a, 0x10
	put16(data[0x20:], 2)
	put16(data[0x22:], 1)
	put16(data[0x24:], 1)
	put16(data[0x28:], 0x1320)
	put16(data[0x2a:], 2)
	copy(data[0x2c:], "SCRM")
	data[0x30], data[0x31], data[0x32], data[0x33] = 64, 6, 125, 0xb0
	for i := range 32 {
		data[0x40+i] = 0xff
	}
	data[0x40] = 0
	data[0x60], data[0x61] = 0, 0xff
	put16(data[0x62:], instrument/16)
	put16(data[0x64:], pattern/16)
	data[instrument] = 1
	put16(data[instrument+0x0e:], sampleOffset/16)
	put32(data[instrument+0x10:], uint32(len(sample)))
	data[instrument+0x1c] = 64
	put32(data[instrument+0x20:], 8363)
	copy(data[instrument+0x4c:], "SCRS")
	p := pattern + 2
	data[p], data[p+1], data[p+2] = 0xa0, 0x40, 1
	data[p+3], data[p+4] = effect, argument
	p += 5
	p += 64
	put16(data[pattern:], uint16(p-pattern-2))
	copy(data[sampleOffset:], sample)
	return data
}
