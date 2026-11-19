package mod

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestITCompatibilityProfile(t *testing.T) {
	for _, tt := range []struct {
		name     string
		legacy   bool
		extended bool
	}{
		{"IT", false, false},
		{"IT extended filter", false, true},
		{"ModPlug", true, false},
		{"ModPlug extended filter", true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := testIT()
			if tt.legacy {
				binary.LittleEndian.PutUint16(b[40:], 0x217)
				binary.LittleEndian.PutUint16(b[42:], 0x200)
			}
			if tt.extended {
				b[45] |= 0x10
			}
			loaded, err := loadIT(b)
			if err != nil {
				t.Fatal(err)
			}
			assets := loaded.Assets
			if assets.ITCompatibility.LegacyModPlug != tt.legacy || assets.ITCompatibility.ExtendedFilter != tt.extended ||
				assets.ITLegacyPitchPan != tt.legacy || assets.ITExtendedFilter != tt.extended {
				t.Fatalf("compatibility profile: %+v", assets.ITCompatibility)
			}
		})
	}
	legacy := NewTracker(SongAssets{ITLegacyPitchPan: true, ITExtendedFilter: true}, 48000, 1)
	if !legacy.Assets.ITCompatibility.LegacyModPlug || !legacy.Assets.ITCompatibility.ExtendedFilter {
		t.Fatal("legacy SongAssets fields were not honored")
	}
}

func TestMapITEffect(t *testing.T) {
	for _, tt := range []struct {
		effect, arg, mapped, value uint8
		err                        string
	}{
		{0, 7, 0, 0, ""},
		{1, 6, 0x0f, 6, ""},
		{2, 8, 0x0b, 8, ""},
		{3, 10, 0x0d, 10, ""},
		{20, 125, 0x14, 125, ""},
		{20, 31, 0, 0, "unsupported IT tempo slide T1F"},
		{24, 0, 0x08, 0, ""},
		{24, 255, 0x08, 255, ""},
	} {
		cmd, arg, err := mapITEffect(tt.effect, tt.arg)
		if tt.err != "" {
			if err == nil || !strings.Contains(err.Error(), tt.err) {
				t.Fatalf("effect %d: error %v, want %q", tt.effect, err, tt.err)
			}
		} else if err != nil || cmd != tt.mapped || arg != tt.value {
			t.Fatalf("effect %d: got (%d, %d, %v), want (%d, %d)", tt.effect, cmd, arg, err, tt.mapped, tt.value)
		}
	}
}
