package mod

import (
	"encoding/binary"
	"os"
	"reflect"
	"strings"
	"testing"
)

type mptTestEntry struct {
	id   string
	data []byte
}

func mptTestAdaptive(value uint64, tagBits int, length int) []byte {
	tag := length - 1
	if tagBits == 2 && length == 8 {
		tag = 3
	}
	n := value<<tagBits | uint64(tag)
	out := make([]byte, length)
	for i := range out {
		out[i] = byte(n >> (8 * i))
	}
	return out
}

func mptTestBlock(object string, entries []mptTestEntry) []byte {
	b := append([]byte("228"), byte(len(object)))
	b = append(b, object...)
	b = append(b, 0x1f, 8, 0, 1, 0, 1)
	b = append(b, mptTestAdaptive(uint64(len(entries)), 2, 2)...)
	mapField := len(b)
	b = append(b, make([]byte, 8)...)
	var index []byte
	for _, entry := range entries {
		index = append(index, mptTestAdaptive(uint64(len(entry.id)), 1, 1)...)
		index = append(index, entry.id...)
		index = append(index, mptTestAdaptive(uint64(len(b)), 2, 8)...)
		index = append(index, mptTestAdaptive(uint64(len(entry.data)), 2, 8)...)
		b = append(b, entry.data...)
	}
	copy(b[mapField:], mptTestAdaptive(uint64(len(b)), 2, 8))
	return append(b, index...)
}

func mptTestSequences(seq []mptTestEntry) []byte {
	sequence := mptTestBlock("mptSeq", seq)
	set := mptTestBlock("mptSeqC", []mptTestEntry{{"n", []byte{1}}, {"c", []byte{0}}, {"\x00", sequence}})
	return mptTestBlock("mptm", []mptTestEntry{{"mptSeqC", set}})
}

func TestParseMPTSequences(t *testing.T) {
	b := mptTestSequences([]mptTestEntry{
		{"u", []byte{1}}, {"n", []byte{0x2c, 0, 0, 0, 'O', 'K'}},
		{"l", []byte{3, 0}}, {"a", []byte{1, 0, 0xfe, 0xff, 0xff, 0xff}},
		{"r", []byte{1, 0}}, {"t", []byte{0xd0, 0x12, 0x13, 0}}, {"s", []byte{6, 0, 0, 0}},
	})
	got, err := parseMPTSequences(b, 2)
	if err != nil {
		t.Fatal(err)
	}
	seq := got.sequences[0]
	if got.current != 0 || seq.name != "OK" || !seq.utf8Name ||
		!reflect.DeepEqual(seq.orders, []uint16{1, 0xfffe, 0xffff}) ||
		seq.restart == nil || *seq.restart != 1 || seq.defaultTempo == nil || *seq.defaultTempo != 125*10000 ||
		seq.defaultSpeed == nil || *seq.defaultSpeed != 6 {
		t.Fatalf("unexpected sequence: %+v", seq)
	}
	for i := range b {
		b[i] = 0
	}
	if seq.name != "OK" || seq.orders[0] != 1 {
		t.Fatal("sequence aliases the input")
	}
}

func TestParseMPTSequencesInvalid(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"unknown property", mptTestBlock("mptm", []mptTestEntry{{"other", []byte{0}}}), "unsupported"},
		{"duplicate property", mptTestBlock("mptm", []mptTestEntry{{"mptSeqC", []byte{0}}, {"mptSeqC", []byte{1}}}), "duplicate"},
		{"missing sequence", mptTestBlock("mptm", []mptTestEntry{{"mptSeqC", mptTestBlock("mptSeqC", []mptTestEntry{{"n", []byte{1}}, {"c", []byte{0}}, {"x", []byte{0}}})}}), "missing"},
		{"invalid selection", mptTestBlock("mptm", []mptTestEntry{{"mptSeqC", mptTestBlock("mptSeqC", []mptTestEntry{{"n", []byte{1}}, {"c", []byte{1}}, {"\x00", []byte{0}}})}}), "selection"},
		{"invalid order", mptTestSequences([]mptTestEntry{{"l", []byte{1, 0}}, {"a", []byte{2, 0}}}), "missing pattern"},
		{"truncated orders", mptTestSequences([]mptTestEntry{{"l", []byte{2, 0}}, {"a", []byte{1, 0}}}), "length"},
		{"missing empty orders", mptTestSequences([]mptTestEntry{{"l", []byte{0, 0}}}), "length"},
		{"unknown sequence field", mptTestSequences([]mptTestEntry{{"x", []byte{0}}}), "unsupported"},
		{"invalid restart", mptTestSequences([]mptTestEntry{{"l", []byte{1, 0}}, {"a", []byte{0, 0}}, {"r", []byte{1, 0}}}), "restart"},
		{"invalid name", mptTestSequences([]mptTestEntry{{"l", []byte{0, 0}}, {"a", []byte{}}, {"u", []byte{1}}, {"n", []byte{0x1c, 0, 0, 0, 0xff}}}), "name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseMPTSequences(tt.data, 2)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
	valid := mptTestSequences([]mptTestEntry{{"l", []byte{1, 0}}, {"a", []byte{0, 0}}})
	for i := range valid {
		if _, err := parseMPTSequences(valid[:i], 2); err == nil {
			t.Fatalf("truncation at %d accepted", i)
		}
	}
}

func TestParseTargetMPTSequences(t *testing.T) {
	path := os.Getenv("MPTECS_MPTM_TARGET")
	if path == "" {
		t.Skip("set MPTECS_MPTM_TARGET to the Triangel Island fixture")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !isMPTM(b) {
		t.Fatal("not an MPTM fixture")
	}
	start := binary.LittleEndian.Uint32(b[len(b)-4:])
	patterns := int(binary.LittleEndian.Uint16(b[38:]))
	root, err := parseMPTSerializedBlock(b[int(start):len(b)-4], "mptm")
	if err != nil {
		t.Fatal(err)
	}
	if _, present := root["mptPc"]; present {
		t.Fatal("target has serialized pattern overrides")
	}
	got, err := parseMPTSequences(b[int(start):len(b)-4], patterns)
	if err != nil {
		t.Fatal(err)
	}
	want := []uint16{4, 5, 6, 0, 1, 2, 3, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 13, 21, 22, 23, 24, 25, 26, 27, 28}
	if got.current != 0 || len(got.sequences) != 1 || !reflect.DeepEqual(got.sequences[0].orders, want) {
		t.Fatalf("unexpected target sequences: %+v", got)
	}
}

func FuzzParseMPTSequences(f *testing.F) {
	f.Add(mptTestSequences([]mptTestEntry{{"l", []byte{1, 0}}, {"a", []byte{0, 0}}}))
	f.Add([]byte("228"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseMPTSequences(data, 32)
	})
}
