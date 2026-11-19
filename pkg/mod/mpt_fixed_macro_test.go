package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestMPTMDefaultMacroMappingAndValidation(t *testing.T) {
	for arg := 0; arg <= 255; arg++ {
		p, err := unpackITPatternMode([]byte{0x81, 8, 26, byte(arg), 0}, 1, true)
		if err != nil || p.Rows[0][0].EffectCmd != 0x19 || p.Rows[0][0].EffectArg != byte(arg) {
			t.Fatalf("Z%02X: pattern %+v, error %v", arg, p, err)
		}
	}
	for arg := 0xf0; arg <= 0xff; arg++ {
		p, err := unpackITPatternMode([]byte{0x81, 8, 19, byte(arg), 0}, 1, true)
		if err != nil || p.Rows[0][0].EffectCmd != 0x15 || p.Rows[0][0].EffectArg != byte(arg) {
			t.Fatalf("S%02X: pattern %+v, error %v", arg, p, err)
		}
	}
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, row := range [][]byte{{0x81, 8, 19, 0xf0, 0}, {0x81, 8, 26, 0x80, 0}} {
		b := mptSharedEffectFixture(t, row, flags)
		binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01250000)
		if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "pre-1.26") {
			t.Fatalf("old profile accepted row %v: %v", row, err)
		}
	}
	for _, row := range [][]byte{{0x81, 8, 26, 0x80, 0}, {0x81, 8, 19, 0xf1, 0, 0x81, 8, 26, 1, 0}} {
		b := mptSharedEffectFixture(t, row, flags)
		b[46] |= 8
		if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "embedded MIDI macros") {
			t.Fatalf("embedded configuration accepted row %v: %v", row, err)
		}
	}
}

func TestMPTMDefaultMacroState(t *testing.T) {
	profile, err := projectMPTMixProfile(4, 64, 64)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(SongAssets{IT: true, NumTracks: 2, ITCompatibility: ITCompatibility{mptMixProfile: &profile},
		Samples: []SampleAsset{{Data: make([]float32, 32), C5Speed: 8363, Volume: 64}}}, 48000, 64)
	special := func(track int, arg byte) {
		w.PendingCmds[track] = ModCommand{EffectCmd: 0x15, EffectArg: arg}
		VolumeEffectSystem(w)
	}
	macro := func(track int, arg byte) {
		w.PendingCmds[track] = ModCommand{EffectCmd: 0x19, EffectArg: arg}
		MPTCutoffMacroSystem(w)
	}
	special(0, 0xf1)
	macro(0, 20)
	macro(1, 40)
	if w.TrackCursors[0].ITFilterCutoff != 127 || w.TrackCursors[1].ITFilterCutoff != 40 {
		t.Fatal("SF1 or channel-local selection failed")
	}
	macro(0, 0x8f)
	if w.TrackCursors[0].ITFilterResonance != 120 || w.TrackCursors[1].ITFilterResonance != 0 {
		t.Fatal("fixed resonance was affected by SF1 or leaked across channels")
	}
	macro(0, 0xff)
	if w.TrackCursors[0].ITFilterResonance != 120 {
		t.Fatal("empty fixed macro altered resonance")
	}
	special(0, 0xf0)
	special(0, 0)
	if w.TrackMemories[0].mptActiveMacro != 0 || w.TrackMemories[0].ITLastSpecial != 0xf0 {
		t.Fatal("S00 failed to recall SF0")
	}
	macro(0, 60)
	if !w.startSampleVoice(0, ModCommand{Note: 61}, 0, 61, 0) {
		t.Fatal("voice allocation failed")
	}
	v := w.TrackCursors[0].ActiveVoice
	if w.VoiceLives[v].ITFilterCutoff != 60 || w.VoiceLives[v].ITFilterResonance != 120 {
		t.Fatalf("sample-mode filter parameters not inherited: %+v", w.VoiceLives[v])
	}
	w.VoiceFilters[v].Y1, w.VoiceRightFilters[v].Y1 = .25, -.25
	macro(0, 0x84)
	if w.VoiceLives[v].ITFilterCutoff != 60 || w.VoiceLives[v].ITFilterResonance != 32 ||
		w.VoiceFilters[v].Y1 != .25 || w.VoiceRightFilters[v].Y1 != -.25 {
		t.Fatal("fixed resonance did not preserve cutoff and filter history")
	}
}

