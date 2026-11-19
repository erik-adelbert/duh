package mod

import (
	"encoding/binary"
	"strings"
	"testing"
)

func mptBoundsFixture(flags, convert byte, length uint32, payload []byte) ([]byte, int) {
	b := make([]byte, 336)
	copy(b, "IMPM")
	binary.LittleEndian.PutUint16(b[36:], 1)
	binary.LittleEndian.PutUint16(b[38:], 1)
	binary.LittleEndian.PutUint16(b[40:], 0x889)
	binary.LittleEndian.PutUint32(b[192:], 256)
	copy(b[256:], "IMPS")
	b[256+18], b[256+46] = flags, convert
	binary.LittleEndian.PutUint32(b[256+48:], length)
	binary.LittleEndian.PutUint32(b[256+72:], 336)
	b = append(b, payload...)
	pattern := len(b)
	binary.LittleEndian.PutUint32(b[196:], uint32(pattern))
	b = append(b, 2, 0, 1, 0, 0, 0, 0, 0, 0, 0)
	start := len(b)
	b = append(b, []byte("STPM")...)
	tail := len(b)
	b = append(b, []byte("228")...)
	b = binary.LittleEndian.AppendUint32(b, uint32(tail))
	return b, start
}

func TestLocateMPTExtensions(t *testing.T) {
	compressed := testITCompressed([]int{1, 2, 3}, 16, true)
	tests := []struct {
		name           string
		flags, convert byte
		length         uint32
		payload        []byte
	}{
		{"empty", 0, 0, 0, nil},
		{"pcm8", 1, 1, 3, []byte{1, 2, 3}},
		{"pcm16 stereo", 7, 1, 3, make([]byte, 12)},
		{"compressed stereo", 15, 5, 3, append(append([]byte{}, compressed...), compressed...)},
		{"opl", 9, 0x40, 12, make([]byte, 12)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, start := mptBoundsFixture(tt.flags, tt.convert, tt.length, tt.payload)
			got, err := locateMPTExtensions(b)
			if err != nil {
				t.Fatal(err)
			}
			if got.propertiesStart != start || got.serializedStart != start+4 || got.serializedEnd != len(b)-4 {
				t.Fatalf("bounds = %+v, want start %d", got, start)
			}
		})
	}
}

func TestLocateMPTExtensionsOrderAndInstrument(t *testing.T) {
	b := make([]byte, 980)
	copy(b, "IMPM")
	binary.LittleEndian.PutUint16(b[34:], 1)
	binary.LittleEndian.PutUint16(b[36:], 1)
	binary.LittleEndian.PutUint16(b[38:], 1)
	binary.LittleEndian.PutUint16(b[40:], 0x889)
	binary.LittleEndian.PutUint32(b[192:], 220)
	binary.LittleEndian.PutUint32(b[196:], 900)
	binary.LittleEndian.PutUint32(b[200:], 205)
	copy(b[220:], "IMPI")
	copy(b[220+550:], "XTPM")
	copy(b[900:], "IMPS")
	b = append(b, []byte("STPM")...)
	b = append(b, []byte("228")...)
	b = binary.LittleEndian.AppendUint32(b, 984)
	got, err := locateMPTExtensions(b)
	if err != nil || got.propertiesStart != 980 {
		t.Fatalf("bounds = %+v, error = %v", got, err)
	}
	copy(b[220+550:], "MPTX")
	if _, err := locateMPTExtensions(b); err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint32(b[196:], 850)
	if _, err := locateMPTExtensions(b); err == nil {
		t.Fatal("expected truncated or overlapping extended instrument to fail")
	}
}

