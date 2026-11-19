package mod

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func TestOPLEnvelopeEndCutScope(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		opl         bool
		loop        bool
		sustain     bool
		lastValue   int16
		wantStopped bool
	}{
		{"enabled OPL zero end", true, true, false, false, 0, true},
		{"disabled profile", false, true, false, false, 0, false},
		{"PCM voice", true, false, false, false, 0, false},
		{"looping envelope", true, true, true, false, 0, false},
		{"sustain envelope", true, true, false, true, 0, false},
		{"nonzero endpoint", true, true, false, false, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, assets := transitionFixture(true, false, false)
			assets.ITCompatibility.oplEnvelopeEndCut = tt.enabled
			assets.ITInstruments = []ITInstrument{{
				GlobalVolume: 128,
				VolumeEnvelope: ITVolumeEnvelope{
					Enabled: true, Loop: tt.loop, Sustain: tt.sustain, LoopEnd: 1, SustainEnd: 1,
					Points: []ITEnvelopePoint{{Tick: 0, Value: 64}, {Tick: 2, Value: tt.lastValue}},
				},
			}}
			sample := 0
			if tt.opl {
				sample = 1
			}
			w := NewTracker(assets, referenceRate, 64)
			if !w.startSampleVoice(0, ModCommand{Note: 61}, sample, 61, 1) {
				t.Fatal("voice allocation failed")
			}
			voice := w.TrackCursors[0].ActiveVoice
			VoiceLifecycleSystem(w)
			if !w.VoiceLives[voice].Active {
				t.Fatal("voice stopped before reaching the envelope end")
			}
			VoiceLifecycleSystem(w)
			if stopped := !w.VoiceLives[voice].Active; stopped != tt.wantStopped {
				t.Fatalf("voice stopped = %v, want %v", stopped, tt.wantStopped)
			}
			if tt.wantStopped && (w.TrackCursors[0].ActiveVoice != -1 || w.oplVoices[0] != -1) {
				t.Fatal("stopped voice retained its track or OPL channel")
			}
		})
	}
}

func TestOpenMPTOPLEnvelopeEndFlags(t *testing.T) {
	for _, tt := range []struct {
		name       string
		flags      []byte
		secondTick bool
	}{
		{"target flags", []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}, false},
		{"minimal flags", []byte{0x80}, true},
		{"target without envelope position handling", []byte{0x81, 0xff, 0xff, 0x7f, 0xff, 0xff, 7}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseMPTSavedPlayBehaviours(tt.flags)
			if err != nil {
				t.Fatal(err)
			}
			for _, version := range []uint32{0x01280019, 0x01280020, 0x01290055, 0x01290056} {
				projection, err := projectMPTBehaviours(parsed, version, true, true, true)
				if err != nil || projection.oplNoResetAtEnvelopeEnd != (version >= 0x01280020 && version <= 0x01290055) {
					t.Fatalf("version %#x: projection %+v, error %v", version, projection, err)
				}
				pcm := referenceTransition(t, oplEnvelopeEndFixture(t, version, tt.flags, []byte{0}))
				first, second, third := rmsWindow(pcm, 2, 15), rmsWindow(pcm, 22, 35), rmsWindow(pcm, 42, 55)
				if first < .001 || (second > .001) != tt.secondTick || third > .0001 {
					t.Fatalf("version %#x: tick RMS %.6f %.6f %.6f", version, first, second, third)
				}
			}
		})
	}
}

func oplEnvelopeEndFixture(t *testing.T, version uint32, flags, followup []byte) []byte {
	return oplEnvelopeEndFixtureRows(t, version, flags, 3, followup)
}

