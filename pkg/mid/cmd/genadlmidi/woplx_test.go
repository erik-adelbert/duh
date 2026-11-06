package main

import (
	"testing"
	"time"

	mid "github.com/erik-adelbert/duh/pkg/mid"
)

func TestXinsDefsAndOps(t *testing.T) {
	x := xins{
		fbconn: map[string]int{
			"FB1":   3,
			"CONN1": 1,
			"FB2":   2,
		},
		ops: [4]xop{
			{xat: 13, xdc: 2, xst: 10, xrl: 4, xwf: 5, xml: 9, xtl: 42, xkl: 2, xvb: 1, xam: 1, xkr: 1},
			{xat: 2, xdc: 3, xst: 4, xrl: 5, xwf: 6, xml: 7, xtl: 8, xkl: 1, xeg: 1},
		},
	}

	defs := x.Defs()
	wantDef := idef{0xD9, 0x27, 0xD2, 0x23, 0xA4, 0x45, 5, 6, 0xAA, 0x48, 7}
	if defs[0] != wantDef {
		t.Errorf("Defs()[0] = %v, want %v", defs[0], wantDef)
	}
	if defs[1][10] != 4 {
		t.Errorf("Defs()[1] feedback/connection = %d, want 4", defs[1][10])
	}

	ops := x.Ops()
	if ops[0].E862 != 0x06452327 || ops[0].D40 != 0x48 {
		t.Errorf("Ops()[0] = %#v, want modulator 0x06452327 and carrier 0x48", ops[0])
	}
	if ops[1].E862 != 0x05A4D2D9 || ops[1].D40 != 0xAA {
		t.Errorf("Ops()[1] = %#v, want modulator 0x05A4D2D9 and carrier 0xAA", ops[1])
	}
}

func TestXinsAttrs(t *testing.T) {
	x := xins{attrs: map[string]int{
		"NOTE_OFF0": -3,
		"NOTE_OFF1": 4,
		"VEL_OFF":   -2,
		"SEC_VOICE": 1,
		"DRUM_KEY":  36,
		"KON":       125,
		"KOFF":      250,
	}}
	ins := mid.NewInsEntry()

	x.Attrs(ins)

	if ins.Note.Off0 != -3 || ins.Note.Off1 != 4 {
		t.Errorf("note offsets = (%d, %d), want (-3, 4)", ins.Note.Off0, ins.Note.Off1)
	}
	if ins.VelOff != -2 || ins.SecVoice != 1 || ins.DrumKey != 36 {
		t.Errorf("instrument attrs = (vel %d, secondary %g, drum %d), want (-2, 1, 36)", ins.VelOff, ins.SecVoice, ins.DrumKey)
	}
	if ins.KON != 125*time.Millisecond || ins.KOFF != 250*time.Millisecond {
		t.Errorf("sounding delays = (%s, %s), want (125ms, 250ms)", ins.KON, ins.KOFF)
	}
}

func TestXinsPercussionDrumKeyIsFixedNote(t *testing.T) {
	x := xins{
		flags: "4OP",
		attrs: map[string]int{"DRUM_KEY": 57},
	}

	melodic := x.insEntry(false)
	if melodic.Flags&mid.FlagFixedTone != 0 {
		t.Fatal("melodic instrument unexpectedly marked fixed-note")
	}

	percussion := x.insEntry(true)
	if percussion.Flags&mid.FlagFixedTone == 0 {
		t.Fatal("percussion instrument with DRUM_KEY not marked fixed-note")
	}
	if percussion.DrumKey != 57 {
		t.Errorf("percussion DrumKey = %d, want 57", percussion.DrumKey)
	}
}
