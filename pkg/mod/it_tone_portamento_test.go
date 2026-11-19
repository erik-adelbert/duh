package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTITTonePortamento(t *testing.T) {
	tests := []struct {
		name       string
		start      byte
		target     byte
		speed      byte
		linear     bool
		legacy     bool
		compatible bool
		instrument bool
	}{
		{"up linear", 60, 72, 16, true, false, false, false},
		{"down linear", 72, 60, 16, true, false, false, false},
		{"up amiga", 60, 72, 16, false, false, false, false},
		{"down amiga", 72, 60, 16, false, false, false, false},
		{"GFF", 60, 72, 255, true, false, false, false},
		{"GFF amiga", 60, 72, 255, false, false, false, false},
		{"GFF legacy amiga", 60, 72, 255, false, true, false, false},
		{"legacy GFF", 60, 72, 255, true, true, false, false},
		{"legacy amiga", 60, 72, 16, false, true, false, false},
		{"compatible Gxx", 60, 72, 16, true, false, true, false},
		{"instrument with note", 60, 72, 16, true, false, false, true},
		{"legacy instrument with note", 60, 72, 16, true, true, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := []byte{0x81, 3, tt.start, 1, 0, 0x81, 9, tt.target, 7, tt.speed, 0, 0x81, 8, 7, 0, 0}
			if tt.instrument {
				rows = []byte{0x81, 3, tt.start, 1, 0, 0x81, 11, tt.target, 1, 7, tt.speed, 0, 0x81, 8, 7, 0, 0}
			}
			b := testITPitchFixture(rows, 2, tt.linear, tt.legacy)
			if tt.compatible {
				b[44] |= 0x20
			}
			ref, actual := referencePCMWithInterpolation(t, b, "2")
			refBase := referenceTickFrequency(t, ref, 1)
			actualBase := referenceTickFrequency(t, actual, 1)
			for tick := 2; tick < 6; tick++ {
				want := referenceTickFrequency(t, ref, tick) / refBase
				got := referenceTickFrequency(t, actual, tick) / actualBase
				if math.Abs(want-got) > .003 {
					t.Errorf("tick %d pitch ratio: OpenMPT %.5f, tracker %.5f", tick, want, got)
				}
			}
		})
	}
}

func TestITTonePortamentoState(t *testing.T) {
	for _, compatible := range []bool{false, true} {
		w := NewTracker(SongAssets{
			IT: true, NumTracks: 1, ITCompatibility: ITCompatibility{LinearSlides: true, CompatibleGxx: compatible},
			Samples: []SampleAsset{{Data: make([]float32, 1024), C5Speed: 8363, Volume: 64}},
		}, 48000, 256)
		w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
		VoiceAllocationSystem(w)
		v := w.TrackCursors[0].ActiveVoice
		w.VoiceSamples[v].Position = 123
		w.PendingCmds[0] = ModCommand{Note: 73, EffectCmd: 0x03, EffectArg: 4}
		VoiceAllocationSystem(w)
		PitchEffectSystem(w)
		initial := w.VoicePitches[v].BaseFreqHz
		if w.TrackCursors[0].ActiveVoice != v || w.VoiceSamples[v].Position != 123 {
			t.Fatal("portamento retriggered the active voice")
		}
		w.Clock.Tick = 1
		PitchEffectSystem(w)
		if w.VoicePitches[v].BaseFreqHz <= initial {
			t.Fatal("portamento did not slide up")
		}
		if got := w.TrackMemories[0].LastPortaUp; compatible && got != 0 || !compatible && got != 4 {
			t.Fatalf("compatible=%v: shared pitch-slide memory = %d", compatible, got)
		}
		w.Clock.Tick = 0
		w.PendingCmds[0] = ModCommand{EffectCmd: 0x02, EffectArg: 7}
		PitchEffectSystem(w)
		if got := w.TrackMemories[0].LastPortaSpeed; compatible && got != 4 || !compatible && got != 7 {
			t.Fatalf("compatible=%v: tone-portamento memory = %d", compatible, got)
		}
	}
}