func oplEnvelopeEndFixtureRows(t *testing.T, version uint32, flags []byte, emptyRows int, followup []byte) []byte {
	t.Helper()
	b, _ := transitionFixture(true, false, false)
	b = b[:len(b)-11]
	b[44] |= 4
	b[50] = 1
	binary.LittleEndian.PutUint16(b[34:], 2)
	binary.LittleEndian.PutUint32(b[202:], 256)
	binary.LittleEndian.PutUint32(b[206:], 8704)
	binary.LittleEndian.PutUint32(b[210:], 336)
	for i := range 2 {
		binary.LittleEndian.PutUint32(b[194+i*4:], uint32(len(b)))
		h := make([]byte, 554)
		copy(h, "IMPI")
		h[17], h[24], h[25] = 0, 128, 128
		for note := range 120 {
			h[64+note*2], h[65+note*2] = byte(note), byte(2-i)
		}
		if i == 0 {
			h[304], h[305], h[310], h[313] = 1, 2, 64, 0
			binary.LittleEndian.PutUint16(h[314:], 2)
		}
		b = append(b, h...)
	}
	packed := []byte{0x81, 3, 60, 1, 0}
	packed = append(packed, make([]byte, emptyRows)...)
	packed = append(packed, followup...)
	packed = append(packed, make([]byte, 63-emptyRows)...)
	binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
	copy(b[344:], packed)
	binary.LittleEndian.PutUint32(b[210:], uint32(len(b)))
	b = append(b, b[336:344+len(packed)]...)
	b = append(b, []byte("STPM")...)
	b = append(b, mptExtensionField("VWSL", 4, binary.LittleEndian.AppendUint32(nil, version))...)
	b = append(b, mptExtensionField(".FSM", uint16(len(flags)), flags)...)
	offset := len(b)
	b = append(b, mptTestSequences([]mptTestEntry{{"l", []byte{1, 0}}, {"a", []byte{0, 0}}})...)
	b = binary.LittleEndian.AppendUint32(b, uint32(offset))
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	ext, err := parseMPTExtensions(b[bounds.propertiesStart:bounds.serializedStart], 2)
	if err != nil || ext.lastSavedVersion == nil || *ext.lastSavedVersion != version ||
		!bytes.Equal(ext.playbackFlags, flags) {
		t.Fatalf("fixture extensions: %+v, error %v", ext, err)
	}
	if _, err := parseMPTSequences(b[bounds.serializedStart:bounds.serializedEnd], 1); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestOpenMPTOPLEnvelopeEnd(t *testing.T) {
	for _, tt := range []struct {
		name    string
		row     []byte
		command ModCommand
		resumes bool
	}{
		{"no followup", []byte{0}, ModCommand{}, false},
		{"opl instrument", []byte{0x81, 2, 1, 0}, ModCommand{Instrument: 1}, true},
		{"pcm instrument", []byte{0x81, 2, 2, 0}, ModCommand{Instrument: 2}, true},
		{"opl note", []byte{0x81, 3, 60, 1, 0}, ModCommand{Note: 61, Instrument: 1}, true},
		{"pcm note", []byte{0x81, 3, 60, 2, 0}, ModCommand{Note: 61, Instrument: 2}, true},
		{"volume", []byte{0x81, 4, 64, 0}, ModCommand{HasVolume: true, Volume: 64}, false},
		{"note off", []byte{0x81, 1, 255, 0}, ModCommand{Note: 255}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, assets := transitionFixture(true, false, false)
			instruments := make([]ITInstrument, 2)
			for i := range instruments {
				instruments[i].GlobalVolume = 128
				for note := range 120 {
					instruments[i].Notes[note] = uint8(note)
					instruments[i].Samples[note] = uint8(2 - i)
				}
			}
			instruments[0].VolumeEnvelope = ITVolumeEnvelope{Enabled: true, Points: []ITEnvelopePoint{{Tick: 0, Value: 64}, {Tick: 2, Value: 0}}}
			assets.ITInstruments = instruments
			assets.ITCompatibility.oplEnvelopeEndCut = true
			rows := make([][]ModCommand, 64)
			for i := range rows {
				rows[i] = make([]ModCommand, 1)
			}
			rows[0][0], rows[4][0] = ModCommand{Note: 61, Instrument: 1}, tt.command
			assets.OrderList, assets.Patterns = []int{0}, []PatternAsset{{Rows: rows}}
			w := NewTracker(assets, referenceRate, 5760)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 1, 125, -1
			actual := make([]float32, 5760*2)
			if got := w.Read(actual, 5760); got != 5760 {
				t.Fatalf("short render: %d", got)
			}
			for _, version := range []uint32{0x01280019, 0x01280020, 0x01290055, 0x01290056} {
				ref := referenceTransition(t, oplEnvelopeEndFixture(t, version, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}, tt.row))
				for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
					if rmsWindow(pcm, 2, 15) < .001 || rmsWindow(pcm, 22, 35) > .0001 || rmsWindow(pcm, 42, 55) > .0001 {
						t.Fatalf("%s version %#x: initial=%g second=%g third=%g", name, version, rmsWindow(pcm, 2, 15), rmsWindow(pcm, 22, 35), rmsWindow(pcm, 42, 55))
					}

					if got := rmsWindow(pcm, 85, 95); (got > .001) != tt.resumes {
						t.Fatalf("%s version %#x: followup RMS %g, resumes %v", name, version, got, tt.resumes)
					}
				}

				if tt.resumes {
					want, got := referenceTickFrequency(t, ref, 4), referenceTickFrequency(t, actual, 4)
					if math.Abs(got/want-1) > .01 {
						t.Fatalf("version %#x: frequency OpenMPT %.2f tracker %.2f", version, want, got)
					}
				}
			}
		})
	}
}
