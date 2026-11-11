// duh midi driver for OPL3
//
// Based on:
//   Wohlstand libADLMIDI
//   bisqwit adlmidi
//
// Uses:
//   sneakernets DMXOPL wopl definitions
//
// Original:
// Copyright (c) 2010-2014 Joel Yliluoma
// Copyright (c) 2015-2026 Vitaly Novichkov
//
// Banks:
// Copyright (c) 2017 Shannon Freeman
//
// Go implementation and modifications:
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package mid

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type testSynth struct {
	events   []string
	format   Format
	prepared int
}

func (t *testSynth) NoteOn(ch uint32, note, velocity uint8) {
	t.events = append(t.events, fmt.Sprintf("on:%d:%d:%d", ch, note, velocity))
}
func (t *testSynth) NoteOff(ch uint32, note uint8) {
	t.events = append(t.events, fmt.Sprintf("off:%d:%d", ch, note))
}
func (t *testSynth) NoteAftertouch(ch uint32, note, pressure uint8) {}
func (t *testSynth) Controller(ch uint32, ctrl, value uint8) {
	t.events = append(t.events, fmt.Sprintf("cc:%d:%d:%d", ch, ctrl, value))
}
func (t *testSynth) ProgramChange(ch uint32, program uint8) {
	t.events = append(t.events, fmt.Sprintf("program:%d:%d", ch, program))
}
func (t *testSynth) ChannelPressure(ch uint32, pressure uint8) {}
func (t *testSynth) PitchBend(ch uint32, value uint16)         {}
func (t *testSynth) SysEx(dev uint32, data []byte) bool {
	t.events = append(t.events, fmt.Sprintf("sysex:%d:%x", dev, data))
	return true
}
func (t *testSynth) DeviceSelect(name string) uint32 {
	t.events = append(t.events, "device:"+name)
	return 16
}
func (t *testSynth) playbackNoteOn(ch uint32, note, velocity uint8) bool {
	t.NoteOn(ch, note, velocity)
	return velocity != 0
}
func (t *testSynth) playbackNoteOff(ch uint32, note, velocity uint8) {
	t.NoteOff(ch, note)
}
func (t *testSynth) prepareFilePlayback() error       { t.prepared++; return nil }
func (t *testSynth) setFileFormat(format Format)      { t.format = format }
func (t *testSynth) setCMFInstruments(data []adlData) {}
func (t *testSynth) useCMFBank()                      {}
func (t *testSynth) useRSXXDefaults()                 {}
func (t *testSynth) writeOPL(reg, value int) {
	t.events = append(t.events, fmt.Sprintf("opl:%x:%x", reg, value))
}

