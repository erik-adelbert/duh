package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTMNoteFadeFineVolumeDown(t *testing.T) {
	for _, shared := range []bool{false, true} {
		for _, old := range []bool{false, true} {
			for _, opl := range []bool{false, true} {
				t.Run(fmt.Sprintf("shared=%t/old=%t/opl=%t", shared, old, opl), func(t *testing.T) {
					fixture := func(withSlide bool, fadeout uint16) []byte {
						rows := []byte{0x81, 7, 60, 2, 32, 0, 0x81, 1, 253, 0}
						if withSlide {
							rows = []byte{0x81, 7, 60, 2, 32, 0, 0x81, 5, 253, 76, 0}
						}
						b := mptS70Fixture(t, rows)
						instrument := int(binary.LittleEndian.Uint32(b[198:]))
						binary.LittleEndian.PutUint16(b[instrument+20:], fadeout)
						if opl {
							for note := range 120 {
								b[instrument+65+note*2] = 2
							}
							sample := int(binary.LittleEndian.Uint32(b[206:]))
							patch := int(binary.LittleEndian.Uint32(b[sample+72:]))
							copy(b[patch:patch+12], []byte{0x21, 0x21, 0, 0, 0xf0, 0xf0, 0x01, 0x01, 0, 0, 0, 0})
						}
						flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITVolColMemory)/8
						if shared {
							b[flag] &^= 1 << (uint(mptITVolColMemory) % 8)
						} else {
							b[flag] |= 1 << (uint(mptITVolColMemory) % 8)
						}
						if old {
							b[44] |= 0x10
						}
						return b
					}
					ref, actual := mptSharedEffectRenderForDuration(t, fixture(true, 128), 120)
					refBase, actualBase := mptSharedEffectRenderForDuration(t, fixture(false, 128), 120)
					refNoFade, actualNoFade := mptSharedEffectRenderForDuration(t, fixture(true, 0), 120)
					for _, ms := range []int{50, 70, 90, 110} {
						r, a := rmsWindow(refBase, ms, ms+5), rmsWindow(actualBase, ms, ms+5)
						if r < .0001 || a < .0001 {
							t.Fatalf("silent comparison baseline at %dms: %g %g", ms, r, a)
						}
						want, got := rmsWindow(ref, ms, ms+5)/r, rmsWindow(actual, ms, ms+5)/a
						tolerance := .012
						if opl {
							tolerance = .04
						} else if ms == 50 {
							tolerance = .02
						}
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > tolerance {
							t.Errorf("%dms: OpenMPT ratio %.3f tracker %.3f", ms, want, got)
						}
						if !opl && (want > .985 || got > .985) {
							t.Errorf("%dms: fine volume down missing: OpenMPT %.3f tracker %.3f", ms, want, got)
						}
						if !opl {
							wantFade := rmsWindow(ref, ms, ms+5) / rmsWindow(refNoFade, ms, ms+5)
							gotFade := rmsWindow(actual, ms, ms+5) / rmsWindow(actualNoFade, ms, ms+5)
							if math.IsNaN(wantFade) || math.IsNaN(gotFade) || math.Abs(gotFade-wantFade) > .05 || wantFade > .99 {
								t.Errorf("%dms: fade ratio OpenMPT %.3f tracker %.3f", ms, wantFade, gotFade)
							}
						}
					}
				})
			}
		}
	}
}

