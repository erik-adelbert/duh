package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTITSampleOffset(t *testing.T) {
	tests := []struct {
		name       string
		rows       []byte
		legacy     bool
		oldEffects bool
		loop       bool
		startMS    int
		want       int
	}{
		{"high offset and O01", []byte{0x81, 8, 19, 0xa1, 0, 0x81, 11, 60, 1, 15, 1, 0}, false, false, false, 21, 12},
		{"high offset and O00", []byte{0x81, 8, 19, 0xa1, 0, 0x81, 11, 60, 1, 15, 0, 0}, false, false, false, 21, 6},
		{"offset memory without note", []byte{0x81, 8, 15, 2, 0, 0x81, 11, 60, 1, 15, 0, 0}, false, false, false, 21, 6},
		{"high offset alone", []byte{0x81, 8, 19, 0xa1, 0, 0x81, 3, 60, 1, 0}, false, false, false, 21, 12},
		{"SA1 with note IT", []byte{0x81, 11, 60, 1, 19, 0xa1, 0, 0}, false, false, false, 1, 12},
		{"SA1 with note ModPlug", []byte{0x81, 11, 60, 1, 19, 0xa1, 0, 0}, true, false, false, 1, 6},
		{"out of range IT", []byte{0x81, 8, 19, 0xa3, 0, 0x81, 11, 60, 1, 15, 0, 0}, false, false, false, 21, 12},
		{"out of range ModPlug", []byte{0x81, 8, 19, 0xa3, 0, 0x81, 11, 60, 1, 15, 0, 0}, true, false, false, 21, 12},
		{"out of range loop", []byte{0x81, 8, 19, 0xa3, 0, 0x81, 11, 60, 1, 15, 0, 0}, false, false, true, 21, 12},
		{"old effects out of range", []byte{0x81, 8, 19, 0xa3, 0, 0x81, 11, 60, 1, 15, 0, 0}, false, true, false, 21, 0},
		{"old effects out of loop", []byte{0x81, 8, 19, 0xa3, 0, 0x81, 11, 60, 1, 15, 0, 0}, false, true, true, 21, 12},
		{"SA0 clears high offset", []byte{0x81, 8, 19, 0xa1, 0, 0x81, 8, 19, 0xa0, 0, 0x81, 11, 60, 1, 15, 0, 0}, false, false, false, 41, 12},
		{"S00 recalls high offset", []byte{0x81, 8, 19, 0xa1, 0, 0x81, 8, 19, 0, 0, 0x81, 11, 60, 1, 15, 0, 0}, false, false, false, 41, 6},
		{"offset with instrument only", []byte{0x81, 3, 60, 1, 0, 0x81, 10, 1, 15, 1, 0}, false, false, false, 21, 6},
		{"legacy offset with instrument only", []byte{0x81, 3, 60, 1, 0, 0x81, 10, 1, 15, 1, 0}, true, false, false, 21, 12},
		{"offset without note or instrument", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 15, 1, 0}, false, false, false, 21, 12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := testIT()
			b[50] = 1
			binary.LittleEndian.PutUint32(b[256+48:], 100000)
			if tt.loop {
				b[256+18] |= 0x10
				binary.LittleEndian.PutUint32(b[256+52:], 1024)
				binary.LittleEndian.PutUint32(b[256+56:], 90000)
			}
			if tt.legacy {
				binary.LittleEndian.PutUint16(b[40:], 0x217)
				binary.LittleEndian.PutUint16(b[42:], 0x200)
			}
			if tt.oldEffects {
				b[44] |= 0x10
			}
			b = append(b[:512], make([]byte, 200000)...)
			for i := range 100000 {
				period := 16.0
				if i >= 65536 {
					period = 32
				}
				if i >= 256 && i < 768 {
					period = 32
				} else if i >= 65536+256 && i < 65536+512 {
					period = 16
				}
				value := int16(math.Sin(float64(i)*2*math.Pi/period) * 16000)
				binary.LittleEndian.PutUint16(b[512+i*2:], uint16(value))
			}
			packed := append(append([]byte(nil), tt.rows...), make([]byte, 62)...)
			binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
			copy(b[344:], packed)
			ref, actual := referencePCMWithInterpolation(t, b, "2")
			crossings := func(pcm []float32) int {
				count := 0
				for i := tt.startMS*48 + 1; i < (tt.startMS+4)*48; i++ {
					if pcm[(i-1)*2] <= 0 && pcm[i*2] > 0 {
						count++
					}
				}
				return count
			}
			want, got := crossings(ref), crossings(actual)
			if absInt(want-tt.want) > 1 {
				t.Fatalf("fixture crossings: OpenMPT %d, expected about %d", want, tt.want)
			}
			if absInt(want-got) > 1 {
				t.Errorf("crossings: OpenMPT %d, tracker %d", want, got)
			}
		})
	}
}

