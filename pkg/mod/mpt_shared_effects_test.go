package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func mptSharedEffectFixture(t *testing.T, rows []byte, flags mptSavedPlayBehaviours) []byte {
	t.Helper()
	b := mptmPlaybackFixture()
	b[50], b[48] = 2, 64
	b[bytesIndex(t, b, "..MT")+6] = 0
	flagOffset := bytesIndex(t, b, ".FSM") + 6
	for i := range 7 {
		b[flagOffset+i] = byte(uint64(flags) >> (8 * i))
	}
	pattern := int(binary.LittleEndian.Uint32(b[202:]))
	packed := append(append([]byte(nil), rows...), make([]byte, 64)...)
	sampleData := int(binary.LittleEndian.Uint32(b[256+72:]))
	if pattern+8+len(packed) > sampleData {
		t.Fatal("shared-effect pattern overlaps sample data")
	}
	binary.LittleEndian.PutUint16(b[pattern:], uint16(len(packed)))
	copy(b[pattern+8:], packed)
	return b
}

func mptSharedEffectRender(t *testing.T, b []byte) ([]float32, []float32) {
	return mptSharedEffectRenderForDuration(t, b, 120)
}

func mptSharedEffectRenderForDuration(t *testing.T, b []byte, milliseconds int) ([]float32, []float32) {
	t.Helper()
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	ref := referenceMPTMForDuration(t, b, milliseconds)
	w := NewTracker(loaded.Assets, referenceRate, 256)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	actual := make([]float32, len(ref))
	for frame := 0; frame < len(ref)/2; frame += 256 {
		n := min(256, len(ref)/2-frame)
		if got := w.Read(actual[frame*2:(frame+n)*2], n); got != n {
			t.Fatalf("short render at frame %d: %d", frame, got)
		}
	}
	return ref, actual
}

func assertMPTPCMCorrelation(t *testing.T, ref, actual []float32, start, end int) {
	t.Helper()
	var cross, r2, a2 float64
	for i := start * 96; i < end*96; i++ {
		r, a := float64(ref[i]), float64(actual[i])
		cross, r2, a2 = cross+r*a, r2+r*r, a2+a*a
	}
	correlation := cross / math.Sqrt(r2*a2)
	if r2 < .001 || a2 < .001 || math.IsNaN(correlation) || correlation < .98 {
		t.Fatalf("correlation %g energy %g/%g", correlation, r2, a2)
	}
}

func assertMPTMVolumeRatio(t *testing.T, ref, actual, baseRef, baseActual []float32, tick int, tolerance float64) {
	t.Helper()
	ms := tick*20 + 19
	want := channelRMS(ref, ms, ms+1, 0) / channelRMS(baseRef, ms, ms+1, 0)
	got := channelRMS(actual, ms, ms+1, 0) / channelRMS(baseActual, ms, ms+1, 0)
	if math.IsNaN(want) || math.IsNaN(got) || math.IsInf(want, 0) || math.IsInf(got, 0) || math.Abs(got-want) > tolerance {
		t.Errorf("tick %d: volume ratio OpenMPT %.3f tracker %.3f", tick, want, got)
	}
}

