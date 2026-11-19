package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTMGlissando(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"up Gxx", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 7, 0, 0}},
		{"down Gxx", []byte{0x81, 11, 72, 1, 19, 0x11, 0, 0x81, 9, 60, 7, 8, 0, 0x81, 8, 7, 0, 0}},
		{"Lxx", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 12, 0, 0}},
		{"volume portamento", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 5, 72, 198, 0, 0x81, 4, 198, 0}},
		{"disable", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 19, 0x10, 0, 0x81, 8, 7, 0, 0}},
		{"S00 recall", []byte{0x81, 11, 60, 1, 19, 0x1f, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 19, 0, 0, 0x81, 8, 7, 0, 0}},
		{"same row volume portamento", []byte{0x81, 3, 60, 1, 0, 0x81, 13, 72, 198, 19, 0x11, 0, 0x81, 4, 198, 0}},
		{"pitch slide after portamento", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 6, 8, 0}},
		{"new note", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 3, 61, 1, 0}},
		{"finetune before", []byte{0x81, 11, 60, 1, 19, 0x2a, 0, 0x81, 8, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 7, 0, 0}},
		{"finetune after", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 19, 0x25, 0, 0x81, 8, 7, 0, 0}},
		{"vibrato and volume portamento", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 13, 72, 198, 8, 0x85, 0, 0x81, 12, 198, 8, 0x85, 0}},
	}
	all := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, tt := range tests {
		for _, nonlinear := range []bool{false, true} {
			for _, old := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/nonlinear=%t/old=%t", tt.name, nonlinear, old), func(t *testing.T) {
					flags := all
					b := mptSharedEffectFixture(t, tt.rows, flags)
					b[50] = 3
					binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01321200)
					if nonlinear {
						b[44] &^= 8
					}
					if old {
						b[44] |= 0x10
					}
					ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
					for tick := range 12 {
						want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
						if math.IsNaN(got) || math.Abs(got/want-1) > .004 {
							t.Errorf("tick %d: OpenMPT %.2f Hz tracker %.2f Hz", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestMPTMGlissandoProfileValidation(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for arg := byte(0x10); arg <= 0x1f; arg++ {
		b := mptSharedEffectFixture(t, []byte{0x81, 8, 19, arg, 0}, flags)
		if _, err := loadMPTM(b); err != nil {
			t.Errorf("S%02X: %v", arg, err)
		}
		binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01250000)
		if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "pre-1.26") {
			t.Errorf("old S%02X: %v", arg, err)
		}
		binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01321200)
		offset := bytesIndex(t, b, ".FSM") + 6
		b[offset] &^= 1 << mptPeriodsAreHertz
		if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "period-based pitch") {
			t.Errorf("period S%02X: %v", arg, err)
		}
	}
	b := mptSharedEffectFixture(t, []byte{0x81, 8, 19, 0x11, 0}, flags)
	b[44] &^= 8
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "period-based pitch") {
		t.Errorf("upgraded nonlinear period mode: %v", err)
	}
}

func TestMPTMGlissandoOPLState(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	rows := []byte{0x81, 11, 60, 2, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 7, 0, 0}
	b := mptSharedEffectFixture(t, rows, flags)
	b[50] = 3
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, referenceRate, 256)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	buf := make([]float32, 512)
	audible := false
	for remaining := 4*960 + 1; remaining > 0; {
		n := min(remaining, 256)
		if got := w.Read(buf[:n*2], n); got != n {
			t.Fatalf("short render: %d", got)
		}
		for _, sample := range buf[:n*2] {
			audible = audible || sample != 0
		}
		remaining -= n
	}
	v := w.TrackCursors[0].ActiveVoice
	if v < 0 || !w.VoiceLives[v].Active || w.VoiceSamples[v].oplChannel == 0 || !audible {
		t.Fatal("expected an active, audible OPL voice")
	}
	pitch := w.VoicePitches[v]
	if !w.TrackMemories[0].itGlissando || !pitch.itPortamento ||
		pitch.BaseFreqHz <= noteToHz(60) || math.Abs(pitch.StepIncrement*referenceRate-pitch.BaseFreqHz) < 1 {
		t.Fatalf("OPL glissando did not quantize output while preserving slide state: %+v", pitch)
	}
}

func TestOpenMPTMGlissandoOPL(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, downward := range []bool{false, true} {
		t.Run(fmt.Sprintf("downward=%t", downward), func(t *testing.T) {
			start, target := byte(60), byte(72)
			if downward {
				start, target = target, start
			}
			rows := []byte{0x81, 11, start, 2, 19, 0x11, 0, 0x81, 9, target, 7, 8, 0, 0x81, 8, 7, 0, 0}
			b := mptSharedEffectFixture(t, rows, flags)
			b[50] = 3
			ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
			refBase, actualBase := referenceTickFrequency(t, ref, 0), referenceTickFrequency(t, actual, 0)
			for tick := range 12 {
				want := referenceTickFrequency(t, ref, tick) / refBase
				got := referenceTickFrequency(t, actual, tick) / actualBase
				if math.Abs(got-want) > .02 {
					t.Errorf("tick %d: OpenMPT ratio %.4f tracker %.4f", tick, want, got)
				}
			}
		})
	}
}

func TestOpenMPTMGlissandoProfiles(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, rate := range []uint32{8363, 11025, 22050} {
		for _, version := range []uint32{0x01260000, 0x01321200} {
			for _, shared := range []bool{false, true} {
				t.Run(fmt.Sprintf("rate=%d/version=%08x/shared=%t", rate, version, shared), func(t *testing.T) {
					profile := flags
					if !shared {
						profile &^= 1 << mptITPortaMemoryShare
					}
					rows := []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0,
						0x81, 8, 12, 0, 0, 0x81, 8, 19, 0, 0}
					b := mptSharedEffectFixture(t, rows, profile)
					b[50] = 3
					binary.LittleEndian.PutUint32(b[256+60:], rate)
					binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], version)
					ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
					for tick := range 12 {
						want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
						if math.Abs(got/want-1) > .004 {
							t.Errorf("tick %d: OpenMPT %.2f Hz tracker %.2f Hz", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestOpenMPTMGlissandoSpeedOne(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("slides=%t", enabled), func(t *testing.T) {
			profile := flags
			if !enabled {
				profile &^= 1 << mptSlidesAtSpeed1
			}
			rows := []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 9, 72, 7, 0x10, 0,
				0x81, 8, 7, 0, 0, 0x81, 8, 7, 0, 0, 0x81, 8, 7, 0, 0}
			b := mptSharedEffectFixture(t, rows, profile)
			b[50] = 1
			ref, actual := mptSharedEffectRender(t, b)
			for tick := range 6 {
				want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
				if math.Abs(got/want-1) > .004 {
					t.Errorf("tick %d: OpenMPT %.2f Hz tracker %.2f Hz", tick, want, got)
				}
			}
		})
	}
}
