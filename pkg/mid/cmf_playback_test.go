// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mid

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
	"time"
)

func testCMF(modulator byte) []byte {
	track := []byte{
		0, 0x90, 60, 100,
		96, 0x80, 60, 0,
		0, 0xff, 0x2f, 0,
	}
	data := make([]byte, 78+len(track))
	copy(data, "CTMF")
	binary.LittleEndian.PutUint16(data[6:], 62)
	binary.LittleEndian.PutUint16(data[8:], 78)
	binary.LittleEndian.PutUint16(data[12:], 96)
	binary.LittleEndian.PutUint16(data[58:], 1)
	copy(data[62:], []byte{
		modulator, 0x21, 0x00, 0x00,
		0xf0, 0xf0, 0x77, 0x77,
		0x00, 0x00, 0x01,
	})
	copy(data[78:], track)
	return data
}

func TestCMFNotePlaybackAndReset(t *testing.T) {
	d, err := Decode(bytes.NewReader(testCMF(0x22)))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close() //nolint:errcheck

	render := func() []byte {
		t.Helper()
		pcm, err := io.ReadAll(d)
		if err != nil {
			t.Fatal(err)
		}
		if int64(len(pcm)) != d.Size() {
			t.Fatalf("rendered %d bytes, want %d", len(pcm), d.Size())
		}
		if bytes.Count(pcm, []byte{0}) == len(pcm) {
			t.Fatal("CMF note rendered silence")
		}
		return pcm
	}

	first := render()
	if err := d.Reset(); err != nil {
		t.Fatal(err)
	}
	if second := render(); !bytes.Equal(first, second) {
		t.Fatal("CMF playback changed after decoder reset")
	}
}

func TestCMFInstrumentBankIsolation(t *testing.T) {
	first := NewSynth(1, 0, 0)
	second := NewSynth(1, 0, 0)
	first.setCMFInstruments([]adlData{{ModulatorE862: 0x11, CarrierE862: 0x21, FeedConn: 1}})
	second.setCMFInstruments([]adlData{{ModulatorE862: 0x12, CarrierE862: 0x22, FeedConn: 0}})

	check := func(s *Synth, modulator, carrier, feedConn byte) {
		t.Helper()
		const channel = 0
		s.OPL.patch(channel, DynamicInstrumentTag)
		s.OPL.pan(channel, 64)
		s.OPL.touch(channel, 100, 100, 127, 127, false)
		s.OPL.mulOffset(channel, 1)
		s.OPL.mulOffset(channel, 0)
		state, ok := s.OPL.Cards[0].State()
		if !ok {
			t.Fatal("OPL state unavailable")
		}
		for slot, want := range []byte{modulator, carrier} {
			reg := 0x20 + s.OPL.operator(channel, slot)
			if got := state.Registers[0][reg]; got != want {
				t.Errorf("operator %d = %#x, want %#x", slot, got, want)
			}
		}
		if got := state.Registers[0][0xc0] & 1; got != feedConn {
			t.Errorf("feed/connection = %#x, want %#x", got, feedConn)
		}
		s.OPL.mute(channel)
	}

	check(first, 0x11, 0x21, 1)
	check(second, 0x12, 0x22, 0)
	check(first, 0x11, 0x21, 1)

	first.setCMFInstruments([]adlData{{ModulatorE862: 0x13, CarrierE862: 0x23, FeedConn: 0}})
	check(first, 0x13, 0x23, 0)
	check(second, 0x12, 0x22, 0)
}

func TestCMFReloadReplacesInstrumentBank(t *testing.T) {
	s := NewSynth(1, 0, 0)
	seq := NewSequencer(s)
	for _, modulator := range []byte{0x21, 0x22} {
		if err := seq.openReader(bytes.NewReader(testCMF(modulator))); err != nil {
			t.Fatal(err)
		}
		if len(s.dyno.ins) != 1 || len(s.dyno.metas) != 1 {
			t.Fatalf("bank sizes after reload = %d/%d, want 1/1", len(s.dyno.ins), len(s.dyno.metas))
		}
		if meta := s.dyno.metas[0]; meta.KON != time.Second || meta.KOFF != 500*time.Millisecond {
			t.Errorf("CMF durations = %s/%s, want 1s/500ms", meta.KON, meta.KOFF)
		}
		s.OPL.patch(0, DynamicInstrumentTag)
		state, ok := s.OPL.Cards[0].State()
		if !ok {
			t.Fatal("OPL state unavailable")
		}
		if got := state.Registers[0][0x20]; got != modulator {
			t.Errorf("reloaded modulator = %#x, want %#x", got, modulator)
		}
	}
}

func TestCMFInvalidReloadPreservesInstrumentBank(t *testing.T) {
	s := NewSynth(1, 0, 0)
	seq := NewSequencer(s)
	if err := seq.openReader(bytes.NewReader(testCMF(0x21))); err != nil {
		t.Fatal(err)
	}
	original := s.GetAdlIns(DynamicInstrumentTag)

	tooMany := testCMF(0x22)
	binary.LittleEndian.PutUint16(tooMany[58:], 0x8000)
	for _, invalid := range [][]byte{testCMF(0x22)[:70], tooMany} {
		if err := seq.openReader(bytes.NewReader(invalid)); err == nil {
			t.Fatal("invalid CMF reload succeeded")
		}
		if got := s.GetAdlIns(DynamicInstrumentTag); got != original {
			t.Errorf("instrument after invalid reload = %+v, want %+v", got, original)
		}
	}
}
