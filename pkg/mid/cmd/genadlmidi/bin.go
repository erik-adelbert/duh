package main

import (
	"bytes"
	"encoding/binary"
	"os"
)

func bread[T any](data []byte) T {
	var x T

	_ = binary.Read(bytes.NewReader(data), binary.LittleEndian, &x)
	return x
}

func fread(f *os.File, data any) error {
	return binary.Read(f, binary.LittleEndian, data)
}

func fwrite(f *os.File, data any) error {
	return binary.Write(f, binary.LittleEndian, data)
}

func u16(data []byte) uint16 {
	return binary.LittleEndian.Uint16(data)
}

func u32(data []byte) uint32 {
	return binary.LittleEndian.Uint32(data)
}

func u64(data []byte) uint64 {
	return binary.LittleEndian.Uint64(data)
}

func i32(data []byte) int32 {
	return int32(u32(data))
}

func lo8[T ~uint | ~uint16](data T) uint8 {
	return uint8(data & 0xFF)
}

func hi8[T ~uint | ~uint16](data T) uint8 {
	return uint8((data >> 8) & 0xFF)
}

func has[T ~int | ~uint8 | ~uint16](flags, mask T) bool {
	return flags&mask != 0
}
