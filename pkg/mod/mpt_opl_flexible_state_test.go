package mod

import (
	"fmt"
	"strings"
	"testing"
)

func TestMPTUnsupportedNewPlaybackFlags(t *testing.T) {
	for _, bit := range []int{121, 125} {
		flags := make([]byte, bit/8+1)
		flags[bit/8] = 1 << (bit % 8)
		_, _, err := parseMPTPlaybackProfileFlags(flags)
		if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("unsupported MPTM playback behaviour bit %d", bit)) {
			t.Errorf("bit %d: expected rejection, got %v", bit, err)
		}
	}
}

func TestMPTPlaybackProfileFlags(t *testing.T) {
	flags := make([]byte, 13)
	flags[0], flags[12] = 0x81, 8
	low, additional, err := parseMPTPlaybackProfileFlags(flags)
	if err != nil || low != 0x81 || !additional.oplFlexibleNoteOff || additional.itRestoreChannelPan || flags[12] != 8 {
		t.Fatalf("low %#x additional %+v error %v, input %x", low, additional, err, flags)
	}
	flags[12] |= 0x40
	_, additional, err = parseMPTPlaybackProfileFlags(flags)
	if err != nil || !additional.oplFlexibleNoteOff || !additional.itRestoreChannelPan {
		t.Fatalf("additional %+v error %v", additional, err)
	}
	flags = append(flags, 1)
	_, additional, err = parseMPTPlaybackProfileFlags(flags)
	if err != nil || !additional.itDuplicateCheck {
		t.Fatalf("additional %+v error %v", additional, err)
	}
	flags[13] |= 2
	_, additional, err = parseMPTPlaybackProfileFlags(flags)
	if err != nil || !additional.oplWithNNA {
		t.Fatalf("additional %+v error %v", additional, err)
	}
	flags[13] |= 0x20
	_, additional, err = parseMPTPlaybackProfileFlags(flags)
	if err != nil || !additional.oplNoResetAtEnvelopeEnd {
		t.Fatalf("additional %+v error %v", additional, err)
	}
	flags[13] |= 0x40
	if _, _, err := parseMPTPlaybackProfileFlags(flags); err == nil || !strings.Contains(err.Error(), "bit 110") {
		t.Fatalf("expected next unknown bit rejection, got %v", err)
	}
	if _, _, err := parseMPTPlaybackProfileFlags(nil); err == nil {
		t.Fatal("missing flags accepted")
	}
}

func TestMPTAdditionalInstrumentBehaviours(t *testing.T) {
	flags := make([]byte, 13)
	flags[10], flags[11], flags[12] = 0x80, 1, 0x10
	_, additional, err := parseMPTPlaybackProfileFlags(flags)
	if err != nil || !additional.itMultiSampleInstrument || !additional.rowDelayWithNoteDelay || !additional.itNoteOffOldEffects {
		t.Fatalf("additional %+v error %v", additional, err)
	}
}

func TestOPLNoteFadeControlsAndCompletion(t *testing.T) {
	for _, flexible := range []bool{false, true} {
		w := testOPLWorld(ModCommand{})
		w.Assets.IT = true
		inst := ITInstrument{Fadeout: 256, GlobalVolume: 128}
		for i := range inst.Samples {
			inst.Samples[i], inst.Notes[i] = 1, uint8(i)
		}
		w.Assets.ITInstruments = []ITInstrument{inst}
		w.Assets.ITCompatibility.oplFlexibleNoteOff = flexible
		w.Clock.Tick = 0
		w.PendingCmds[0] = ModCommand{Note: 60, Instrument: 1}
		VoiceAllocationSystem(w)
		VoiceKinematicsSystem(w)
		v := w.TrackCursors[0].ActiveVoice
		channel := w.VoiceSamples[v].oplChannel - 1
		w.PendingCmds[0] = ModCommand{Note: 253}
		VoiceAllocationSystem(w)
		if !w.VoiceLives[v].ITFade || w.VoiceLives[v].KeyOff {
			t.Fatalf("flexible=%v: incorrect fade flags", flexible)
		}
		key := w.opl.keyOn[channel]
		w.VoicePitches[v].BaseFreqHz *= 2
		VoiceKinematicsSystem(w)
		if flexible {
			if w.opl.keyOn[channel]&0x20 == 0 || w.opl.keyOn[channel] == key {
				t.Fatal("flexible fade did not preserve pitch control")
			}
		} else if w.opl.keyOn[channel] != key || key&0x20 != 0 {
			t.Fatal("legacy fade did not hold key-off and pitch")
		}
		for range 4 {
			VoiceLifecycleSystem(w)
		}
		if w.VoiceLives[v].Active || w.oplVoices[channel] != -1 {
			t.Fatalf("flexible=%v: completed fade did not release voice", flexible)
		}
	}
}
