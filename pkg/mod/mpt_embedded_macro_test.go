package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func embeddedMacroFixture(t *testing.T, data []byte, history int, parametered map[int]string, fixed map[int]string) []byte {
	t.Helper()
	u16 := func(off int) int { return int(binary.LittleEndian.Uint16(data[off:])) }
	start := 192 + u16(32)
	count := u16(34) + u16(36) + u16(38)
	pos := start + 4*count
	extra := make([]byte, mptMacroConfigSize)
	for slot, text := range parametered {
		offset := (mptGlobalMacroCount + slot) * mptMacroLength
		copy(extra[offset:offset+mptMacroLength], text)
	}
	for slot, text := range fixed {
		offset := (mptGlobalMacroCount + mptParameterMacroCount + slot) * mptMacroLength
		copy(extra[offset:offset+mptMacroLength], text)
	}
	if history >= 0 {
		prefix := make([]byte, 2+8*history)
		binary.LittleEndian.PutUint16(prefix, uint16(history))
		extra = append(prefix, extra...)
		data[46] |= 2
	}
	data[46] |= 8
	for i := 0; i < u16(36); i++ {
		ptr := int(binary.LittleEndian.Uint32(data[start+4*(u16(34)+i):]))
		if ptr != 0 {
			offset := binary.LittleEndian.Uint32(data[ptr+72:])
			if offset != 0 {
				binary.LittleEndian.PutUint32(data[ptr+72:], offset+uint32(len(extra)))
			}
		}
	}
	for i := range count {
		offset := binary.LittleEndian.Uint32(data[start+4*i:])
		if offset != 0 {
			binary.LittleEndian.PutUint32(data[start+4*i:], offset+uint32(len(extra)))
		}
	}
	tail := binary.LittleEndian.Uint32(data[len(data)-4:])
	binary.LittleEndian.PutUint32(data[len(data)-4:], tail+uint32(len(extra)))
	result := append([]byte(nil), data[:pos]...)
	result = append(result, extra...)
	return append(result, data[pos:]...)
}

func TestMPTEmbeddedMacroParsing(t *testing.T) {
	if mptMacroConfigSize != 4896 {
		t.Fatalf("embedded macro block size %d, want 4896", mptMacroConfigSize)
	}
	tests := []struct {
		text        string
		parametered bool
		want        mptMacro
	}{
		{"", true, mptMacro{}},
		{"F0F000z", true, mptMacro{kind: mptMacroCutoff}},
		{"F0 F0 01 z", true, mptMacro{kind: mptMacroResonance}},
		{"F0F0007F", false, mptMacro{kind: mptMacroCutoff, value: 127}},
		{"F0F0012A", false, mptMacro{kind: mptMacroResonance, value: 42}},
		{"F0F00180", false, mptMacro{kind: mptMacroUnsupported}},
		{"F0F002z", true, mptMacro{kind: mptMacroUnsupported}},
		{"Bc07z", true, mptMacro{kind: mptMacroUnsupported}},
		{"F0F000z F0F001z", true, mptMacro{kind: mptMacroUnsupported}},
		{"F0F000", true, mptMacro{kind: mptMacroUnsupported}},
		{"f0f000z", true, mptMacro{kind: mptMacroUnsupported}},
	}
	for _, tt := range tests {
		raw := make([]byte, mptMacroLength)
		copy(raw, tt.text)
		if got := parseMPTMacro(raw, tt.parametered); got != tt.want {
			t.Errorf("%q: got %+v, want %+v", tt.text, got, tt.want)
		}
	}
	raw := make([]byte, mptMacroLength)
	copy(raw, "F0F000z\x00F0F001z")
	if got := parseMPTMacro(raw, true); got != (mptMacro{kind: mptMacroCutoff}) {
		t.Fatalf("null-terminated macro: %+v", got)
	}
}