func TestITTonePortamentoMappingAndInitialNote(t *testing.T) {
	instrument := ITInstrument{}
	for i := range instrument.Samples {
		instrument.Samples[i], instrument.Notes[i] = 1, uint8(i)
	}
	instrument.Notes[72] = 65
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 1, ITInstruments: []ITInstrument{instrument},
		Samples: []SampleAsset{{Data: make([]float32, 1024), C5Speed: 8363, Volume: 64}},
	}, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1, EffectCmd: 0x03, EffectArg: 4}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	if v == -1 {
		t.Fatal("Gxx with no active voice did not trigger its note")
	}
	w.PendingCmds[0] = ModCommand{Note: 73, EffectCmd: 0x03, EffectArg: 4}
	PitchEffectSystem(w)
	if got, want := w.VoicePitches[v].PortaTargetHz, noteToHz(65); got != want {
		t.Fatalf("mapped target = %v, want %v", got, want)
	}
}

func TestITTonePortamentoPackedReuse(t *testing.T) {
	p, err := unpackITPattern([]byte{0x81, 8, 7, 255, 0, 0x81, 0x80, 0, 0x81, 8, 7, 0, 0}, 3)
	if err != nil {
		t.Fatal(err)
	}
	for row, arg := range []uint8{255, 255, 0} {
		cmd := p.Rows[row][0]
		if cmd.EffectCmd != 0x03 || cmd.EffectArg != arg {
			t.Errorf("row %d: %+v", row, cmd)
		}
	}
	b := testIT()
	b[44] |= 0x20
	loaded, err := loadIT(b)
	if err != nil || !loaded.Assets.ITCompatibility.CompatibleGxx || binary.LittleEndian.Uint16(b[44:])&0x20 == 0 {
		t.Fatal("compatible Gxx flag was not loaded", err)
	}
}

func TestOpenMPTITToneFirstNote(t *testing.T) {
	b := testITPitchFixture([]byte{0x81, 11, 60, 1, 7, 255, 0}, 6, true, false)
	ref, actual := referencePCMWithInterpolation(t, b, "2")
	if channelRMS(ref, 30, 40, 0) < .001 || channelRMS(actual, 30, 40, 0) < .001 {
		t.Fatal("first note with Gxx should play")
	}
}

func TestOpenMPTITToneWithChangedSample(t *testing.T) {
	for _, tt := range []struct {
		name   string
		legacy bool
		speed  uint32
	}{
		{"IT", false, 48000}, {"legacy", true, 48000},
		{"IT different C5 speed", false, 24000}, {"legacy different C5 speed", true, 24000},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows := []byte{0x81, 3, 60, 1, 0, 0x81, 11, 72, 2, 7, 16, 0}
			b := testITPitchFixture(rows, 2, true, tt.legacy)
			sampleHeader := append([]byte(nil), b[256:336]...)
			binary.LittleEndian.PutUint32(sampleHeader[60:], tt.speed)
			sample := append([]byte(nil), b[512:512+8192]...)
			for i := range 4096 {
				binary.LittleEndian.PutUint16(sample[i*2:], uint16(int16(math.Sin(float64(i)*2*math.Pi/12)*16000)))
			}
			binary.LittleEndian.PutUint16(b[36:], 2)
			binary.LittleEndian.PutUint32(b[198:], uint32(len(b)))
			binary.LittleEndian.PutUint32(b[202:], 336)
			binary.LittleEndian.PutUint32(sampleHeader[72:], uint32(len(b)+len(sampleHeader)))
			b = append(b, sampleHeader...)
			b = append(b, sample...)
			ref, actual := referencePCMWithInterpolation(t, b, "2")
			refBase := referenceTickFrequency(t, ref, 1)
			actualBase := referenceTickFrequency(t, actual, 1)
			for tick := 2; tick < 6; tick++ {
				want := referenceTickFrequency(t, ref, tick) / refBase
				got := referenceTickFrequency(t, actual, tick) / actualBase
				if math.Abs(want-got) > .003 {
					t.Errorf("tick %d pitch ratio: OpenMPT %.5f, tracker %.5f", tick, want, got)
				}
			}
		})
	}
}

