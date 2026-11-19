package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestMPTVolumeReleaseState(t *testing.T) {
	loaded, err := loadMPTM(mptVolumeReleaseFixture(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 2}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	for range 5 {
		VoiceLifecycleSystem(w)
	}
	life := &w.VoiceLives[v]
	releaseITVoice(w, v)
	if !life.mptEnvelopeReleased || life.ITEnvelopeTick != 6 || life.mptReleaseValue != 168 {
		t.Fatalf("release jump: %+v", *life)
	}
	VoiceLifecycleSystem(w)
	if life.ITEnvelopeVolume != 168.0/256 || life.ITEnvelopeTick != 7 {
		t.Fatalf("release initial volume: %+v", *life)
	}
	releaseITVoice(w, v)
	if life.ITEnvelopeTick != 7 || life.mptReleaseValue != 168 {
		t.Fatal("repeated note-off retriggered release")
	}
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 2}
	VoiceAllocationSystem(w)
	next := w.TrackCursors[0].ActiveVoice
	if w.VoiceLives[next].mptEnvelopeReleased {
		t.Fatal("new voice inherited release state")
	}
}

func TestMPTVolumeReleaseVersionAndDisabledEnvelope(t *testing.T) {
	b := mptVolumeReleaseFixture(t, 2)
	old := append([]byte(nil), b...)
	binary.LittleEndian.PutUint32(old[bytesIndex(t, old, "VWSL")+6:], 0x01280042)
	if _, err := loadMPTM(old); err == nil || !strings.Contains(err.Error(), "pre-1.28.00.43") {
		t.Fatalf("old version: %v", err)
	}
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	b[instrument+304] &^= 1
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	env := loaded.Assets.ITInstruments[1].VolumeEnvelope
	if env.Enabled || len(env.Points) != 4 {
		t.Fatalf("disabled envelope not retained: %+v", env)
	}
}

func TestMPTVolumeReleaseNoteOffPaths(t *testing.T) {
	for _, path := range []string{"duplicate", "new-note-action", "past-note"} {
		t.Run(path, func(t *testing.T) {
			loaded, err := loadMPTM(mptVolumeReleaseFixture(t, 2))
			if err != nil {
				t.Fatal(err)
			}
			inst := &loaded.Assets.ITInstruments[1]
			inst.NewNoteAction = 1
			if path == "duplicate" {
				inst.DuplicateCheck, inst.DuplicateAction = 3, 1
			}
			if path == "new-note-action" {
				inst.NewNoteAction = 2
			}
			w := NewTracker(loaded.Assets, 48000, 256)
			w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 2}
			VoiceAllocationSystem(w)
			old := w.TrackCursors[0].ActiveVoice
			for range 5 {
				VoiceLifecycleSystem(w)
			}
			w.PendingCmds[0] = ModCommand{Note: 62, Instrument: 2}
			VoiceAllocationSystem(w)
			if path == "past-note" {
				w.applyITInstrumentControl(0, w.TrackCursors[0].ActiveVoice, 0x71)
			}
			if !w.VoiceLives[old].Active || !w.VoiceLives[old].mptEnvelopeReleased || w.VoiceLives[old].ITEnvelopeTick != 6 {
				t.Fatalf("%s did not release prior voice: %+v", path, w.VoiceLives[old])
			}
			if w.VoiceLives[w.TrackCursors[0].ActiveVoice].mptEnvelopeReleased {
				t.Fatal("new voice inherited prior release")
			}
		})
	}
}

func TestOpenMPTVolumeReleaseAudibleEffect(t *testing.T) {
	ref, actual := mptSharedEffectRenderForDuration(t, mptVolumeReleaseFixture(t, 3), 220)
	baseRef, baseActual := mptSharedEffectRenderForDuration(t, mptVolumeReleaseFixture(t, 255), 220)
	for name, pair := range map[string][2][]float32{"OpenMPT": {ref, baseRef}, "tracker": {actual, baseActual}} {
		var difference float64
		for i := 140 * 96; i < 180*96; i++ {
			d := float64(pair[0][i] - pair[1][i])
			difference += d * d
		}
		if math.IsNaN(difference) || difference < .01 {
			t.Fatalf("%s release node had no audible effect: %g", name, difference)
		}
	}
}

func TestMPTVolumeReleaseChunkingAndAllocation(t *testing.T) {
	loaded, err := loadMPTM(mptVolumeReleaseFixture(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	const frames = 12000
	render := func(chunk int) []float32 {
		w := NewTracker(loaded.Assets, referenceRate, 1024)
		w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
		out := make([]float32, frames*2)
		for pos := 0; pos < frames; {
			n := min(chunk, frames-pos)
			if got := w.Read(out[pos*2:(pos+n)*2], n); got != n {
				t.Fatalf("short render at frame %d: %d", pos, got)
			}
			pos += n
		}
		return out
	}
	want := render(1024)
	for _, chunk := range []int{1, 17, 127, 960} {
		got := render(chunk)
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("chunk %d sample %d: got %g, want %g", chunk, i, got[i], want[i])
			}
		}
	}
	w := NewTracker(loaded.Assets, referenceRate, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 2}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	if got := testing.AllocsPerRun(100, func() {
		w.VoiceLives[v].mptEnvelopeReleased = false
		releaseITVoice(w, v)
	}); got != 0 {
		t.Fatalf("release jump allocated %g times", got)
	}
}

func TestOpenMPTVolumeReleaseProfiles(t *testing.T) {
	for _, compatible := range []bool{false, true} {
		for _, node := range []byte{0, 1, 2, 3} {
			t.Run(fmt.Sprintf("compatible=%t/node=%d", compatible, node), func(t *testing.T) {
				b := mptVolumeReleaseFixture(t, node)
				if !compatible {
					flags := bytesIndex(t, b, ".FSM") + 6
					b[flags+3] &^= 0x80
				}
				ref, actual := mptSharedEffectRenderForDuration(t, b, 260)
				if rmsWindow(ref, 78, 79) < .001 || rmsWindow(actual, 78, 79) < .001 {
					t.Fatal("silent comparison before release")
				}
				for _, ms := range []int{138, 158, 178, 198, 218} {
					r, a := rmsWindow(ref, ms, ms+1), rmsWindow(actual, ms, ms+1)
					if math.Abs(r-a) > .003 && math.Abs(r-a) > .08*math.Max(r, a) {
						t.Errorf("%d ms RMS: OpenMPT %g tracker %g", ms, r, a)
					}
				}
			})
		}
	}
}
