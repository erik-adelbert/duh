package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func mptInitialNoteFixture(t *testing.T, enabled bool) []byte {
	t.Helper()
	b := mptPortaFilterFixture(t, false)
	pattern := int(binary.LittleEndian.Uint32(b[202:]))
	clear(b[pattern+8 : pattern+8+int(binary.LittleEndian.Uint16(b[pattern:]))])
	copy(b[pattern+8:], []byte{0x81, 2, 1, 0})
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	field := bytesIndex(t, b, ".FSM")
	flags := make([]byte, 16)
	copy(flags, b[field+6:field+21])
	if enabled {
		flags[mptITInitialNoteMemoryBit/8] |= 1 << (mptITInitialNoteMemoryBit % 8)
	}
	out := append([]byte(nil), b[:field]...)
	out = append(out, mptExtensionField(".FSM", uint16(len(flags)), flags)...)
	out = append(out, b[field+21:bounds.serializedStart]...)
	tail := len(out)
	out = append(out, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(out, uint32(tail))
}

func TestOpenMPTInitialNoteMemory(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			ref, actual := mptSharedEffectRenderForDuration(t, mptInitialNoteFixture(t, enabled), 100)
			var refEnergy, actualEnergy float64
			for i := range ref {
				refEnergy += float64(ref[i]) * float64(ref[i])
				actualEnergy += float64(actual[i]) * float64(actual[i])
			}
			if enabled {
				assertMPTPCMCorrelation(t, ref, actual, 10, 90)
			} else if math.IsNaN(refEnergy) || refEnergy > 1e-8 || actualEnergy > 1e-8 {
				t.Fatal("disabled initial note memory should not trigger a note")
			}
		})
	}
}

func TestMPTInitialNoteMemoryVersion(t *testing.T) {
	for _, tt := range []struct {
		version uint32
		enabled bool
	}{
		{0x01300000, false}, {0x01310000, true}, {0x01310024, false},
		{0x01310025, true}, {0x01310400, true},
	} {
		b := mptInitialNoteFixture(t, true)
		binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], tt.version)
		loaded, err := loadMPTM(b)
		if err != nil {
			t.Fatal(err)
		}
		w := NewTracker(loaded.Assets, referenceRate, 256)
		if got := w.TrackMemories[0].LastNote == 1; got != tt.enabled {
			t.Errorf("version %#x: initial note=%d, enabled=%v", tt.version, w.TrackMemories[0].LastNote, tt.enabled)
		}
	}
}

func TestOpenMPTInitialNoteMemoryAfterSpecialNote(t *testing.T) {
	for _, note := range []byte{254, 255} {
		b := mptInitialNoteFixture(t, true)
		pattern := int(binary.LittleEndian.Uint32(b[202:]))
		clear(b[pattern+8 : pattern+8+int(binary.LittleEndian.Uint16(b[pattern:]))])
		copy(b[pattern+8:], []byte{0x81, 1, note, 0, 0x81, 2, 1, 0})
		ref, actual := mptSharedEffectRenderForDuration(t, b, 100)
		refLevel, actualLevel := channelRMS(ref, 50, 90, 0), channelRMS(actual, 50, 90, 0)
		if math.Abs(refLevel-actualLevel) > .001 {
			t.Errorf("special note %d: OpenMPT RMS %g, tracker RMS %g", note, refLevel, actualLevel)
		}
	}
}

func TestOpenMPTInitialNoteMemoryInstrumentMode(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		flags := make([]byte, 16)
		copy(flags, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7})
		if enabled {
			flags[mptITInitialNoteMemoryBit/8] |= 1 << (mptITInitialNoteMemoryBit % 8)
		}
		b := oplEnvelopeEndFixture(t, 0x01310400, flags, nil)
		b[30], b[44], b[50] = 8, b[44]|8, 2
		pattern := int(binary.LittleEndian.Uint32(b[210:]))
		clear(b[pattern+8 : pattern+8+int(binary.LittleEndian.Uint16(b[pattern:]))])
		copy(b[pattern+8:], []byte{0x81, 2, 2, 0})
		bounds, err := locateMPTExtensions(b)
		if err != nil {
			t.Fatal(err)
		}
		out := append([]byte(nil), b[:bounds.serializedStart]...)
		for _, field := range []struct {
			code string
			data []byte
		}{
			{"...C", []byte{1, 0}}, {"..MT", []byte{0}}, {".MMP", []byte{4, 0, 0, 0}},
			{".APS", []byte{64, 0, 0, 0}}, {"VTSV", []byte{64, 0, 0, 0}},
		} {
			out = append(out, mptExtensionField(field.code, uint16(len(field.data)), field.data)...)
		}
		tail := len(out)
		out = append(out, b[bounds.serializedStart:bounds.serializedEnd]...)
		out = binary.LittleEndian.AppendUint32(out, uint32(tail))
		ref, actual := mptSharedEffectRenderForDuration(t, out, 100)
		var refEnergy, actualEnergy float64
		for i := range ref {
			refEnergy += float64(ref[i]) * float64(ref[i])
			actualEnergy += float64(actual[i]) * float64(actual[i])
		}
		if enabled {
			assertMPTPCMCorrelation(t, ref, actual, 10, 90)
		} else if refEnergy > 1e-8 || actualEnergy > 1e-8 {
			t.Fatalf("disabled: reference energy %g, actual %g", refEnergy, actualEnergy)
		}
	}
}