func TestMPTMNoteFadeFineVolumeValidation(t *testing.T) {
	tests := []struct {
		name string
		row  []byte
		ok   bool
	}{
		{"fine down one", []byte{0x81, 5, 253, 76, 0}, true},
		{"fine down other", []byte{0x81, 5, 253, 77, 0}, false},
		{"fine down and E04", []byte{0x81, 13, 253, 76, 5, 4, 0}, false},
		{"fine down and instrument", []byte{0x81, 7, 253, 2, 76, 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadMPTM(mptS70Fixture(t, tt.row))
			if tt.ok && err != nil || !tt.ok && (err == nil || !strings.Contains(err.Error(), "note fade with other commands")) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestMPTMNoteFadeCombinationSupported(t *testing.T) {
	tests := []struct {
		name string
		cmd  ModCommand
		want bool
	}{
		{"standalone", ModCommand{}, true},
		{"E04", ModCommand{EffectCmd: 0x02, EffectArg: 4}, true},
		{"D01", ModCommand{EffectCmd: 0x0a, EffectArg: 1}, true},
		{"D00", ModCommand{EffectCmd: 0x0a}, true},
		{"D01 and volume", ModCommand{EffectCmd: 0x0a, EffectArg: 1, HasVolume: true, Volume: 32}, false},
		{"D01 and instrument", ModCommand{EffectCmd: 0x0a, EffectArg: 1, Instrument: 1}, false},
		{"fine down", ModCommand{HasVolume: true, Volume: 76}, true},
		{"instrument", ModCommand{Instrument: 1}, false},
		{"other volume", ModCommand{HasVolume: true, Volume: 77}, false},
		{"other effect", ModCommand{EffectCmd: 0x02, EffectArg: 3}, false},
		{"unmapped argument", ModCommand{EffectArg: 4}, false},
		{"volume and effect", ModCommand{HasVolume: true, Volume: 76, EffectCmd: 0x02, EffectArg: 4}, false},
		{"volume and argument", ModCommand{HasVolume: true, Volume: 76, EffectArg: 4}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mptNoteFadeCombinationSupported(tt.cmd); got != tt.want {
				t.Fatalf("got %t, want %t", got, tt.want)
			}
		})
	}
}

func TestMPTMNoteFadeFineVolumeVoiceIsolation(t *testing.T) {
	loaded, err := loadMPTM(mptS70Fixture(t, []byte{0x81, 5, 253, 76, 0}))
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, referenceRate, 256)
	w.Clock.Speed = 2
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 2}
	VoiceAllocationSystem(w)
	past := w.TrackCursors[0].ActiveVoice
	w.PendingCmds[0] = ModCommand{Note: 62, Instrument: 2}
	VoiceAllocationSystem(w)
	current := w.TrackCursors[0].ActiveVoice
	if past == current || past < 0 || current < 0 || !w.VoiceLives[past].ITPastNote {
		t.Fatal("expected foreground and past voices")
	}
	w.VoiceAmps[current].Volume = 1.0 / 64
	previous := w.VoiceAmps[past].Volume
	w.PendingCmds[0] = ModCommand{Note: 253, HasVolume: true, Volume: 76}
	VoiceAllocationSystem(w)
	VolumeEffectSystem(w)
	if !w.VoiceLives[current].ITFade || w.VoiceAmps[current].Volume != 0 ||
		w.VoiceLives[past].ITFade || w.VoiceAmps[past].Volume != previous {
		t.Fatal("fade or fine volume down affected the wrong voice or failed to clamp")
	}
	w.Clock.Tick = 1
	VolumeEffectSystem(w)
	if w.VoiceAmps[current].Volume != 0 {
		t.Fatal("fine volume slide repeated on a later tick")
	}
}

func TestOpenMPTMNoteFadeVolumeSlide(t *testing.T) {
	for _, arg := range []byte{0, 0x01, 0x03, 0x10, 0x30, 0x1f, 0xf1, 0x11, 0xff} {
		for _, speed := range []byte{1, 2, 6} {
			for _, old := range []bool{false, true} {
				t.Run(fmt.Sprintf("D%02X/speed=%d/old=%t", arg, speed, old), func(t *testing.T) {
					fixture := func(fade bool, slide byte) []byte {
						row := []byte{0x81, 8, 4, slide, 0}
						if fade {
							row = []byte{0x81, 9, 253, 4, slide, 0}
						}
						rows := append([]byte{0x81, 15, 60, 2, 32, 4, 0x02, 0}, row...)
						rows = append(rows, 0, 0)
						b := mptPCMNoteFadeSlideFixture(t, rows)
						b[50] = speed
						instrument := int(binary.LittleEndian.Uint32(b[198:]))
						binary.LittleEndian.PutUint16(b[instrument+20:], 128)
						if old {
							b[44] |= 0x10
						}
						return b
					}
					ref, actual := mptSharedEffectRenderForDuration(t, fixture(true, arg), 500)
					refBase, actualBase := mptSharedEffectRenderForDuration(t, fixture(false, arg), 500)
					initialRef, initialActual := rmsWindow(ref, 8, 11), rmsWindow(actual, 8, 11)
					if initialRef < .001 || initialActual < .001 {
						t.Fatal("silent initial note")
					}
					compared := 0
					for tick := int(speed); tick < int(speed)*3; tick++ {
						ms := tick*20 + 15
						r, a := rmsWindow(refBase, ms, ms+3), rmsWindow(actualBase, ms, ms+3)
						if r < .001 || a < .001 {
							continue
						}
						refLevel, actualLevel := rmsWindow(ref, ms, ms+3), rmsWindow(actual, ms, ms+3)
						want, got := refLevel/r, actualLevel/a
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .06 {
							t.Errorf("tick %d: fade ratio OpenMPT %.3f tracker %.3f", tick, want, got)
						}
						want, got = refLevel/initialRef, actualLevel/initialActual
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .06 {
							t.Errorf("tick %d: level ratio OpenMPT %.3f tracker %.3f", tick, want, got)
						}
						compared++
					}
					if compared == 0 {
						t.Fatal("no audible comparison windows")
					}
					if arg == 1 && speed == 6 && !old {
						ms := (int(speed)*2-1)*20 + 15
						if rmsWindow(refBase, ms, ms+3)-rmsWindow(ref, ms, ms+3) < .001 ||
							rmsWindow(actualBase, ms, ms+3)-rmsWindow(actual, ms, ms+3) < .001 {
							t.Fatal("fade has no measurable effect")
						}
						refFine, actualFine := mptSharedEffectRenderForDuration(t, fixture(false, 0xff), 500)
						if math.Abs(rmsWindow(refBase, ms, ms+3)-rmsWindow(refFine, ms, ms+3)) < .001 ||
							math.Abs(rmsWindow(actualBase, ms, ms+3)-rmsWindow(actualFine, ms, ms+3)) < .001 {
							t.Fatal("different volume slides have no measurable effect")
						}
					}
				})
			}
		}
	}
}

func TestMPTMNoteFadeVolumeSlideValidation(t *testing.T) {
	rows := []byte{0x81, 3, 60, 2, 0, 0x81, 9, 253, 4, 1, 0}
	b := mptS70Fixture(t, rows)
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "note fade with Dxx and OPL samples") {
		t.Fatalf("OPL combination: %v", err)
	}
	b = mptPCMNoteFadeSlideFixture(t, rows)
	if _, err := loadMPTM(b); err != nil {
		t.Fatalf("PCM combination: %v", err)
	}
	assertMPTPatternDelayChunking(t, b, referenceRate/10)
	itRows := []byte{0x81, 3, 60, 2, 0, 0x81, 9, 246, 4, 1, 0}
	if _, err := loadITData(mptPCMNoteFadeSlideFixture(t, itRows), false); err == nil || !strings.Contains(err.Error(), "note fade with other commands") {
		t.Fatalf("IT combination: %v", err)
	}
}

