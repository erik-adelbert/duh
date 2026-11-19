package mod

import (
	"encoding/binary"
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestMPTResonanceSwingValidation(t *testing.T) {
	instruments := make([]ITInstrument, 2)
	if err := applyMPTResonanceSwing(instruments, []uint8{26, 48}); err != nil ||
		instruments[0].mptResonanceSwing != 26 || instruments[1].mptResonanceSwing != 48 {
		t.Fatalf("instruments %+v, error %v", instruments, err)
	}
	before := append([]ITInstrument(nil), instruments...)
	for _, invalid := range [][]uint8{{1}, {1, 65}} {
		if err := applyMPTResonanceSwing(instruments, invalid); err == nil {
			t.Fatalf("accepted %v", invalid)
		}
		if !reflect.DeepEqual(instruments, before) {
			t.Fatal("invalid values partially modified instruments")
		}
	}
}

func TestMPTResonanceSwingMath(t *testing.T) {
	for _, tt := range []struct {
		base, swing uint8
		random      int
		delta       int16
		want        uint8
	}{
		{94, 26, -128, -18, 76}, {94, 26, 127, 19, 113},
		{127, 64, -128, -62, 65}, {127, 64, 127, 63, 127},
		{0, 64, -128, 0, 0}, {0, 64, 127, 0, 0},
		{94, 0, 127, 0, 94}, {94, 26, -1, 0, 94},
	} {
		delta := mptFilterSwingDelta(tt.base, tt.swing, tt.random)
		if got := mptApplyFilterSwing(tt.base, delta); delta != tt.delta || got != tt.want {
			t.Errorf("%+v: delta %d, resonance %d", tt, delta, got)
		}
	}
	if got := mptApplyFilterSwing(2, -50); got != 0 {
		t.Fatalf("lower clamp: %d", got)
	}
}

func TestMPTResonanceSwingVoiceAndMacro(t *testing.T) {
	for _, old := range []bool{false, true} {
		t.Run(map[bool]string{false: "modern", true: "old"}[old], func(t *testing.T) {
			b := testITInstrument()
			h := b[len(b)-554:]
			h[17], h[58], h[59], h[304] = 1, 128|94, 128|90, 0
			loaded, err := loadIT(b)
			if err != nil {
				t.Fatal(err)
			}
			inst := &loaded.Assets.ITInstruments[0]
			inst.VolumeSwing, inst.PanSwing, inst.mptCutoffSwing, inst.mptResonanceSwing = 12, 14, 26, 48
			inst.mptHasFilterMode, inst.mptFilterMode = true, 1
			profile, err := projectMPTMixProfile(4, 64, 64)
			if err != nil {
				t.Fatal(err)
			}
			loaded.Assets.ITCompatibility.mptMixProfile = &profile
			loaded.Assets.ITCompatibility.mptOldSwingBehaviour = old
			w := NewTracker(loaded.Assets, 48000, 256)
			prng := rand.New(rand.NewSource(1))
			prng.Intn(256)
			prng.Intn(256)
			cutoff := mptRandomCutoff(94, 26, prng.Intn(256)-128)
			delta := mptFilterSwingDelta(90, 48, prng.Intn(256)-128)
			resonance := mptApplyFilterSwing(90, delta)
			w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
			VoiceAllocationSystem(w)
			v := w.TrackCursors[0].ActiveVoice
			life := &w.VoiceLives[v]
			if life.ITFilterCutoff != cutoff || life.ITFilterResonance != resonance ||
				w.TrackCursors[0].ITFilterResonance != 90 {
				t.Fatalf("voice cutoff/resonance %d/%d, expected %d/%d", life.ITFilterCutoff, life.ITFilterResonance, cutoff, resonance)
			}
			expected := newITFilter(cutoff, resonance, 48000, true, false, false)
			setITFilterMode(&expected, true)
			if w.VoiceFilters[v] != expected || w.VoiceRightFilters[v] != expected {
				t.Fatal("stereo high-pass filter coefficients differ")
			}
			w.VoiceFilters[v].Y1, w.VoiceRightFilters[v].Y1 = .25, -.25
			w.PendingCmds[0] = ModCommand{EffectCmd: 0x19, EffectArg: 0x84}
			MPTCutoffMacroSystem(w)
			want := uint8(32)
			if old {
				want = mptApplyFilterSwing(want, delta)
			}
			if life.ITFilterResonance != want || w.TrackCursors[0].ITFilterResonance != 32 ||
				w.VoiceFilters[v].Y1 != .25 || w.VoiceRightFilters[v].Y1 != -.25 {
				t.Fatalf("macro: resonance %d, base %d, want %d", life.ITFilterResonance, w.TrackCursors[0].ITFilterResonance, want)
			}
			updateITFilterEnvelope(w, int(v), -8)
			expected = itFilterCoefficients(cutoff, want, 48000, false, false, -64)
			setITFilterMode(&expected, true)
			if w.VoiceFilters[v].A0 != expected.A0 || w.VoiceRightFilters[v].A0 != expected.A0 {
				t.Fatal("filter envelope ignored resonance variation")
			}
			past := *life
			w.PendingCmds[0] = ModCommand{Note: 62, Instrument: 1}
			VoiceAllocationSystem(w)
			if !w.VoiceLives[v].Active || w.VoiceLives[v].ITFilterResonance != past.ITFilterResonance {
				t.Fatal("new note altered past voice")
			}
		})
	}
}

func TestMPTResonanceSwingPortamentoRestore(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	h[58], h[59], h[304] = 128|94, 128|90, 0
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Assets.ITInstruments[0].mptResonanceSwing = 64
	profile, err := projectMPTMixProfile(4, 64, 64)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Assets.ITCompatibility.mptMixProfile = &profile
	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	if w.VoiceLives[v].ITFilterResonance == 90 {
		t.Fatal("fixture did not produce resonance swing")
	}
	w.PendingCmds[0] = ModCommand{Note: 64, EffectCmd: 0x03, EffectArg: 4}
	VoiceAllocationSystem(w)
	if w.TrackCursors[0].ActiveVoice != v || w.VoiceLives[v].ITFilterResonance != 90 {
		t.Fatal("portamento did not restore base resonance")
	}
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x19, EffectArg: 60}
	MPTCutoffMacroSystem(w)
	if w.VoiceFilters[v].A0 != itFilterCoefficients(60, 90, 48000, false, false, 256).A0 {
		t.Fatal("cutoff macro used stale resonance after portamento")
	}
}