func TestMPTMacroCandidates(t *testing.T) {
	config := defaultMPTMacroConfig()
	for i, macro := range config.parametered {
		want := mptMacro{}
		if i == 0 {
			want.kind = mptMacroCutoff
		}
		if macro != want {
			t.Fatalf("SF%X: got %+v, want %+v", i, macro, want)
		}
	}
	for i, macro := range config.fixed {
		want := mptMacro{}
		if i < mptDefaultResonanceCount {
			want = mptMacro{kind: mptMacroResonance, value: byte(i * 8)}
		}
		if macro != want {
			t.Fatalf("Z%02X: got %+v, want %+v", i+0x80, macro, want)
		}
	}
	config.fixed[127] = mptMacro{kind: mptMacroCutoff, value: 127}
	for _, tt := range []struct {
		arg  byte
		size int
		last mptMacro
	}{
		{0, 16, mptMacro{}},
		{0x7f, 16, mptMacro{}},
		{0x80, 1, mptMacro{kind: mptMacroResonance}},
		{0xff, 1, mptMacro{kind: mptMacroCutoff, value: 127}},
	} {
		candidates := config.candidates(tt.arg)
		if len(candidates) != tt.size || candidates[len(candidates)-1] != tt.last {
			t.Fatalf("Z%02X: unexpected candidates %+v", tt.arg, candidates)
		}
	}
}
func TestMPTEmbeddedMacroLoad(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	fixture := func(rows []byte, history int, parametered map[int]string, fixed map[int]string) []byte {
		return embeddedMacroFixture(t, mptSharedEffectFixture(t, rows, flags), history, parametered, fixed)
	}
	for _, history := range []int{-1, 0, 3} {
		b := fixture([]byte{0x81, 8, 26, 0x80, 0}, history,
			map[int]string{0: "F0F000z", 1: "F0F001z", 15: "F0F001z"},
			map[int]string{0: "F0F00021", 127: "F0F0017F"})
		loaded, err := loadMPTM(b)
		if err != nil {
			t.Fatalf("history %d: %v", history, err)
		}
		cfg := loaded.Assets.ITCompatibility.mptMacroConfig
		if cfg == nil || cfg.parametered[1].kind != mptMacroResonance || cfg.parametered[15].kind != mptMacroResonance ||
			cfg.fixed[0] != (mptMacro{kind: mptMacroCutoff, value: 33}) ||
			cfg.fixed[127] != (mptMacro{kind: mptMacroResonance, value: 127}) {
			t.Fatalf("history %d: wrong config %+v", history, cfg)
		}
		b[46] &^= 8
		b[44] |= 0x80
		if _, err := loadMPTM(b); err != nil {
			t.Fatalf("required configuration flag: %v", err)
		}
	}
	for _, tt := range []struct {
		name        string
		rows        []byte
		parametered map[int]string
		fixed       map[int]string
		want        string
	}{
		{"unsupported parametered", []byte{0x81, 8, 26, 1, 0}, map[int]string{3: "Bc07z"}, nil, "unsupported embedded"},
		{"unsupported fixed", []byte{0x81, 8, 26, 0x80, 0}, nil, map[int]string{0: "F0F00210"}, "unsupported embedded"},
		{"unselected fixed", []byte{0x81, 8, 26, 0x80, 0}, nil, map[int]string{1: "Bc0700"}, ""},
		{"fixed ignores SFx", []byte{0x81, 8, 26, 0x80, 0}, map[int]string{0: "Bc07z"}, nil, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadMPTM(fixture(tt.rows, -1, tt.parametered, tt.fixed))
			if tt.want == "" && err != nil || tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
	for _, fixed := range []bool{false, true} {
		rows := []byte{0x81, 8, 26, 1, 0}
		param, fixedConfig := map[int]string{0: "F0F001z"}, map[int]string{}
		if fixed {
			rows = []byte{0x81, 8, 26, 0x80, 0}
			param, fixedConfig = nil, map[int]string{0: "F0F0017F"}
		}
		b := fixture(rows, -1, param, fixedConfig)
		flagOffset := bytesIndex(t, b, ".FSM") + 6
		b[flagOffset+int(mptITFilterBehaviour)/8] &^= 1 << (uint(mptITFilterBehaviour) % 8)
		if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "legacy filter behavior") {
			t.Fatalf("legacy resonance accepted: %v", err)
		}
	}
	b := fixture([]byte{0x81, 8, 26, 1, 0}, -1, map[int]string{0: "F0F000z"}, nil)
	binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01250000)
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "pre-1.26") {
		t.Fatalf("old embedded configuration accepted: %v", err)
	}
}

