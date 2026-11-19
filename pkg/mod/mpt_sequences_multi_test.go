package mod

import (
	"reflect"
	"testing"
)

func TestParseMPTMultipleSequences(t *testing.T) {
	first := mptTestBlock("mptSeq", []mptTestEntry{
		{"l", []byte{2, 0}}, {"a", []byte{0, 0, 1, 0}},
	})
	second := mptTestBlock("mptSeq", []mptTestEntry{
		{"l", []byte{1, 0}}, {"a", []byte{2, 0}},
	})
	set := mptTestBlock("mptSeqC", []mptTestEntry{
		{"n", []byte{2}}, {"c", []byte{1}}, {"\x00", first}, {"\x01", second},
	})
	root := mptTestBlock("mptm", []mptTestEntry{{"mptSeqC", set}})
	got, err := parseMPTSequences(root, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.current != 1 || len(got.sequences) != 2 ||
		!reflect.DeepEqual(got.sequences[0].orders, []uint16{0, 1}) ||
		!reflect.DeepEqual(got.sequences[1].orders, []uint16{2}) {
		t.Fatalf("unexpected sequence set: %+v", got)
	}
}
