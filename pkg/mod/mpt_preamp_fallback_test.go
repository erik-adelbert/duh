package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTSamplePreampHeaderFallback(t *testing.T) {
	for _, header := range []byte{32, 64, 128, 200} {
		b := mptmPlaybackFixture()
		b[49], b[50] = header, 2
		b[bytesIndex(t, b, "..MT")+6] = 0
		bounds, err := locateMPTExtensions(b)
		if err != nil {
			t.Fatal(err)
		}

		field := bytesIndex(t, b, ".APS")
		out := append([]byte(nil), b[:field]...)
		out = append(out, b[field+10:bounds.serializedStart]...)
		tail := len(out)
		out = append(out, b[bounds.serializedStart:bounds.serializedEnd]...)
		out = binary.LittleEndian.AppendUint32(out, uint32(tail))
		loaded, err := loadMPTM(out)
		if err != nil {
			t.Fatal(err)
		}
		want := float32(min(header, 128)) / 128
		if got := loaded.Assets.ITCompatibility.mptMixProfile.pcmGain; got != want {
			t.Fatalf("header %d: gain %g, want %g", header, got, want)
		}
		ref, actual := mptSharedEffectRenderForDuration(t, out, 100)
		wantLevel, gotLevel := channelRMS(ref, 30, 90, 0), channelRMS(actual, 30, 90, 0)
		if math.Abs(gotLevel-wantLevel) > .002 {
			t.Errorf("header %d: OpenMPT RMS %g, tracker RMS %g", header, wantLevel, gotLevel)
		}
	}
}

func TestMPTSamplePreampExtensionOverridesHeader(t *testing.T) {
	b := mptmPlaybackFixture()
	b[49] = 32
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Assets.ITCompatibility.mptMixProfile.pcmGain; got != .5 {
		t.Fatalf("gain = %g, want .5 from the extension", got)
	}
}