func TestOpenMPTITToneInstrumentChangeVolume(t *testing.T) {
	for _, tt := range []struct {
		name       string
		legacy     bool
		compatible bool
	}{
		{"IT", false, false},
		{"IT compatible", false, true},
		{"legacy", true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows := []byte{0x81, 3, 60, 1, 0, 0x81, 11, 72, 2, 7, 16, 0}
			b := testITPitchFixture(rows, 2, true, tt.legacy)
			src := testITInstrument()
			first := append([]byte(nil), src[len(src)-554:]...)
			first[304] = 0
			second := append([]byte(nil), first...)
			second[24] = 64
			binary.LittleEndian.PutUint16(b[34:], 2)
			b[44] |= 4
			if tt.compatible {
				b[44] |= 0x20
			}
			binary.LittleEndian.PutUint32(b[194:], uint32(len(b)))
			binary.LittleEndian.PutUint32(b[198:], uint32(len(b)+len(first)))
			binary.LittleEndian.PutUint32(b[202:], 256)
			binary.LittleEndian.PutUint32(b[206:], 336)
			b = append(b, first...)
			b = append(b, second...)
			ref, actual := referencePCMWithInterpolation(t, b, "2")
			want := channelRMS(ref, 96, 116, 0) / channelRMS(ref, 26, 36, 0)
			got := channelRMS(actual, 96, 116, 0) / channelRMS(actual, 26, 36, 0)
			if math.IsNaN(want) || math.IsNaN(got) || math.Abs(want-got) > .05 {
				t.Errorf("volume ratio: OpenMPT %.3f, tracker %.3f", want, got)
			}
		})
	}
}

func TestOpenMPTITToneInstrument(t *testing.T) {
	for _, tt := range []struct {
		name       string
		legacy     bool
		compatible bool
		mapped     bool
	}{
		{"IT", false, false, false},
		{"IT mapped", false, false, true},
		{"IT compatible", false, true, false},
		{"legacy", true, false, false},
		{"legacy compatible", true, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows := []byte{0x81, 3, 60, 1, 0, 0x81, 11, 72, 1, 7, 16, 0, 0x81, 8, 7, 0, 0}
			b := testITPitchFixture(rows, 2, true, tt.legacy)
			src := testITInstrument()
			header := append([]byte(nil), src[len(src)-554:]...)
			header[304] = 0
			if tt.mapped {
				header[64+72*2] = 65
			}
			binary.LittleEndian.PutUint16(b[34:], 1)
			b[44] |= 4
			if tt.compatible {
				b[44] |= 0x20
			}
			binary.LittleEndian.PutUint32(b[194:], uint32(len(b)))
			binary.LittleEndian.PutUint32(b[198:], 256)
			binary.LittleEndian.PutUint32(b[202:], 336)
			b = append(b, header...)
			ref, actual := referencePCMWithInterpolation(t, b, "2")
			refBase := referenceTickFrequency(t, ref, 1)
			actualBase := referenceTickFrequency(t, actual, 1)
			for tick := 2; tick < 6; tick++ {
				want := referenceTickFrequency(t, ref, tick) / refBase
				got := referenceTickFrequency(t, actual, tick) / actualBase
				if math.Abs(want-got) > .003 {
					t.Errorf("tick %d pitch ratio: OpenMPT %.5f, tracker %.5f", tick, want, got)
				}
			}
		})
	}
}

