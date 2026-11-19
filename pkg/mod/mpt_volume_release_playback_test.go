package mod

import (
	"encoding/binary"
	"testing"
)

func mptVolumeReleaseFixture(t *testing.T, node byte) []byte {
	t.Helper()
	b := mptResamplingFixture(t, 1, 60, true)
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	pcm := int(binary.LittleEndian.Uint32(b[sample+72:]))
	for i := range 4096 {
		binary.LittleEndian.PutUint16(b[pcm+i*2:], uint16(int16(12000)))
	}
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	setTestEnvelope(b[instrument:instrument+554], 304, 5, []int8{16, 48, 24, 0}, []uint16{0, 2, 6, 12})
	b[instrument+308], b[instrument+309] = 1, 1
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	rows := []byte{0x81, 3, 60, 2, 0, 0, 0, 0x81, 1, 255, 0}
	rows = append(rows, make([]byte, 60)...)
	oldLength := int(binary.LittleEndian.Uint16(b[pattern:]))
	oldTail := binary.LittleEndian.Uint32(b[len(b)-4:])
	data := append([]byte(nil), b[:pattern+8]...)
	binary.LittleEndian.PutUint16(data[pattern:], uint16(len(rows)))
	data = append(data, rows...)
	data = append(data, b[pattern+8+oldLength:len(b)-4]...)
	b = binary.LittleEndian.AppendUint32(data, oldTail+uint32(len(rows)-oldLength))
	binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01290000)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	data = append([]byte(nil), b[:bounds.propertiesStart+4]...)
	data = append(data, mptExtensionField("NREV", 1, []byte{255, node})...)
	data = append(data, b[bounds.propertiesStart+4:bounds.serializedStart]...)
	tail := len(data)
	data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(data, uint32(tail))
}