func TestMPTResonanceSwingConsumesRandomAtZero(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	h[58], h[59], h[304] = 128|94, 128, 0
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Assets.ITInstruments[0].mptResonanceSwing = 48
	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	prng := rand.New(rand.NewSource(1))
	prng.Intn(256)
	if got, want := w.itRandom.Intn(256), prng.Intn(256); got != want {
		t.Fatalf("random sequence after zero resonance: got %d, want %d", got, want)
	}
}

func TestLoadMPTMResonanceSwing(t *testing.T) {
	b := oplEnvelopeEndFixture(t, 0x01280100, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}, []byte{0})
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	properties := append([]byte("XTPM"), mptExtensionField("..SR", 1, []byte{26, 48})...)
	properties = append(properties, b[bounds.propertiesStart:bounds.serializedStart]...)
	for _, field := range []struct {
		code string
		data []byte
	}{
		{"...C", []byte{1, 0}}, {"..MT", []byte{2}},
		{".MMP", []byte{4, 0, 0, 0}}, {".APS", []byte{64, 0, 0, 0}}, {"VTSV", []byte{64, 0, 0, 0}},
	} {
		properties = append(properties, mptExtensionField(field.code, uint16(len(field.data)), field.data)...)
	}
	b[30], b[44] = 8, b[44]|8
	data := append([]byte(nil), b[:bounds.propertiesStart]...)
	data = append(data, properties...)
	tail := len(data)
	data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
	data = binary.LittleEndian.AppendUint32(data, uint32(tail))
	loaded, err := loadMPTM(data)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Assets.ITInstruments[0].mptResonanceSwing != 26 || loaded.Assets.ITInstruments[1].mptResonanceSwing != 48 {
		t.Fatal("loaded resonance variations differ")
	}
	if loaded.Assets.ITCompatibility.mptOldSwingBehaviour {
		t.Fatal("unexpected old swing behavior")
	}
	filterFlag := bytesIndex(t, data, ".FSM") + 6 + int(mptITFilterBehaviour)/8
	data[filterFlag] &^= 1 << (uint(mptITFilterBehaviour) % 8)
	if _, err := loadMPTM(data); err == nil {
		t.Fatal("accepted modern resonance swing without compatible filter behavior")
	}
	for i := range 2 {
		instrument := int(binary.LittleEndian.Uint32(data[194+i*4:]))
		data[instrument+58], data[instrument+59] = 128|90, 128|80
	}
	if _, err := loadMPTM(data); err != nil {
		t.Fatalf("explicit nonzero resonance and cutoff: %v", err)
	}
	instrument := int(binary.LittleEndian.Uint32(data[194:]))
	data[instrument+59] = 128
	if _, err := loadMPTM(data); err == nil {
		t.Fatal("accepted zero resonance without compatible filter behavior")
	}
	data[instrument+59] = 128 | 80
	data[filterFlag] |= 1 << (uint(mptITFilterBehaviour) % 8)
	data[bytesIndex(t, data, ".FSM")+6] |= 2
	loaded, err = loadMPTM(data)
	if err != nil || !loaded.Assets.ITCompatibility.mptOldSwingBehaviour {
		t.Fatalf("saved old swing behavior: %v", err)
	}
	data[bytesIndex(t, data, ".FSM")+6] &^= 2
	binary.LittleEndian.PutUint32(data[bytesIndex(t, data, "VWSL")+6:], 0x01170249)
	loaded, err = loadMPTM(data)
	if err != nil || !loaded.Assets.ITCompatibility.mptOldSwingBehaviour {
		t.Fatalf("upgraded old swing behavior: %v", err)
	}
}