func TestITOffsetMemoryAndIsolation(t *testing.T) {
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 2,
		Samples: []SampleAsset{{Data: make([]float32, 140000), Volume: 64, C5Speed: 48000}},
	}, 48000, 256)
	step := func(track int, cmd ModCommand) {
		t.Helper()
		w.PendingCmds = [MaxTracks]ModCommand{}
		w.PendingCmds[track] = cmd
		VoiceAllocationSystem(w)
		VolumeEffectSystem(w)
	}
	step(0, ModCommand{EffectCmd: 0x15, EffectArg: 0xa1})
	step(1, ModCommand{EffectCmd: 0x12, EffectArg: 2})
	step(0, ModCommand{Note: 61, Instrument: 1, EffectCmd: 0x12, EffectArg: 1})
	v0 := w.TrackCursors[0].ActiveVoice
	if got := w.VoiceSamples[v0].Position; got != 65536+256 {
		t.Fatalf("high and low offset = %v", got)
	}
	step(1, ModCommand{Note: 61, Instrument: 1, EffectCmd: 0x12})
	v1 := w.TrackCursors[1].ActiveVoice
	if got := w.VoiceSamples[v1].Position; got != 512 {
		t.Fatalf("second track offset = %v", got)
	}
	step(0, ModCommand{EffectCmd: 0x15, EffectArg: 0xa0})
	step(0, ModCommand{EffectCmd: 0x15})
	step(0, ModCommand{Note: 61, EffectCmd: 0x12})
	v0 = w.TrackCursors[0].ActiveVoice
	if got := w.VoiceSamples[v0].Position; got != 256 {
		t.Fatalf("SA0 then S00 should leave low offset only, got %v", got)
	}
	if w.TrackMemories[1].ITHighOffset != 0 || w.TrackMemories[1].LastOffset != 2 || w.VoiceSamples[v1].Position != 512 {
		t.Fatal("offset state leaked between tracks")
	}
}

func TestITOldVersionHighOffsetDoesNotChangeSpecialMemory(t *testing.T) {
	b := testIT()
	binary.LittleEndian.PutUint16(b[40:], 0x100)
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	for _, arg := range []byte{0x91, 0xa1, 0} {
		w.PendingCmds[0] = ModCommand{EffectCmd: 0x15, EffectArg: arg}
		VolumeEffectSystem(w)
	}
	if !w.TrackCursors[0].Surround || w.TrackMemories[0].ITLastSpecial != 0x91 || w.TrackMemories[0].ITHighOffset != 0 {
		t.Fatalf("old IT high-offset command altered state: %+v", w.TrackMemories[0])
	}
}

func TestITOffsetPackedEffectReuse(t *testing.T) {
	b := testIT()
	packed := []byte{0x81, 8, 19, 0xa0, 0, 0x81, 0x80, 0, 0x81, 8, 15, 0, 0, 0x81, 0x80, 0}
	packed = append(packed, make([]byte, 60)...)
	binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
	copy(b[344:], packed)
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	rows := loaded.Assets.Patterns[0].Rows
	for i, want := range []ModCommand{{EffectCmd: 0x15, EffectArg: 0xa0}, {EffectCmd: 0x15, EffectArg: 0xa0}, {EffectCmd: 0x12}, {EffectCmd: 0x12}} {
		if got := rows[i][0]; got != want {
			t.Errorf("row %d: got %+v, want %+v", i, got, want)
		}
	}

}

func TestITLegacyHighOffsetPreservesBackgroundVoice(t *testing.T) {
	instrument := ITInstrument{NewNoteAction: 1}
	for i := range instrument.Samples {
		instrument.Samples[i] = 1
		instrument.Notes[i] = uint8(i)
	}
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 1, ITCompatibility: ITCompatibility{LegacyModPlug: true},
		ITInstruments: []ITInstrument{instrument},
		Samples:       []SampleAsset{{Data: make([]float32, 140000), Volume: 64, C5Speed: 48000}},
	}, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	old := w.TrackCursors[0].ActiveVoice
	w.VoiceSamples[old].Position = 1234
	w.PendingCmds[0] = ModCommand{Note: 61, EffectCmd: 0x15, EffectArg: 0xa1}
	VoiceAllocationSystem(w)
	VolumeEffectSystem(w)
	current := w.TrackCursors[0].ActiveVoice
	if old == current || !w.VoiceLives[old].Active || w.VoiceSamples[old].Position != 1234 || w.VoiceSamples[current].Position != 65536 {
		t.Fatal("SA1 did not isolate the new voice from the background voice")
	}
}

func TestITLegacyHighOffsetLoopBoundary(t *testing.T) {
	for _, tt := range []struct {
		loopEnd uint32
		want    float64
	}{{65536, 0}, {65537, 65536}} {
		w := NewTracker(SongAssets{
			IT: true, NumTracks: 1, ITCompatibility: ITCompatibility{LegacyModPlug: true},
			Samples: []SampleAsset{{Data: make([]float32, 70000), LoopEnable: true, LoopEnd: tt.loopEnd, Volume: 64, C5Speed: 48000}},
		}, 48000, 256)
		w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1, EffectCmd: 0x15, EffectArg: 0xa1}
		VoiceAllocationSystem(w)
		VolumeEffectSystem(w)
		if got := w.VoiceSamples[w.TrackCursors[0].ActiveVoice].Position; got != tt.want {
			t.Errorf("loop end %d: position %v, want %v", tt.loopEnd, got, tt.want)
		}
	}
}
