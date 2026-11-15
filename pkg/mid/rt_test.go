package mid

import (
	"reflect"
	"testing"
	"time"
)

func TestRTClipsBeforeNarrowing(t *testing.T) {
	for _, tt := range []struct {
		input int64
		want  int16
	}{
		{0, 0}, {32767, 32767}, {32768, 32767},
		{65536, 32767}, {-32768, -32768}, {-32769, -32768},
		{-65536, -32768},
	} {
		if got := clipRTSample(tt.input); got != tt.want {
			t.Errorf("clipRTSample(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func ExampleSynth_RTGenerate() {
	s := NewSynth(1, 0, 0)
	if err := s.SetBank(79); err != nil {
		panic(err)
	}
	started, err := s.RTNoteOn(0, 60, 100)
	if err != nil {
		panic(err)
	}
	if !started {
		return
	}

	// Generate 10 ms of stereo PCM, then send it to an audio output.
	samples := make([]int16, PCMRate/100*2)
	if _, err := s.RTGenerate(samples); err != nil {
		panic(err)
	}
	if err := s.RTNoteOff(0, 60); err != nil {
		panic(err)
	}
	// Continue generating audio to render the note's release.
	if _, err := s.RTGenerate(samples); err != nil {
		panic(err)
	}
}

func TestRTEvents(t *testing.T) {
	s := NewSynth(1, 0, 0)
	if err := s.RTPatchChange(0, 5); err != nil {
		t.Fatal(err)
	}
	if err := s.RTBankChange(0, 0x0203); err != nil {
		t.Fatal(err)
	}
	if err := s.RTPitchBendML(0, 70, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.RTChannelAfterTouch(0, 23); err != nil {
		t.Fatal(err)
	}
	if err := s.RTNoteAfterTouch(0, 60, 34); err != nil {
		t.Fatal(err)
	}
	ch := &s.mchans[0]
	if ch.patch != 5 || ch.bank.MSB != 2 || ch.bank.LSB != 3 || !ch.hasBank {
		t.Fatalf("program/bank = %d/%+v, has bank %v", ch.patch, ch.bank, ch.hasBank)
	}
	if ch.bend != float64((70<<7|1)-8192) || ch.aftertouch != 23 || ch.noteAftertouch[60] != 34 {
		t.Fatalf("pitch/pressure state = %v/%d/%d", ch.bend, ch.aftertouch, ch.noteAftertouch[60])
	}
	if err := s.RTBankChangeMSB(0, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.RTBankChangeLSB(0, 0); err != nil {
		t.Fatal(err)
	}
	started, err := s.RTNoteOn(0, 60, 100)
	if err != nil || !started {
		t.Fatalf("RTNoteOn = %v, %v", started, err)
	}
	if _, ok := ch.activeNotes[60]; !ok {
		t.Fatal("note not active")
	}
	started, err = s.RTNoteOn(0, 60, 0)
	if err != nil || started {
		t.Fatalf("zero velocity RTNoteOn = %v, %v", started, err)
	}
	if _, ok := ch.activeNotes[60]; ok {
		t.Fatal("zero velocity did not release note")
	}
	if err := s.RTNoteOff(0, 60); err != nil {
		t.Fatal(err)
	}
}

func TestRTRejectsInvalidEvents(t *testing.T) {
	s := NewSynth(1, 0, 0)
	for _, test := range []struct {
		name string
		call func() error
	}{
		{"channel", func() error { _, err := s.RTNoteOn(16, 60, 100); return err }},
		{"note", func() error { return s.RTNoteOff(0, 128) }},
		{"velocity", func() error { _, err := s.RTNoteOn(0, 60, 128); return err }},
		{"controller", func() error { return s.RTControllerChange(0, 128, 0) }},
		{"program", func() error { return s.RTPatchChange(0, 128) }},
		{"pressure", func() error { return s.RTChannelAfterTouch(0, 128) }},
		{"note pressure", func() error { return s.RTNoteAfterTouch(0, 60, 128) }},
		{"pitch", func() error { return s.RTPitchBend(0, 16384) }},
		{"pitch bytes", func() error { return s.RTPitchBendML(0, 128, 0) }},
		{"bank", func() error { return s.RTBankChange(0, 0x0180) }},
		{"sysex", func() error { _, err := s.RTSystemExclusive([]byte{0x7e, 0x7f, 0xf7}); return err }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("invalid event accepted")
			}
		})
	}
	if len(s.mchans) != 0 {
		t.Fatal("invalid events created channels")
	}
}

func TestRTSysExAndReset(t *testing.T) {
	s := NewSynth(1, 0, 0)
	ok, err := s.RTSystemExclusive([]byte{0xf0, 0x7d, 'D', 'U', 'H', 1, 1, 0, 78, 0xf7})
	if err != nil || !ok || s.bank != 78 {
		t.Fatalf("bank selection = %v, %v, bank %d", ok, err, s.bank)
	}
	ok, err = s.RTSystemExclusive([]byte{0xf0, 0x7d, 0, 0xf7})
	if err != nil || ok {
		t.Fatalf("unsupported SysEx = %v, %v", ok, err)
	}
	if err := s.RTControllerChange(0, 7, 37); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.RTNoteOn(0, 60, 100); err != nil || !ok {
		t.Fatalf("RTNoteOn = %v, %v", ok, err)
	}
	if err := s.RTControllerChange(0, 66, 127); err != nil {
		t.Fatal(err)
	}
	if err := s.RTNoteOff(0, 60); err != nil {
		t.Fatal(err)
	}
	s.RTResetState()
	if s.mchans[0].vol != s.mchans[0].vol0 {
		t.Fatal("reset did not restore volume")
	}
	for i := range s.achans {
		if len(s.achans[i].users) != 0 {
			t.Fatalf("channel %d has remaining users after reset", i)
		}
	}
}

func TestRTPanicStopsActiveAndSustainedNotes(t *testing.T) {
	s := NewSynth(1, 0, 0)
	if ok, err := s.RTNoteOn(0, 60, 100); err != nil || !ok {
		t.Fatalf("RTNoteOn = %v, %v", ok, err)
	}
	if err := s.RTControllerChange(0, 64, 127); err != nil {
		t.Fatal(err)
	}
	if err := s.RTNoteOff(0, 60); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.RTNoteOn(0, 62, 100); err != nil || !ok {
		t.Fatalf("RTNoteOn = %v, %v", ok, err)
	}
	s.RTAllNotesOff()
	if len(s.mchans[0].activeNotes) != 0 {
		t.Fatal("active notes survived panic")
	}
	for i := range s.achans {
		if len(s.achans[i].users) != 0 {
			t.Fatalf("channel %d has remaining users", i)
		}
	}
}

func TestRTGenerateMixAndTiming(t *testing.T) {
	s := NewSynth(2, 0, 0)
	reference := NewSynth(2, 0, 0)
	for _, synth := range []*Synth{s, reference} {
		if ok, err := synth.RTNoteOn(0, 60, 100); err != nil || !ok {
			t.Fatalf("RTNoteOn = %v, %v", ok, err)
		}
	}
	out := make([]int16, 512*2)
	if n, err := s.RTGenerate(out); n != len(out) || err != nil {
		t.Fatalf("RTGenerate = %d, %v", n, err)
	}
	expected := make([]int16, len(out))
	samples := make([]int16, len(out))
	mix := make([]int64, len(out))
	for _, card := range reference.OPL.Cards {
		card.Stream2Ch(samples, len(samples))
		for i, sample := range samples {
			mix[i] += int64(sample)
		}
	}
	for i, sample := range mix {
		expected[i] = int16(max(-32768, min(32767, sample)))
	}
	if !reflect.DeepEqual(out, expected) {
		t.Fatal("rendered output differs from mixed OPL output")
	}
	nonzero := false
	for _, sample := range out {
		nonzero = nonzero || sample != 0
	}
	if !nonzero {
		t.Fatal("rendered note was silent")
	}
	loc := location{mchan: 0, note: 60}
	for i := range s.achans {
		if data, ok := s.achans[i].users[loc]; ok {
			if want := time.Duration(int64(512) * int64(time.Second) / PCMRate); data.vibDelay != want {
				t.Fatalf("elapsed = %v, want %v", data.vibDelay, want)
			}
			return
		}
	}
	t.Fatal("note has no physical channel")
}

func TestRTGenerateValidationAndFractionalTiming(t *testing.T) {
	s := NewSynth(1, 0, 0)
	if n, err := s.RTGenerate([]int16{123}); n != 0 || err == nil {
		t.Fatalf("odd output = %d, %v", n, err)
	}
	if n, err := s.RTGenerate(nil); n != 0 || err != nil {
		t.Fatalf("empty output = %d, %v", n, err)
	}
	if ok, err := s.RTNoteOn(0, 60, 100); err != nil || !ok {
		t.Fatalf("RTNoteOn = %v, %v", ok, err)
	}
	for range PCMRate {
		if _, err := s.RTGenerate(make([]int16, 2)); err != nil {
			t.Fatal(err)
		}
	}
	loc := location{mchan: 0, note: 60}
	for i := range s.achans {
		if data, ok := s.achans[i].users[loc]; ok {
			if data.vibDelay != time.Second || s.rtRemainder != 0 {
				t.Fatalf("elapsed = %v, remainder %d", data.vibDelay, s.rtRemainder)
			}
			return
		}
	}
	t.Fatal("note has no physical channel")
}

func TestRTGenerateAcrossChunks(t *testing.T) {
	whole := NewSynth(1, 0, 0)
	split := NewSynth(1, 0, 0)
	for _, s := range []*Synth{whole, split} {
		if ok, err := s.RTNoteOn(0, 60, 100); err != nil || !ok {
			t.Fatalf("RTNoteOn = %v, %v", ok, err)
		}
	}
	a, b := make([]int16, 2050), make([]int16, 2050)
	if _, err := whole.RTGenerate(a); err != nil {
		t.Fatal(err)
	}
	for start := 0; start < len(b); {
		end := min(start+1024, len(b))
		if _, err := split.RTGenerate(b[start:end]); err != nil {
			t.Fatal(err)
		}
		start = end
	}
	if !reflect.DeepEqual(a, b) || whole.rtRemainder != split.rtRemainder {
		t.Fatal("rendering across chunks differs from separate calls")
	}
}