func TestMPTEmbeddedMacroStateAndAllocations(t *testing.T) {
	profile, err := projectMPTMixProfile(4, 64, 64)
	if err != nil {
		t.Fatal(err)
	}
	config := mptMacroConfig{}
	config.parametered[1] = mptMacro{kind: mptMacroResonance}
	config.parametered[2] = mptMacro{kind: mptMacroCutoff}
	config.fixed[0] = mptMacro{kind: mptMacroCutoff, value: 42}
	config.fixed[127] = mptMacro{kind: mptMacroResonance, value: 73}
	w := NewTracker(SongAssets{IT: true, NumTracks: 2,
		ITCompatibility: ITCompatibility{mptMixProfile: &profile, mptMacroConfig: &config},
		Samples:         []SampleAsset{{Data: make([]float32, 32), C5Speed: 8363, Volume: 64}}}, 48000, 64)
	selectMacro := func(track int, slot byte) {
		w.PendingCmds[track] = ModCommand{EffectCmd: 0x15, EffectArg: 0xf0 | slot}
		VolumeEffectSystem(w)
	}
	run := func(track int, arg byte) {
		w.PendingCmds[track] = ModCommand{EffectCmd: 0x19, EffectArg: arg}
		MPTCutoffMacroSystem(w)
		w.PendingCmds[track] = ModCommand{}
	}
	selectMacro(0, 1)
	selectMacro(1, 2)
	run(0, 91)
	run(1, 35)
	if w.TrackCursors[0].ITFilterResonance != 91 || w.TrackCursors[1].ITFilterCutoff != 35 ||
		w.TrackCursors[0].ITFilterCutoff != 127 || w.TrackCursors[1].ITFilterResonance != 0 {
		t.Fatal("parametered macros did not preserve channel-local state")
	}
	run(0, 0x80)
	run(1, 0xff)
	if w.TrackCursors[0].ITFilterCutoff != 42 || w.TrackCursors[1].ITFilterResonance != 73 {
		t.Fatal("fixed macros were affected by SFx selection")
	}
	if !w.startSampleVoice(0, ModCommand{Note: 61}, 0, 61, 0) {
		t.Fatal("voice allocation failed")
	}
	v := w.TrackCursors[0].ActiveVoice
	if w.VoiceLives[v].ITFilterCutoff != 42 || w.VoiceLives[v].ITFilterResonance != 91 {
		t.Fatal("voice did not inherit embedded macro filter state")
	}
	w.VoiceFilters[v].Y1, w.VoiceRightFilters[v].Y1 = .25, -.25
	run(0, 0xff)
	if w.VoiceLives[v].ITFilterResonance != 73 || w.VoiceFilters[v].Y1 != .25 || w.VoiceRightFilters[v].Y1 != -.25 {
		t.Fatal("active voice did not preserve filter history")
	}
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x19, EffectArg: 0x80}
	if got := testing.AllocsPerRun(100, func() { MPTCutoffMacroSystem(w) }); got != 0 {
		t.Fatalf("embedded macro allocated %g times", got)
	}
	w.Assets.ITCompatibility.mptMacroConfig = nil
	if got := testing.AllocsPerRun(100, func() { MPTCutoffMacroSystem(w) }); got != 0 {
		t.Fatalf("default macro allocated %g times", got)
	}
}

func TestOpenMPTMEmbeddedFilterMacros(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, opl := range []bool{false, true} {
		for _, kind := range []string{"parametered cutoff", "parametered resonance", "fixed cutoff", "fixed resonance"} {
			t.Run(fmt.Sprintf("%s/opl=%t", kind, opl), func(t *testing.T) {
				fixture := func(value byte) []byte {
					sample := byte(1)
					if opl {
						sample = 2
					}

					parametered := map[int]string{0: "F0F000z"}
					fixed := map[int]string{}
					arg := value
					switch kind {
					case "parametered cutoff":
						parametered[1] = "F0F000z"
					case "parametered resonance":
						parametered[1] = "F0F001z"
					case "fixed cutoff":
						fixed[0] = fmt.Sprintf("F0F000%02X", value)
						arg = 0x80
					case "fixed resonance":
						fixed[0] = fmt.Sprintf("F0F001%02X", value)
						arg = 0x80
					}
					rows := []byte{
						0x81, 11, 60, sample, 26, 64, 0,
						0x81, 8, 19, 0xf1, 0,
						0x81, 8, 26, arg, 0, 0,
					}
					b := mptSharedEffectFixture(t, rows, flags)
					if opl {
						header := int(binary.LittleEndian.Uint32(b[198:]))
						patch := int(binary.LittleEndian.Uint32(b[header+72:]))
						copy(b[patch:], []byte{0x21, 0x21, 0, 0, 0xf0, 0xf0, 0x0f, 0x0f, 0, 0, 0, 0})
					}
					return embeddedMacroFixture(t, b, 2, parametered, fixed)
				}
				value, baseline := byte(32), byte(96)
				if strings.Contains(kind, "resonance") {
					value, baseline = 96, 0
				}
				ref, actual := mptSharedEffectRenderForDuration(t, fixture(value), 180)
				baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture(baseline), 180)
				wantBase, gotBase := rmsWindow(baseRef, 85, 105), rmsWindow(baseActual, 85, 105)
				if wantBase < .00001 || gotBase < .00001 {
					t.Fatal("silent comparison baseline")
				}
				want, got := rmsWindow(ref, 85, 105)/wantBase, rmsWindow(actual, 85, 105)/gotBase
				if math.IsNaN(want) || math.IsNaN(got) || math.IsInf(want, 0) || math.IsInf(got, 0) ||
					math.Abs(want-got) > .12 || (!opl || strings.Contains(kind, "cutoff")) && math.Abs(want-1) < .05 {
					t.Fatalf("relative RMS: OpenMPT %.4f, tracker %.4f", want, got)
				}
			})
		}
	}
}