func TestOpenMPTITToneSharedMemory(t *testing.T) {
	for _, tt := range []struct {
		name       string
		rows       []byte
		legacy     bool
		compatible bool
	}{
		{"E then G00 shared", []byte{0x81, 11, 60, 1, 5, 16, 0, 0x81, 9, 72, 7, 0, 0}, false, false},
		{"E then G00 compatible", []byte{0x81, 11, 60, 1, 5, 16, 0, 0x81, 9, 72, 7, 0, 0}, false, true},
		{"E then G00 legacy", []byte{0x81, 11, 60, 1, 5, 16, 0, 0x81, 9, 72, 7, 0, 0}, true, false},
		{"G then E00 shared", []byte{0x81, 11, 60, 1, 7, 16, 0, 0x81, 8, 5, 0, 0}, false, false},
		{"G then E00 compatible", []byte{0x81, 11, 60, 1, 7, 16, 0, 0x81, 8, 5, 0, 0}, false, true},
		{"G then E00 legacy", []byte{0x81, 11, 60, 1, 7, 16, 0, 0x81, 8, 5, 0, 0}, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := testITPitchFixture(tt.rows, 2, true, tt.legacy)
			if tt.compatible {
				b[44] |= 0x20
			}
			ref, actual := referencePCMWithInterpolation(t, b, "2")
			refBase := referenceTickFrequency(t, ref, 0)
			actualBase := referenceTickFrequency(t, actual, 0)
			for tick := 1; tick < 5; tick++ {
				want := referenceTickFrequency(t, ref, tick) / refBase
				got := referenceTickFrequency(t, actual, tick) / actualBase
				if math.Abs(want-got) > .003 {
					t.Errorf("tick %d pitch ratio: OpenMPT %.5f, tracker %.5f", tick, want, got)
				}
			}
		})
	}
}

func TestOpenMPTITToneInstrumentSampleMapping(t *testing.T) {
	for _, tt := range []struct {
		name       string
		legacy     bool
		instrument bool
	}{
		{"IT instrument", false, true},
		{"IT no instrument", false, false},
		{"legacy instrument", true, true},
		{"legacy no instrument", true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows := []byte{0x81, 3, 60, 1, 0, 0x81, 9, 72, 7, 16, 0}
			if tt.instrument {
				rows = []byte{0x81, 3, 60, 1, 0, 0x81, 11, 72, 1, 7, 16, 0}
			}
			b := testITPitchFixture(rows, 2, true, tt.legacy)
			src := testITInstrument()
			instrument := append([]byte(nil), src[len(src)-554:]...)
			instrument[304] = 0
			instrument[65+72*2] = 2
			sampleHeader := append([]byte(nil), b[256:336]...)
			sample := make([]byte, 8192)
			for i := range 4096 {
				binary.LittleEndian.PutUint16(sample[i*2:], uint16(int16(math.Sin(float64(i)*2*math.Pi/12)*16000)))
			}
			binary.LittleEndian.PutUint16(b[34:], 1)
			binary.LittleEndian.PutUint16(b[36:], 2)
			b[44] |= 4
			binary.LittleEndian.PutUint32(b[194:], uint32(len(b)))
			binary.LittleEndian.PutUint32(b[198:], 256)
			binary.LittleEndian.PutUint32(b[202:], uint32(len(b)+len(instrument)))
			binary.LittleEndian.PutUint32(b[206:], 336)
			binary.LittleEndian.PutUint32(sampleHeader[72:], uint32(len(b)+len(instrument)+len(sampleHeader)))
			b = append(b, instrument...)
			b = append(b, sampleHeader...)
			b = append(b, sample...)
			ref, actual := referencePCMWithInterpolation(t, b, "2")
			refBase := referenceTickFrequency(t, ref, 1)
			actualBase := referenceTickFrequency(t, actual, 1)
			for tick := 2; tick < 6; tick++ {
				want := referenceTickFrequency(t, ref, tick) / refBase
				got := referenceTickFrequency(t, actual, tick) / actualBase
				if math.Abs(want-got) > .003 {
					t.Errorf("tick %d pitch ratio: OpenMPT %.5f, tracker %.5f", tick, want, got)
				}
			}
		})
	}
}