func mptPCMNoteFadeSlideFixture(t *testing.T, rows []byte) []byte {
	t.Helper()
	b := mptS70Fixture(t, rows)
	binary.LittleEndian.PutUint32(b[206:], 0)
	return b
}

func TestMPTMNoteFadeVolumeSlideVoiceIsolation(t *testing.T) {
	b := mptPCMNoteFadeSlideFixture(t, []byte{0x81, 9, 253, 4, 1, 0})
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, referenceRate, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 2}
	VoiceAllocationSystem(w)
	past := w.TrackCursors[0].ActiveVoice
	w.PendingCmds[0] = ModCommand{Note: 62, Instrument: 2}
	VoiceAllocationSystem(w)
	current := w.TrackCursors[0].ActiveVoice
	if past < 0 || current < 0 || past == current || !w.VoiceLives[past].ITPastNote {
		t.Fatal("expected foreground and past voices")
	}
	previous := w.VoiceAmps[past].Volume
	w.PendingCmds[0] = ModCommand{Note: 253, EffectCmd: 0x0a, EffectArg: 1}
	VoiceAllocationSystem(w)
	w.Clock.Tick = 1
	VolumeEffectSystem(w)
	if !w.VoiceLives[current].ITFade || w.VoiceLives[past].ITFade ||
		w.VoiceAmps[current].Volume != 63.0/64 || w.VoiceAmps[past].Volume != previous {
		t.Fatal("fade or slide affected the wrong voice")
	}
}

func TestLoadMPTMStandaloneNoteFade(t *testing.T) {
	b := oplEnvelopeEndFixture(t, 0x01280100, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}, []byte{0x81, 1, 253, 0})
	instrument := int(binary.LittleEndian.Uint32(b[194:]))
	b[instrument+304] = 0
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	properties := append([]byte(nil), b[bounds.propertiesStart:bounds.serializedStart]...)
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
	if got := loaded.Assets.Patterns[0].Rows[4][0].Note; got != 253 {
		t.Fatalf("loaded note = %d, want fade", got)
	}
}
