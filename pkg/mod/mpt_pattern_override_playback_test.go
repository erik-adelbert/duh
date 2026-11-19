package mod

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTPluginAutomationBypass(t *testing.T) {
	b := mptSongResamplingFixture(t, 1, 5, true)
	b[bytesIndex(t, b, "...C")+6] = 2
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseMPTSerializedBlock(b[bounds.serializedStart:bounds.serializedEnd], "mptm")
	if err != nil {
		t.Fatal(err)
	}
	patternData := append([]byte{0x82, 0x23, 0xfb, 1, 80}, make([]byte, 64)...)
	pattern := mptTestBlock("mptP", []mptTestEntry{{"data", patternData}})
	patterns := mptTestBlock("mptPc", []mptTestEntry{{"num", []byte{1, 0}}, {"\x00\x00", pattern}})
	tail := mptTestBlock("mptm", []mptTestEntry{{"mptSeqC", root["mptSeqC"]}, {"mptPc", patterns}})
	b = append(append([]byte(nil), b[:bounds.serializedStart]...), tail...)
	b = binary.LittleEndian.AppendUint32(b, uint32(bounds.serializedStart))
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(loaded.Warnings, "\n"), "parameter automation is bypassed") {
		t.Fatalf("missing automation warning: %v", loaded.Warnings)
	}
	ref, actual := mptSharedEffectRenderForDuration(t, b, 100)
	var cross, r2, a2 float64
	for i := 10 * 96; i < 80*96; i++ {
		r, a := float64(ref[i]), float64(actual[i])
		cross, r2, a2 = cross+r*a, r2+r*r, a2+a*a
	}
	correlation := cross / math.Sqrt(r2*a2)
	if r2 < .001 || a2 < .001 || math.IsNaN(correlation) || correlation < .98 {
		t.Fatalf("correlation %g energy %g/%g", correlation, r2, a2)
	}
}