func TestOpenMPTMSharedEffects(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
		pan  bool
	}{
		{"K vibrato volume", []byte{0x81, 15, 60, 1, 48, 8, 0x84, 0, 0x81, 8, 11, 0x02, 0, 0x81, 8, 11, 0, 0}, false},
		{"K fine slide", []byte{0x81, 15, 60, 1, 48, 8, 0x44, 0, 0x81, 8, 11, 0xf2, 0, 0x81, 8, 11, 0, 0}, false},
		{"L portamento volume", []byte{0x81, 15, 60, 1, 48, 7, 16, 0, 0x81, 9, 64, 12, 0x02, 0, 0x81, 8, 12, 0, 0}, false},
		{"L fine slide", []byte{0x81, 15, 60, 1, 48, 7, 16, 0, 0x81, 9, 64, 12, 0x2f, 0, 0x81, 8, 12, 0, 0}, false},
		{"N channel slide", []byte{0x81, 11, 60, 1, 14, 0x03, 0, 0x81, 8, 14, 0, 0}, false},
		{"P pan slide", []byte{0x81, 11, 60, 1, 16, 0x04, 0, 0x81, 8, 16, 0, 0}, true},
		{"U fine vibrato", []byte{0x81, 11, 60, 1, 21, 0x8f, 0, 0x81, 8, 21, 0, 0}, false},
		{"H to U memory", []byte{0x81, 11, 60, 1, 8, 0x47, 0, 0x81, 8, 21, 0x20, 0, 0x81, 8, 21, 0x04, 0}, false},
		{"U to H memory", []byte{0x81, 11, 60, 1, 21, 0x47, 0, 0x81, 8, 8, 0x20, 0, 0x81, 8, 8, 0x04, 0}, false},
		{"H vibrato", []byte{0x81, 11, 60, 1, 8, 0x8f, 0, 0x81, 8, 8, 0, 0}, false},
		{"W global slide", []byte{0x81, 11, 60, 1, 23, 0x03, 0, 0x81, 8, 23, 0, 0}, false},
		{"N fine slide", []byte{0x81, 11, 60, 1, 14, 0xf3, 0, 0x81, 8, 14, 0, 0}, false},
		{"P fine slide", []byte{0x81, 11, 60, 1, 16, 0xf3, 0, 0x81, 8, 16, 0, 0}, true},
		{"W fine slide", []byte{0x81, 11, 60, 1, 23, 0xf3, 0, 0x81, 8, 23, 0, 0}, false},
		{"N mixed slide", []byte{0x81, 11, 60, 1, 14, 0x23, 0, 0x81, 8, 14, 0, 0}, false},
		{"P mixed slide", []byte{0x81, 11, 60, 1, 16, 0x23, 0, 0x81, 8, 16, 0, 0}, true},
		{"W mixed slide", []byte{0x81, 11, 60, 1, 23, 0x23, 0, 0x81, 8, 23, 0, 0}, false},
	}

	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	profiles := []struct {
		name       string
		clear      mptSavedPlayBehaviours
		oldEffects bool
		nonlinear  bool
		version    uint32
	}{
		{name: "default"},
		{name: "legacy-wave", clear: 1 << mptITVibratoTremoloPanbrello},
		{name: "old-effects", oldEffects: true},
		{name: "old-effects-legacy-wave", oldEffects: true, clear: 1 << mptITVibratoTremoloPanbrello},
		{name: "nonlinear", nonlinear: true},
		{name: "period-pitch", nonlinear: true, clear: 1 << mptPeriodsAreHertz},
		{name: "version-1.26", version: 0x01260000},
		{name: "version-1.32", version: 0x01321200},
		{name: "version-1.32-nonlinear-hertz", version: 0x01321200, nonlinear: true},
		{name: "version-1.32-nonlinear-period", version: 0x01321200, nonlinear: true, clear: 1 << mptPeriodsAreHertz},
	}
	for _, tt := range tests {
		for _, profile := range profiles {
			t.Run(fmt.Sprintf("%s/%s", tt.name, profile.name), func(t *testing.T) {
				fixture := func(rows []byte) []byte {
					b := mptSharedEffectFixture(t, rows, flags&^profile.clear)
					if profile.oldEffects {
						b[44] |= 0x10
					}
					if profile.nonlinear {
						b[44] &^= 8
					}
					if profile.version != 0 {
						binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], profile.version)
					}
					return b
				}
				ref, actual := mptSharedEffectRender(t, fixture(tt.rows))
				baseRef, baseActual := mptSharedEffectRender(t, fixture([]byte{0x81, 7, 60, 1, 48, 0}))
				for tick := range 6 {
					ms := tick*20 + 19
					if tt.pan {
						balance := func(pcm []float32) float64 {
							l, r := channelRMS(pcm, ms, ms+1, 0), channelRMS(pcm, ms, ms+1, 1)
							return (r - l) / (r + l)
						}
						want, got := balance(ref), balance(actual)
						if math.IsNaN(want) || math.IsNaN(got) || math.IsInf(want, 0) || math.IsInf(got, 0) || math.Abs(got-want) > .05 {
							t.Errorf("tick %d: balance OpenMPT %.3f tracker %.3f", tick, want, got)
						}
					} else {
						assertMPTMVolumeRatio(t, ref, actual, baseRef, baseActual, tick, .05)
						wantFreq, gotFreq := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
						if math.Abs(gotFreq/wantFreq-1) > .004 {
							t.Errorf("tick %d: frequency OpenMPT %.2f tracker %.2f", tick, wantFreq, gotFreq)
						}
					}
				}
			})
		}
	}
}
