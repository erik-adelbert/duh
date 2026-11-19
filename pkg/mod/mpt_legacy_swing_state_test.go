package mod

import (
	"encoding/binary"
	"math"
	"math/rand"
	"testing"
)

func TestMPTMissingFlagsSwingState(t *testing.T) {
	b := mptMissingFlagsInstrumentFixture(t)
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	b[instrument+17], b[instrument+25], b[instrument+26], b[instrument+27] = 1, 128, 40, 40
	b[instrument+304], b[256+19] = 0, 32
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}

	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 2}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	if v < 0 {
		t.Fatal("voice was not allocated")
	}
	amp, cursor := &w.VoiceAmps[v], &w.TrackCursors[0]
	volume, pan := amp.Volume, amp.Panning
	volDelta, panDelta := amp.ITVolumeSwing, amp.ITPanSwing
	if !cursor.itHasRestorePan || !amp.mptApplyLegacySwing || volDelta == 0 || panDelta == 0 {
		t.Fatalf("swing was not initialized: %+v", amp)
	}
	VoiceKinematicsSystem(w)
	if amp.Volume != clamp32(volume+float32(volDelta)/256, 0, 1) ||
		amp.Panning != clamp32(pan+float32(panDelta)/128, -1, 1) ||
		cursor.Panning != amp.Panning || amp.ITVolumeSwing != 0 || amp.ITPanSwing != 0 {
		t.Fatal("legacy variation was not applied to persistent channel state")
	}
	swungVolume, swungPan := amp.Volume, amp.Panning
	VoiceKinematicsSystem(w)
	if amp.Volume != swungVolume || amp.Panning != swungPan {
		t.Fatal("legacy variation was applied more than once")
	}
	w.PendingCmds[0] = ModCommand{Note: 64, EffectCmd: 0x03, EffectArg: 4}
	VoiceAllocationSystem(w)
	if cursor.ActiveVoice != v || cursor.Panning != pan || amp.Panning != pan || cursor.itHasRestorePan {
		t.Fatal("portamento did not restore the unswung pan")
	}
	w.PendingCmds[0] = ModCommand{Note: 65, Instrument: 2}
	VoiceAllocationSystem(w)
	next := cursor.ActiveVoice
	if next == v || !w.VoiceLives[v].ITPastNote || w.VoiceAmps[v].Volume != swungVolume {
		t.Fatal("new note altered the previous NNA voice")
	}
	setTrackPanning(w, 0, next, .25)
	if cursor.itHasRestorePan || !w.VoiceAmps[next].mptApplyLegacySwing || w.VoiceAmps[next].ITPanSwing == 0 {
		t.Fatal("set-pan command should retain pending legacy swing and cancel restoration")
	}
	VoiceKinematicsSystem(w)
	if cursor.Panning != w.VoiceAmps[next].Panning {
		t.Fatal("set-pan variation was not applied to the foreground channel")
	}
}

func TestMPTMissingFlagsSwingChunkingAndAllocation(t *testing.T) {
	b := mptMissingFlagsInstrumentFixture(t)
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	b[pattern+11] = 2
	b[instrument+26], b[instrument+27], b[instrument+304] = 40, 40, 0
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	assertMPTNearestChunkingAndAllocation(t, loaded)
	w := NewTracker(loaded.Assets, 48000, 256)
	cmd := ModCommand{Note: 61, Instrument: 2}
	if got := testing.AllocsPerRun(100, func() {
		w.startSampleVoice(0, cmd, 0, 61, 2)
		VoiceKinematicsSystem(w)
	}); got != 0 {
		t.Fatalf("legacy swing note and kinematics allocated %g times", got)
	}
}

func TestOpenMPTLegacySwingVolumeColumn(t *testing.T) {
	b := mptMissingFlagsInstrumentFixture(t)
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	copy(b[pattern+8:], []byte{0x81, 7, 60, 2, 20, 0})
	b[instrument+304], b[256+19] = 0, 32
	random := rand.New(rand.NewSource(1)).Intn(256) - 128
	swing, delta := 0, 0
	for value := 1; value <= 64; value++ {
		d := ((random*value/64 + 1) * 64) / 199
		if d != 0 && d%4 == 0 && d > -64 && d < 64 {
			swing, delta = value, d
			break
		}
	}
	if swing == 0 {
		t.Fatal("could not construct fixed-swing reference")
	}
	b[instrument+26] = byte(swing)
	_, actual := mptSharedEffectRenderForDuration(t, b, 100)
	fixed := append([]byte(nil), b...)
	fixed[instrument+26] = 0
	fixed[pattern+12] = byte(20 + delta/4)
	ref := referenceMPTMForDuration(t, fixed, 100)
	base := append([]byte(nil), b...)
	base[instrument+26] = 0
	refBase, actualBase := mptSharedEffectRenderForDuration(t, base, 100)
	gotBase, wantBase := rmsWindow(actualBase, 10, 30), rmsWindow(refBase, 10, 30)
	if gotBase < .00001 || wantBase < .00001 {
		t.Fatal("silent comparison baseline")
	}
	got, want := rmsWindow(actual, 10, 30)/gotBase, rmsWindow(ref, 10, 30)/wantBase
	if math.Abs(got-want) > .04 {
		t.Fatalf("volume column swing %d: tracker %g, OpenMPT %g", delta, got, want)
	}
}