func TestOpenMPTMDefaultFixedMacros(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, opl := range []bool{false, true} {
		for _, arg := range []byte{0x80, 0x84, 0x8a, 0x8f, 0x90, 0xff} {
			t.Run(fmt.Sprintf("opl=%t/Z%02X", opl, arg), func(t *testing.T) {
				fixture := func(value byte) []byte {
					sample := byte(1)
					if opl {
						sample = 2
					}
					return mptSharedEffectFixture(t, []byte{
						0x81, 11, 60, sample, 26, 64, 0,
						0x81, 8, 26, value, 0,
						0, 0,
					}, flags)
				}
				ref, actual := mptSharedEffectRenderForDuration(t, fixture(arg), 150)
				baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture(0x80), 150)
				for _, win := range [][2]int{{50, 70}, {90, 110}} {
					wantBase, gotBase := rmsWindow(baseRef, win[0], win[1]), rmsWindow(baseActual, win[0], win[1])
					if wantBase < .00001 || gotBase < .00001 {
						t.Fatal("silent comparison baseline")
					}
					want := rmsWindow(ref, win[0], win[1]) / wantBase
					got := rmsWindow(actual, win[0], win[1]) / gotBase
					if math.IsNaN(want) || math.IsNaN(got) || math.IsInf(want, 0) || math.IsInf(got, 0) || math.Abs(want-got) > .09 {
						t.Errorf("%d-%d ms: OpenMPT ratio %.4f, tracker %.4f", win[0], win[1], want, got)
					}
				}
			})
		}
	}
}

func TestOpenMPTMDefaultMacroSelection(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, opl := range []bool{false, true} {
		for _, slot := range []byte{1, 7, 15} {
			t.Run(fmt.Sprintf("opl=%t/SF%X", opl, slot), func(t *testing.T) {
				sample := byte(1)
				if opl {
					sample = 2
				}
				fixture := func(value byte) []byte {
					return mptSharedEffectFixture(t, []byte{
						0x81, 3, 60, sample, 0,
						0x81, 8, 19, 0xf0 | slot, 0,
						0x81, 8, 26, value, 0,
						0, 0,
					}, flags)
				}
				ref, actual := mptSharedEffectRenderForDuration(t, fixture(0), 180)
				baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture(127), 180)
				for name, pair := range map[string][2][]float32{"OpenMPT": {ref, baseRef}, "tracker": {actual, baseActual}} {
					base := rmsWindow(pair[1], 100, 140)
					if base < .00001 || math.Abs(rmsWindow(pair[0], 100, 140)/base-1) > .01 {
						t.Errorf("%s: empty SF%X changed sound", name, slot)
					}
				}
			})
		}
	}
}

func TestOpenMPTMFixedMacroInstrument(t *testing.T) {
	for _, opl := range []bool{false, true} {
		for _, envelope := range []bool{false, true} {
			t.Run(fmt.Sprintf("opl=%t/envelope=%t", opl, envelope), func(t *testing.T) {
				fixture := func(resonance byte) ([]byte, []float32) {
					b := oplEnvelopeEndFixture(t, 0x01280100, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}, []byte{0})
					b[30], b[44], b[50] = 8, b[44]|8, 2
					instrument := 1
					if !opl {
						instrument = 2
					}
					ptr := int(binary.LittleEndian.Uint32(b[194+4*(instrument-1):]))
					b[ptr+304] = 0
					cutoff := byte(64)
					if opl {
						cutoff = 127
					}
					b[ptr+58] = 0x80 | cutoff
					b[ptr+59] = 0
					if envelope {
						setTestEnvelope(b[ptr:ptr+554], 468, 0x81, []int8{-16, 16}, []uint16{0, 4})
					}
					pattern := int(binary.LittleEndian.Uint32(b[210:]))
					length := int(binary.LittleEndian.Uint16(b[pattern:]))
					binary.LittleEndian.PutUint16(b[pattern+2:], 32)
					clear(b[pattern+8 : pattern+8+length])
					copy(b[pattern+8:], []byte{0x81, 11, 60, byte(instrument), 26, resonance, 0})
					return renderMPTFilterInstrumentFixture(t, b, []byte{0, 0})
				}
				b, actual := fixture(0x8c)
				base, actualBase := fixture(0x80)
				ref, refBase := referenceTransition(t, b), referenceTransition(t, base)
				wantBase, gotBase := rmsWindow(refBase, 40, 100), rmsWindow(actualBase, 40, 100)
				if wantBase < .00001 || gotBase < .00001 {
					t.Fatalf("silent comparison baseline: OpenMPT %g tracker %g", wantBase, gotBase)
				}
				want, got := rmsWindow(ref, 40, 100)/wantBase, rmsWindow(actual, 40, 100)/gotBase
				if math.IsNaN(want) || math.IsNaN(got) || math.IsInf(want, 0) || math.IsInf(got, 0) || math.Abs(want-got) > .09 {
					t.Fatalf("relative RMS: OpenMPT %g tracker %g", want, got)
				}
			})
		}
	}
}