func writeTestSMF(t *testing.T, track []byte) string {
	t.Helper()
	data := []byte{'M', 'T', 'h', 'd', 0, 0, 0, 6, 0, 0, 0, 1, 0, 96, 'M', 'T', 'r', 'k', 0, 0, 0, 0}
	binary.BigEndian.PutUint32(data[18:22], uint32(len(track)))
	data = append(data, track...)
	path := filepath.Join(t.TempDir(), "sequence.mid")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSequencerSchedulesEventsWithoutSynthClock(t *testing.T) {
	track := []byte{
		0, 0xff, 0x51, 3, 0x07, 0xa1, 0x20,
		0, 0xff, 0x09, 1, 'A',
		0, 0xc0, 5,
		0, 0x90, 60, 100,
		96, 0x80, 60, 0,
		0, 0xf0, 3, 0x7d, 0x01, 0xf7,
		0, 0xff, 0xe3, 2, 0xc0, 1,
		0, 0xff, 0x2f, 0,
	}
	target := &testSynth{}
	s := newSequencer(target)
	s.SetNoLooping(true)
	if err := s.Open(writeTestSMF(t, track)); err != nil {
		t.Fatal(err)
	}
	if target.format != FormatSMF || target.prepared != 1 {
		t.Fatalf("format %v, prepare count %d", target.format, target.prepared)
	}
	if wait := s.Tick(0); wait != 500*time.Millisecond {
		t.Fatalf("initial wait = %s, want 500ms", wait)
	}
	if want := []string{"device:A", "program:16:5", "on:16:60:100"}; !reflect.DeepEqual(target.events, want) {
		t.Fatalf("initial events = %v, want %v", target.events, want)
	}
	s.Tick(250 * time.Millisecond)
	if len(target.events) != 3 {
		t.Fatalf("events dispatched before due: %v", target.events)
	}
	s.Tick(250 * time.Millisecond)
	want := []string{"device:A", "program:16:5", "on:16:60:100", "off:16:60", "sysex:16:7d01f7", "opl:c0:31"}
	if !reflect.DeepEqual(target.events, want) || !s.IsFinished() {
		t.Fatalf("events = %v, finished %v; want %v and finished", target.events, s.IsFinished(), want)
	}
}

func TestSequencerRestoresLoopPositionForEachIteration(t *testing.T) {
	track := []byte{
		0, 0xff, 0x51, 3, 0x07, 0xa1, 0x20,
		0, 0x90, 50, 100,
		96, 0xff, 0x06, 9, 'l', 'o', 'o', 'p', 'S', 't', 'a', 'r', 't',
		0, 0x90, 60, 100,
		96, 0x80, 60, 0,
		0, 0xff, 0x06, 7, 'l', 'o', 'o', 'p', 'E', 'n', 'd',
		0, 0xff, 0x2f, 0,
	}
	target := &testSynth{}
	s := newSequencer(target)
	if err := s.Open(writeTestSMF(t, track)); err != nil {
		t.Fatal(err)
	}
	s.Tick(0)
	for range 5 {
		s.Tick(500 * time.Millisecond)
	}
	want := []string{"on:0:50:100", "on:0:60:100", "off:0:60", "on:0:60:100", "off:0:60", "on:0:60:100", "off:0:60", "on:0:60:100", "off:0:60", "on:0:60:100"}
	if !reflect.DeepEqual(target.events, want) {
		t.Fatalf("loop events count = %d, first = %v, want %v", len(target.events), target.events[:min(20, len(target.events))], want)
	}
}

func TestSequencerRestoresTempoAtLoopStart(t *testing.T) {
	track := []byte{
		0, 0xff, 0x51, 3, 0x07, 0xa1, 0x20,
		0, 0x90, 50, 100,
		96, 0xff, 0x06, 9, 'l', 'o', 'o', 'p', 'S', 't', 'a', 'r', 't',
		0, 0x90, 60, 100,
		96, 0xff, 0x51, 3, 0x03, 0xd0, 0x90,
		96, 0xff, 0x06, 7, 'l', 'o', 'o', 'p', 'E', 'n', 'd',
		0, 0xff, 0x2f, 0,
	}
	s := newSequencer(&testSynth{})
	if err := s.Open(writeTestSMF(t, track)); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct{ dt, wait time.Duration }{
		{0, 500 * time.Millisecond},
		{500 * time.Millisecond, 500 * time.Millisecond},
		{500 * time.Millisecond, 250 * time.Millisecond},
		{250 * time.Millisecond, 500 * time.Millisecond},
		{500 * time.Millisecond, 250 * time.Millisecond},
	} {
		if got := s.Tick(step.dt); got != step.wait {
			t.Fatalf("Tick(%s) = %s, want %s", step.dt, got, step.wait)
		}
	}
}

func TestSynthCanReceiveEventsAndAdvanceWithoutSequencer(t *testing.T) {
	s := NewSynth(1, 0, 0)
	base := s.DeviceSelect("live")
	s.NoteOn(base, 60, 100)
	info := s.mchans[base].activeNotes[60]
	if info == nil || len(info.phys) == 0 {
		t.Fatal("live note failed to allocate a voice")
	}
	loc := location{mchan: uint16(base), note: 60}
	for channel := range info.phys {
		before := s.achans[channel].users[loc].vibDelay
		s.Advance(10 * time.Millisecond)
		if got := s.achans[channel].users[loc].vibDelay; got != before+10*time.Millisecond {
			t.Errorf("vibrato timer = %s, want %s", got, before+10*time.Millisecond)
		}
		break
	}
	s.NoteOff(base, 60)
	if _, ok := s.mchans[base].activeNotes[60]; ok {
		t.Fatal("live note remains active after note off")
	}
}

func TestSequencerSkipsInitialSilenceAndUsesRunningStatus(t *testing.T) {
	track := []byte{
		0, 0xff, 0x51, 3, 0x07, 0xa1, 0x20,
		96, 0x90, 60, 100,
		96, 60, 0,
		0, 0xff, 0x2f, 0,
	}
	target := &testSynth{}
	s := newSequencer(target)
	s.SetNoLooping(true)
	if err := s.Open(writeTestSMF(t, track)); err != nil {
		t.Fatal(err)
	}
	if wait := s.Tick(0); wait != 500*time.Millisecond {
		t.Fatalf("wait = %s, want 500ms after skipping initial silence", wait)
	}
	if want := []string{"on:0:60:100"}; !reflect.DeepEqual(target.events, want) {
		t.Fatalf("events = %v, want %v", target.events, want)
	}
	s.Tick(500 * time.Millisecond)
	want := []string{"on:0:60:100", "on:0:60:0"}
	if !reflect.DeepEqual(target.events, want) || !s.IsFinished() {
		t.Fatalf("events = %v, finished %v; want %v and finished", target.events, s.IsFinished(), want)
	}
}

func TestDecoderTickAdvancesSynthAfterPlaybackStarts(t *testing.T) {
	track := []byte{
		0, 0xff, 0x51, 3, 0x07, 0xa1, 0x20,
		0, 0x90, 60, 100,
		96, 0x80, 60, 0,
		0, 0xff, 0x2f, 0,
	}
	m := NewMIDIPlayer(1, 0, 0)
	base := m.DeviceSelect("")
	m.SetNoLooping(true)
	if err := m.Open(writeTestSMF(t, track)); err != nil {
		t.Fatal(err)
	}
	m.Tick(100 * time.Millisecond)
	info := m.mchans[base].activeNotes[60]
	if info == nil || len(info.phys) == 0 {
		t.Fatal("first note not allocated")
	}
	loc := location{mchan: uint16(base), note: 60}
	for channel := range info.phys {
		if got := m.achans[channel].users[loc].vibDelay; got != 0 {
			t.Fatalf("first tick advanced a newly started note by %s", got)
		}
		m.Sequencer.Tick(100 * time.Millisecond)
		if got := m.achans[channel].users[loc].vibDelay; got != 0 {
			t.Fatalf("sequencer alone advanced synth timer by %s", got)
		}
		m.Tick(100 * time.Millisecond)
		if got := m.achans[channel].users[loc].vibDelay; got != 100*time.Millisecond {
			t.Fatalf("decoder advanced synth timer to %s, want 100ms", got)
		}
		break
	}
}

func TestDecoderNoteOnPreservesPlaybackActivation(t *testing.T) {
	m := NewMIDIPlayer(1, 0, 0)
	base := m.DeviceSelect("")
	m.NoteOn(base, 60, 0)
	if m.pos.cur.isPlaying {
		t.Fatal("zero-velocity note started playback")
	}
	m.NoteOn(base, 60, 100)
	if !m.pos.cur.isPlaying {
		t.Fatal("allocated live note did not start playback")
	}
	m.pos.cur.isPlaying = false
	m.volume.toon = true
	m.NoteOn(base, 60, 80)
	if m.pos.cur.isPlaying {
		t.Fatal("RSXX volume-only retrigger started playback")
	}
	m.playbackNoteOff(base, 60, 50)
	if got := m.mchans[base].activeNotes[60]; got == nil || got.vol != 50 {
		t.Fatalf("RSXX note-off did not update volume: %v", got)
	}
}

func TestDecoderNoteOnWithoutAvailableVoiceDoesNotStartPlayback(t *testing.T) {
	m := NewMIDIPlayer(1, 0, 0)
	base := m.DeviceSelect("")
	for i := range m.OPL.quads {
		m.OPL.quads[i] = -1
	}
	m.NoteOn(base, 60, 100)
	if m.pos.cur.isPlaying || m.mchans[base].activeNotes[60] != nil {
		t.Fatal("note without an available voice started playback")
	}
}

func TestSequencerConfiguresCMFSynth(t *testing.T) {
	data := make([]byte, 84)
	copy(data, "CTMF")
	binary.LittleEndian.PutUint16(data[6:], 62)
	binary.LittleEndian.PutUint16(data[8:], 78)
	binary.LittleEndian.PutUint16(data[12:], 96)
	binary.LittleEndian.PutUint16(data[58:], 1)
	data[62], data[63], data[72] = 0x21, 0x22, 0x03
	copy(data[78:], []byte{0, 0xff, 0x2f, 0})
	path := filepath.Join(t.TempDir(), "song.cmf")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	synth := NewSynth(1, 0, 0)
	s := NewSequencer(synth)
	if err := s.Open(path); err != nil {
		t.Fatal(err)
	}
	if !synth.OPL.cmf || synth.bank != ^uint32(0) || !synth.volume.log || len(synth.dyno.ins) != 1 || len(synth.dyno.metas) != 1 {
		t.Fatalf("CMF synth configuration: cmf=%v bank=%d log=%v instruments=%d metadata=%d",
			synth.OPL.cmf, synth.bank, synth.volume.log, len(synth.dyno.ins), len(synth.dyno.metas))
	}
	ins := synth.dyno.ins[0]
	if ins.ModulatorE862 != 0x21 || ins.CarrierE862 != 0x22 || ins.FeedConn != 3 || synth.dyno.metas[0].ID0 != DynamicInstrumentTag {
		t.Fatalf("CMF instrument = %+v, metadata = %+v", ins, synth.dyno.metas[0])
	}
}

func TestSequencerDispatchesIMFRegisterWrites(t *testing.T) {
	data := make([]byte, 170)
	binary.LittleEndian.PutUint16(data, 168)
	for i := 2; i+3 < len(data); i += 4 {
		data[i], data[i+1], data[i+2] = 0xc0, 1, 1
	}
	path := filepath.Join(t.TempDir(), "song.imf")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	target := &testSynth{}
	s := newSequencer(target)
	if err := s.Open(path); err != nil {
		t.Fatal(err)
	}
	if target.format != FormatIMF || !s.pos.cur.isPlaying {
		t.Fatalf("IMF format = %d, playing = %v", target.format, s.pos.cur.isPlaying)
	}
	if wait := s.Tick(0); wait != 1428*time.Microsecond {
		t.Fatalf("IMF wait = %s, want 1428µs", wait)
	}
	if len(target.events) != 1 || target.events[0] != "opl:c0:31" {
		t.Fatalf("IMF register events = %v", target.events)
	}
	s.Tick(1427 * time.Microsecond)
	if len(target.events) != 1 {
		t.Fatalf("IMF event dispatched early: %v", target.events)
	}
	s.Tick(time.Microsecond)
	if len(target.events) != 2 || target.events[1] != "opl:c0:31" {
		t.Fatalf("IMF register events after one tick = %v", target.events)
	}
}
