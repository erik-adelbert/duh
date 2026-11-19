package mod

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func mptPitchPanProfileFixture(t *testing.T, enabled bool) []byte {
	t.Helper()
	flags := []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}
	if enabled {
		flags = append(flags, make([]byte, 8)...)
		flags[115/8] |= 1 << (115 % 8)
	}

	b := oplEnvelopeEndFixture(t, 0x01311400, flags, []byte{0})
	b[30], b[44], b[50] = 8, b[44]|8, 2
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	binary.LittleEndian.PutUint16(b[pattern+2:], 32)
	copy(b[pattern+8:], []byte{0x81, 3, 60, 2, 0, 0x81, 8, 24, 200, 0})
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	properties := append([]byte(nil), b[bounds.propertiesStart:bounds.serializedStart]...)
	for _, field := range []struct {
		code string
		data []byte
	}{{"...C", []byte{1, 0}}, {"..MT", []byte{0}}, {".MMP", []byte{4, 0, 0, 0}},
		{".APS", []byte{64, 0, 0, 0}}, {"VTSV", []byte{64, 0, 0, 0}}} {
		properties = append(properties, mptExtensionField(field.code, uint16(len(field.data)), field.data)...)
	}
	result := append([]byte(nil), b[:bounds.propertiesStart]...)
	result = append(result, properties...)
	tail := len(result)
	result = append(result, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(result, uint32(tail))
}

func TestMPTPitchPanProfile(t *testing.T) {
	bits := make([]byte, 15)
	bits[115/8] = 1 << (115 % 8)
	_, flags, err := parseMPTPlaybackProfileFlags(bits)
	if err != nil || !flags.itPitchPanSeparation {
		t.Fatalf("bit 115: flags %+v, error %v", flags, err)
	}
	if _, err := parseMPTSavedPlayBehaviours(bits); err == nil {
		t.Fatal("strict flag parser accepted bit 115")
	}
	for _, enabled := range []bool{false, true} {
		b := mptPitchPanProfileFixture(t, enabled)
		loaded, err := loadMPTM(b)
		if err != nil {
			t.Fatalf("enabled=%t zero separation: %v", enabled, err)
		}
		if loaded.Assets.ITCompatibility.mptPitchPanSeparation != enabled {
			t.Fatalf("enabled=%t: wrong projected pitch-pan flag", enabled)
		}
		ptr := int(binary.LittleEndian.Uint32(b[198:]))
		for _, separation := range []byte{1, 0xff} {
			b[ptr+22] = separation
			_, err := loadMPTM(b)
			if err != nil {
				t.Fatalf("separation=%d: %v", separation, err)
			}
		}
	}
}

func TestMPTPitchPanVersionGate(t *testing.T) {
	for _, tt := range []struct {
		version uint32
		valid   bool
	}{
		{0x012f0000, false},
		{0x01300000, true},
		{0x01300052, false},
		{0x01300053, true},
	} {
		b := mptPitchPanProfileFixture(t, true)
		binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], tt.version)
		ptr := int(binary.LittleEndian.Uint32(b[198:]))
		b[ptr+22] = 16
		_, err := loadMPTM(b)
		if tt.valid && err != nil || !tt.valid && (err == nil || !strings.Contains(err.Error(), "unavailable playback behaviour")) {
			t.Fatalf("version %#x: %v", tt.version, err)
		}
	}
}

func TestOpenMPTMPitchPanProfileZero(t *testing.T) {
	plainRef, plainActual := mptSharedEffectRenderForDuration(t, mptPitchPanProfileFixture(t, false), 120)
	flagRef, flagActual := mptSharedEffectRenderForDuration(t, mptPitchPanProfileFixture(t, true), 120)
	var energy float64
	for i := range plainRef {
		energy += float64(plainRef[i]) * float64(plainRef[i])
		if math.Abs(float64(plainRef[i]-flagRef[i])) > 1e-6 {
			t.Fatalf("reference differs at sample %d", i)
		}
		if plainActual[i] != flagActual[i] {
			t.Fatalf("tracker differs at sample %d", i)
		}
	}
	if energy < .001 {
		t.Fatal("silent comparison")
	}
}