func TestMPTResonanceSwingChunkingAndAllocation(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	h[58], h[59], h[304] = 128|75, 128|90, 0
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Assets.ITInstruments[0].mptResonanceSwing = 64
	assertMPTNearestChunkingAndAllocation(t, loaded)
	w := NewTracker(loaded.Assets, 48000, 256)
	cmd := ModCommand{Note: 61, Instrument: 1}
	if got := testing.AllocsPerRun(100, func() {
		w.startSampleVoice(0, cmd, 0, 61, 1)
	}); got != 0 {
		t.Fatalf("resonance swing voice start allocated %g times", got)
	}
}

func TestOpenMPTResonanceSwing(t *testing.T) {
	for _, opl := range []bool{false, true} {
		t.Run(map[bool]string{false: "PCM", true: "OPL"}[opl], func(t *testing.T) {
			b := oplEnvelopeEndFixture(t, 0x01280100, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}, []byte{0})
			index, tolerance := 1, .05
			if opl {
				index, tolerance = 0, .08
				sample := int(binary.LittleEndian.Uint32(b[206:]))
				patchOffset := int(binary.LittleEndian.Uint32(b[sample+72:]))
				patch := oplPatch{0x21, 0x21, 0, 0, 0xf0, 0xf0, 0x0f, 0x0f, 0, 0, 0, 0}
				copy(b[patchOffset:patchOffset+12], patch[:])
			} else {
				pattern := int(binary.LittleEndian.Uint32(b[210:]))
				b[pattern+11] = 2
			}

			instrument := int(binary.LittleEndian.Uint32(b[194+index*4:]))
			b[instrument+304] = 0
			b[instrument+58], b[instrument+59] = 128|75, 128|90
			loaded, err := loadITData(b, true)
			if err != nil {
				t.Fatal(err)
			}
			loaded.Assets.ITInstruments[index].mptResonanceSwing = 64
			w := NewTracker(loaded.Assets, referenceRate, 5760)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 1, 125, -1
			actual := make([]float32, 5760*2)
			if n := w.Read(actual, 5760); n != 5760 {
				t.Fatalf("short render %d", n)
			}
			delta := mptFilterSwingDelta(90, 64, rand.New(rand.NewSource(1)).Intn(256)-128)
			resonance := mptApplyFilterSwing(90, delta)
			refData := append([]byte(nil), b...)
			refData[instrument+59] = 128 | resonance
			ref := referenceTransition(t, refData)
			base := append([]byte(nil), b...)
			base[instrument+59] = 128
			refBase := referenceTransition(t, base)
			baseLoaded, err := loadITData(base, true)
			if err != nil {
				t.Fatal(err)
			}
			baseWorld := NewTracker(baseLoaded.Assets, referenceRate, 5760)
			baseWorld.Clock.Speed, baseWorld.Clock.Tempo, baseWorld.Clock.Tick = 1, 125, -1
			actualBase := make([]float32, 5760*2)
			if n := baseWorld.Read(actualBase, 5760); n != 5760 {
				t.Fatalf("short baseline render %d", n)
			}
			gotBase, wantBase := rmsWindow(actualBase, 5, 15), rmsWindow(refBase, 5, 15)
			if gotBase < .001 || wantBase < .001 {
				t.Fatal("silent baseline")
			}
			got, want := rmsWindow(actual, 5, 15)/gotBase, rmsWindow(ref, 5, 15)/wantBase
			if math.Abs(got-want) > tolerance {
				t.Fatalf("resonance %d relative RMS: tracker %g, OpenMPT %g", resonance, got, want)
			}
		})
	}
}
