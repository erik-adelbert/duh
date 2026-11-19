package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func mptExtraChannelsFixture(t *testing.T, channel byte, settings []byte) []byte {
	t.Helper()
	b := mptmPlaybackFixture()
	b[50] = 2
	b[bytesIndex(t, b, "..MT")+6] = 0
	binary.LittleEndian.PutUint16(b[bytesIndex(t, b, "...C")+6:], uint16(channel))
	pattern := int(binary.LittleEndian.Uint32(b[202:]))
	clear(b[pattern+8 : pattern+8+int(binary.LittleEndian.Uint16(b[pattern:]))])
	copy(b[pattern+8:], []byte{channel | 0x80, 3, 60, 1, 0})
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	out := append([]byte(nil), b[:bounds.serializedStart]...)
	if settings != nil {
		out = append(out, mptExtensionField("SnhC", uint16(len(settings)), settings)...)
	}
	tail := len(out)
	out = append(out, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(out, uint32(tail))
}

func TestMPTExtraChannelFieldValidation(t *testing.T) {
	for _, tt := range []struct {
		size int
		ok   bool
	}{{0, true}, {2, true}, {126, true}, {1, false}, {127, false}, {128, false}} {
		field := append([]byte("STPM"), mptExtensionField("SnhC", uint16(tt.size), make([]byte, tt.size))...)
		ext, err := parseMPTExtensions(field, 0)
		if (err == nil) != tt.ok || err == nil && len(ext.extraChannelSettings) != tt.size {
			t.Errorf("size %d: error %v, parsed %d", tt.size, err, len(ext.extraChannelSettings))
		}
	}
}

func TestOpenMPTExtraChannels(t *testing.T) {
	for _, tt := range []struct {
		name     string
		channel  byte
		pan, vol byte
	}{
		{"channel 65 left", 65, 0, 64},
		{"channel 65 right", 65, 64, 32},
		{"channel 127 center", 127, 32, 64},
		{"channel 127 surround", 127, 100, 64},
		{"channel 127 muted", 127, 0x80 | 32, 64},
		{"channel 127 default", 127, 0xff, 0},
		{"channel 65 unusual pan", 65, 90, 64},
	} {
		t.Run(tt.name, func(t *testing.T) {
			settings := make([]byte, int(tt.channel-64)*2)
			for i := 0; i < len(settings); i += 2 {
				settings[i], settings[i+1] = 32, 64
			}
			settings[len(settings)-2], settings[len(settings)-1] = tt.pan, tt.vol
			b := mptExtraChannelsFixture(t, tt.channel, settings)
			ref, actual := mptSharedEffectRenderForDuration(t, b, 100)
			for channel := range 2 {
				want, got := channelRMS(ref, 30, 90, channel), channelRMS(actual, 30, 90, channel)
				if math.IsNaN(got) || math.Abs(got-want) > .002 {
					t.Errorf("side %d: OpenMPT RMS %g, tracker RMS %g", channel, want, got)
				}
			}
			if tt.pan != 0x80|32 {
				assertMPTPCMCorrelation(t, ref, actual, 30, 90)
			}
		})
	}
}

func TestMPTExtraChannelFieldExtendsCount(t *testing.T) {
	b := mptExtraChannelsFixture(t, 65, []byte{32, 64})
	binary.LittleEndian.PutUint16(b[bytesIndex(t, b, "...C")+6:], 1)
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Assets.NumTracks != 65 {
		t.Fatalf("channels = %d, want 65", loaded.Assets.NumTracks)
	}
	ref, actual := mptSharedEffectRenderForDuration(t, b, 100)
	assertMPTPCMCorrelation(t, ref, actual, 30, 90)
}

func TestMPTEmptyExtraChannelFieldExtendsCount(t *testing.T) {
	b := mptExtraChannelsFixture(t, 1, []byte{})
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Assets.NumTracks != itHeaderChannels {
		t.Fatalf("channels = %d, want %d", loaded.Assets.NumTracks, itHeaderChannels)
	}
}

func TestMPTExtraChannelDefaultsAndPatternLimit(t *testing.T) {
	for _, channel := range []byte{65, 127} {
		t.Run(fmt.Sprint(channel), func(t *testing.T) {
			loaded, err := loadMPTM(mptExtraChannelsFixture(t, channel, nil))
			if err != nil {
				t.Fatal(err)
			}
			i := int(channel) - 1
			if loaded.Assets.Panning[i] != 0 || loaded.Assets.ITChannelVolume[i] != 64 || loaded.Assets.ITMuted[i] {
				t.Fatalf("channel %d defaults: pan %g, volume %d, muted %v", channel,
					loaded.Assets.Panning[i], loaded.Assets.ITChannelVolume[i], loaded.Assets.ITMuted[i])
			}
			if len(loaded.Assets.Patterns[0].Rows[0]) != int(channel) {
				t.Fatal("pattern width does not match required channels")
			}
		})
	}
	if _, err := unpackITPattern([]byte{0xc1, 3, 60, 1, 0}, 1); err == nil {
		t.Fatal("ordinary IT accepted a channel beyond its 64-channel limit")
	}
	for _, channels := range []int{0, MaxTracks + 1} {
		if _, err := unpackITPatternChannels(nil, 1, true, channels); err == nil {
			t.Errorf("accepted invalid pattern width %d", channels)
		}
	}
}