func TestOpenMPTMFixedMacroProfiles(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, version := range []uint32{0x01260000, 0x01280100, 0x01321200} {
		for _, old := range []bool{false, true} {
			t.Run(fmt.Sprintf("%08x/old=%t", version, old), func(t *testing.T) {
				fixture := func(resonance byte) []byte {
					b := mptSharedEffectFixture(t, []byte{
						0x81, 11, 60, 1, 26, 64, 0,
						0x81, 8, 26, resonance, 0,
					}, flags)
					if old {
						b[44] |= 0x10
					}
					binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], version)
					return b
				}
				ref, actual := mptSharedEffectRenderForDuration(t, fixture(0x8c), 140)
				baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture(0x80), 140)
				wantBase, gotBase := rmsWindow(baseRef, 60, 100), rmsWindow(baseActual, 60, 100)
				if wantBase < .00001 || gotBase < .00001 {
					t.Fatal("silent comparison baseline")
				}
				want, got := rmsWindow(ref, 60, 100)/wantBase, rmsWindow(actual, 60, 100)/gotBase
				if math.IsNaN(want) || math.IsNaN(got) || math.IsInf(want, 0) || math.IsInf(got, 0) || math.Abs(want-got) > .09 {
					t.Fatalf("relative RMS: OpenMPT %g tracker %g", want, got)
				}
			})
		}
	}
}

func TestOpenMPTMFixedMacroBeforeNote(t *testing.T) {
	for _, allFlags := range []bool{false, true} {
		for _, opl := range []bool{false, true} {
			t.Run(fmt.Sprintf("flags=%t/opl=%t", allFlags, opl), func(t *testing.T) {
				flags := mptSavedPlayBehaviours(0)
				if allFlags {
					flags = mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
				}
				sample := byte(1)
				if opl {
					sample = 2
				}
				fixture := func(resonance byte) []byte {
					return mptSharedEffectFixture(t, []byte{
						0x81, 8, 26, 64, 0,
						0x81, 8, 26, resonance, 0,
						0x81, 3, 60, sample, 0,
						0, 0,
					}, flags)
				}
				if !allFlags {
					if _, err := loadMPTM(fixture(0x8f)); err == nil {
						t.Fatal("legacy filter behavior accepted for fixed resonance")
					}
					return
				}
				ref, actual := mptSharedEffectRenderForDuration(t, fixture(0x8f), 180)
				baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture(0x80), 180)
				wantBase, gotBase := rmsWindow(baseRef, 100, 140), rmsWindow(baseActual, 100, 140)
				if wantBase < .00001 || gotBase < .00001 {
					t.Fatal("silent comparison baseline")
				}
				want, got := rmsWindow(ref, 100, 140)/wantBase, rmsWindow(actual, 100, 140)/gotBase
				if math.IsNaN(want) || math.IsNaN(got) || math.IsInf(want, 0) || math.IsInf(got, 0) || math.Abs(want-got) > .09 {
					t.Fatalf("relative RMS: OpenMPT %g tracker %g", want, got)
				}
			})
		}
	}
}

func TestOpenMPTMSelectionRestoration(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	fixture := func(switchBack bool) []byte {
		selected := byte(0xf1)
		if switchBack {
			selected = 0xf0
		}
		return mptSharedEffectFixture(t, []byte{
			0x81, 3, 60, 1, 0,
			0x81, 8, 19, 0xf1, 0,
			0x81, 8, 19, selected, 0,
			0x81, 8, 19, 0, 0,
			0x81, 8, 26, 16, 0,
		}, flags)
	}
	ref, actual := mptSharedEffectRenderForDuration(t, fixture(true), 250)
	baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture(false), 250)
	wantBase, gotBase := rmsWindow(baseRef, 180, 220), rmsWindow(baseActual, 180, 220)
	if wantBase < .00001 || gotBase < .00001 {
		t.Fatal("silent comparison baseline")
	}
	want, got := rmsWindow(ref, 180, 220)/wantBase, rmsWindow(actual, 180, 220)/gotBase
	if math.IsNaN(want) || math.IsNaN(got) || math.Abs(want-got) > .06 || want > .8 {
		t.Fatalf("SF0 restoration: OpenMPT %g tracker %g", want, got)
	}
}
