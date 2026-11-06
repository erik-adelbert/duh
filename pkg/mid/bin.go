package mid

import "encoding/binary"

func ReadBEInt(data []byte, nbytes int) (x uint32) {
	for i := range nbytes {
		x = (x << 8) + uint32(data[i])
	}

	return
}

func u16(data []byte) uint16 {
	return binary.LittleEndian.Uint16(data)
}

func u32(data []byte) uint32 {
	return binary.LittleEndian.Uint32(data)
}

func U16(data []byte) uint16 {
	return binary.BigEndian.Uint16(data)
}

func U32(data []byte) uint32 {
	return binary.BigEndian.Uint32(data)
}
