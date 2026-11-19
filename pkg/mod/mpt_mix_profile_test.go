package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestMPTMixProfile(t *testing.T) {
	for _, tt := range []struct {
		mix, sample, plugin uint32
		pcm, opl            float32
		valid               bool
		soft                bool
	}{
		{4, 64, 64, .5, 8225.0 / 4096, true, false},
		{4, 0, 128, 0, 16450.0 / 4096, true, false},
		{4, 128, 0, 1, 0, true, false},
		{3, 48, 48, .375, 6169.0 / 4096, true, true},
		{2, 64, 64, 0, 0, false, false},
		{4, 129, 64, 0, 0, false, false},
		{4, 64, 129, 0, 0, false, false},
	} {
		profile, err := projectMPTMixProfile(tt.mix, tt.sample, tt.plugin)
		if (err == nil) != tt.valid || tt.valid && (profile.pcmGain != tt.pcm || profile.oplGain != tt.opl || profile.softPanning != tt.soft) {
			t.Errorf("%+v: profile %+v, error %v", tt, profile, err)
		}
	}
}

func mptMixFixture(t *testing.T, opl, mixed bool, sample, plugin uint32, patch oplPatch) ([]byte, SongAssets) {
	return mptMixGlobalFixture(t, opl, mixed, sample, plugin, patch, 128, nil)
}

func mptMixGlobalFixture(t *testing.T, opl, mixed bool, sample, plugin uint32, patch oplPatch, global byte, changes []byte) ([]byte, SongAssets) {
	t.Helper()
	b, assets := transitionFixture(opl, false, false)
	b = b[:len(b)-11]
	copy(b[len(b)-12:], patch[:])
	assets.Samples[1].opl = &patch
	instrument := byte(1)
	if opl {
		instrument = 2
	}
	packed := []byte{0x81, 3, 60, instrument, 0}
	if mixed {
		packed = []byte{0x81, 3, 60, 1, 0x82, 3, 60, 2, 0}
		assets.NumTracks = 2
		assets.ITChannelVolume[1] = 64
	}
	for _, value := range changes {
		packed = append(packed, 0x81, 8, 22, value, 0)
	}
	packed = append(packed, make([]byte, 63-len(changes))...)
	b[48], assets.ITGlobalVolume = global, global
	binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
	copy(b[344:], packed)
	binary.LittleEndian.PutUint32(b[202:], uint32(len(b)))
	b = append(b, b[336:344+len(packed)]...)
	b = append(b, []byte("STPM")...)
	for _, field := range []struct {
		code  string
		value uint32
	}{{".MMP", 4}, {".APS", sample}, {"VTSV", plugin}, {"VWSL", 0x01280100}} {
		b = append(b, mptExtensionField(field.code, 4, binary.LittleEndian.AppendUint32(nil, field.value))...)
	}
	tail := len(b)
	b = append(b, mptTestSequences([]mptTestEntry{{"l", []byte{1, 0}}, {"a", []byte{0, 0}}})...)
	b = binary.LittleEndian.AppendUint32(b, uint32(tail))
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	ext, err := parseMPTExtensions(b[bounds.propertiesStart:bounds.serializedStart], 0)
	if err != nil || ext.mixLevels == nil || ext.samplePreamp == nil || ext.pluginVolume == nil {
		t.Fatalf("fixture extensions: %+v, error %v", ext, err)
	}
	profile, err := projectMPTMixProfile(*ext.mixLevels, *ext.samplePreamp, *ext.pluginVolume)
	if err != nil {
		t.Fatal(err)
	}
	assets.ITCompatibility.mptMixProfile = &profile
	rows := make([][]ModCommand, 64)
	for i := range rows {
		rows[i] = make([]ModCommand, assets.NumTracks)
	}
	rows[0][0] = ModCommand{Note: 61, Instrument: instrument}
	if mixed {
		rows[0][0] = ModCommand{Note: 61, Instrument: 1}
		rows[0][1] = ModCommand{Note: 61, Instrument: 2}
	}
	for i, value := range changes {
		rows[i+1][0] = ModCommand{EffectCmd: 0x17, EffectArg: value}
	}
	assets.OrderList, assets.Patterns = []int{0}, []PatternAsset{{Rows: rows}}
	return b, assets
}

func renderMPTMixFixture(t *testing.T, assets SongAssets, frames int) []float32 {
	t.Helper()
	w := NewTracker(assets, referenceRate, 960)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 3, 125, -1
	out := make([]float32, frames*2)
	for i := 0; i < frames; {
		n := min(960, frames-i)
		if w.Read(out[i*2:(i+n)*2], n) != n {
			t.Fatal("short tracker render")
		}
		i += n
	}
	return out
}

func TestOpenMPTMixProfile(t *testing.T) {
	for _, opl := range []bool{false, true} {
		for _, volumes := range [][2]uint32{{64, 64}, {32, 64}, {128, 64}, {64, 32}, {64, 128}, {0, 64}, {64, 0}} {
			t.Run(fmt.Sprintf("opl=%v/sample=%d/plugin=%d", opl, volumes[0], volumes[1]), func(t *testing.T) {
				fixture := func(sample, plugin uint32) ([]byte, SongAssets) {
					return mptMixFixture(t, opl, false, sample, plugin, testOPLPatch)
				}
				module, assets := fixture(volumes[0], volumes[1])
				base, baseAssets := fixture(64, 64)
				ref, refBase := referenceTransition(t, module), referenceTransition(t, base)
				actual, actualBase := renderMPTMixFixture(t, assets, 5760), renderMPTMixFixture(t, baseAssets, 5760)
				refLevel, baseRefLevel := rmsWindow(ref, 40, 100), rmsWindow(refBase, 40, 100)
				actualLevel, baseActualLevel := rmsWindow(actual, 40, 100), rmsWindow(actualBase, 40, 100)
				if baseRefLevel < 1e-5 || baseActualLevel < 1e-5 {
					t.Fatal("silent comparison baseline")
				}

				want, got := refLevel/baseRefLevel, actualLevel/baseActualLevel
				expected := float64(volumes[0]) / 64
				if opl {
					expected = float64(volumes[1]) / 64
				}
				if math.Abs(want-expected) > .035 {
					t.Errorf("reference gain %.5f, expected %.5f; fixture may not apply extensions", want, expected)
				}
				if math.Abs(got-want) > .035 {
					t.Errorf("relative gain: OpenMPT %.5f, tracker %.5f", want, got)
				}
			})
		}
	}
}

func TestOpenMPTMixAbsolute(t *testing.T) {
	patches := []oplPatch{testOPLPatch, {0x21, 0x21, 0x1f, 0x08, 0xf0, 0xf0, 0x0f, 0x0f, 0, 0, 1, 0}}
	for i, patch := range patches {
		for _, source := range []string{"pcm", "opl", "mixed"} {
			for _, preamp := range [][2]uint32{{64, 64}, {32, 128}, {128, 32}} {
				t.Run(fmt.Sprintf("patch=%d/source=%s/sample=%d/plugin=%d", i, source, preamp[0], preamp[1]), func(t *testing.T) {
					b, assets := mptMixFixture(t, source == "opl", source == "mixed", preamp[0], preamp[1], patch)
					ref, actual := referenceTransition(t, b), renderMPTMixFixture(t, assets, 5760)
					refLevel, level := rmsWindow(ref, 40, 100), rmsWindow(actual, 40, 100)
					if refLevel < .001 || level < .001 || math.Abs(level/refLevel-1) > .03 {
						t.Errorf("RMS: OpenMPT %.6f, tracker %.6f", refLevel, level)
					}
				})
			}
		}
	}
}