func TestLocateMPTExtensionsAbsentSectionsAndMessage(t *testing.T) {
	b, start := mptBoundsFixture(1, 1, 3, []byte{1, 2, 3})
	binary.LittleEndian.PutUint32(b[192:], 0)
	if got, err := locateMPTExtensions(b); err != nil || got.propertiesStart != start {
		t.Fatalf("absent sample: bounds = %+v, error = %v", got, err)
	}

	b, _ = mptBoundsFixture(1, 1, 3, []byte{1, 2, 3})
	binary.LittleEndian.PutUint32(b[196:], 0)
	b = b[:339]
	b = append(b, []byte("STPM228")...)
	b = binary.LittleEndian.AppendUint32(b, 343)
	if got, err := locateMPTExtensions(b); err != nil || got.propertiesStart != 339 {
		t.Fatalf("absent pattern: bounds = %+v, error = %v", got, err)
	}

	b, start = mptBoundsFixture(1, 1, 3, []byte{1, 2, 3})
	b[46] = 1
	binary.LittleEndian.PutUint16(b[54:], 4)
	binary.LittleEndian.PutUint32(b[56:], 240)
	if got, err := locateMPTExtensions(b); err != nil || got.propertiesStart != start {
		t.Fatalf("message: bounds = %+v, error = %v", got, err)
	}
	binary.LittleEndian.PutUint32(b[56:], 257)
	if _, err := locateMPTExtensions(b); err == nil {
		t.Fatal("overlapping message was accepted")
	}
}

func TestLocateMPTExtensionsRejectsInvalid(t *testing.T) {
	base, start := mptBoundsFixture(1, 1, 12, make([]byte, 12))
	tests := []struct {
		name string
		edit func([]byte) []byte
		want string
	}{
		{"old header", func(b []byte) []byte { copy(b, "tpm."); return b }, "header"},
		{"table overlaps tail", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[32:], 65535); return b }, "table"},
		{"bad sample pointer", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[192:], ^uint32(0)); return b }, "bounds"},
		{"bad sample magic", func(b []byte) []byte { b[256] = 0; return b }, "header"},
		{"external sample", func(b []byte) []byte { b[302] = 0x80; return b }, "encoding"},
		{"external empty sample", func(b []byte) []byte { b[302] = 0x80; binary.LittleEndian.PutUint32(b[304:], 0); return b }, "encoding"},
		{"empty sample bad pointer", func(b []byte) []byte {
			binary.LittleEndian.PutUint32(b[304:], 0)
			binary.LittleEndian.PutUint32(b[328:], ^uint32(0))
			return b
		}, "offset"},
		{"unsupported PCM conversion", func(b []byte) []byte { b[302] = 4; return b }, "encoding"},
		{"bad OPL length", func(b []byte) []byte { b[302] = 0x40; binary.LittleEndian.PutUint32(b[304:], 11); return b }, "OPL"},
		{"overflow length", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[304:], ^uint32(0)); return b }, "bounds"},
		{"truncated PCM", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[328:], uint32(start-2)); return b }, "bounds"},
		{"truncated compressed", func(b []byte) []byte { b[274] = 9; b[336], b[337] = 255, 255; return b }, "compressed"},
		{"pattern pointer", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[196:], uint32(start+5)); return b }, "pattern"},
		{"truncated pattern", func(b []byte) []byte { b[348], b[349] = 255, 255; return b }, "pattern"},
		{"overlap", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[196:], 336); return b }, "overlapping"},
		{"missing extensions", func(b []byte) []byte { b[start] = 0; return b }, "extension header"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.edit(append([]byte(nil), base...))
			_, err := locateMPTExtensions(b)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestScanITCompressedMatchesDecoder(t *testing.T) {
	for _, bits := range []int{8, 16} {
		for _, it215 := range []bool{false, true} {
			values := make([]int, 33000)
			for i := range values {
				values[i] = i & ((1 << uint(bits)) - 1)
			}
			data := testITCompressed(values, bits, it215)
			_, decoded, err := decodeITCompressed(data, len(values), bits == 16, it215)
			if err != nil {
				t.Fatal(err)
			}
			pcm, scanned, err := readITCompressed(data, len(values), bits == 16, it215, false)
			if err != nil || pcm != nil || scanned != decoded {
				t.Fatalf("bits=%d it215=%v: scanned=%d decoded=%d pcm=%v err=%v", bits, it215, scanned, decoded, pcm, err)
			}
			if _, _, err := readITCompressed(data[:len(data)-1], len(values), bits == 16, it215, false); err == nil {
				t.Fatal("truncated stream was accepted")
			}
		}
	}
}

func FuzzLocateMPTExtensions(f *testing.F) {
	b, _ := mptBoundsFixture(1, 1, 3, []byte{1, 2, 3})
	f.Add(b)
	f.Add([]byte("tpm."))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = locateMPTExtensions(data)
	})
}
