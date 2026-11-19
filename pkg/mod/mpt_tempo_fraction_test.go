package mod

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestMPTDefaultTempoFraction(t *testing.T) {
	for _, tt := range []struct {
		fraction uint32
		valid    bool
	}{
		{0, true}, {3425, true}, {9999, true}, {10000, false}, {^uint32(0), false},
	} {
		field := binary.LittleEndian.AppendUint32(nil, tt.fraction)
		ext, err := parseMPTExtensions(append([]byte("STPM"), mptExtensionField("DTFR", 4, field)...), 0)
		if err != nil || ext.defaultTempoFraction == nil || *ext.defaultTempoFraction != tt.fraction {
			t.Fatalf("fraction %d: extensions %+v, error %v", tt.fraction, ext, err)
		}
		legacy, err := mptLegacySequenceDefaults(125, 6, nil, ext.defaultTempoFraction)
		if (err == nil) != tt.valid {
			t.Fatalf("fraction %d: %v", tt.fraction, err)
		}
		if !tt.valid {
			continue
		}
		if legacy.tempo != 1250000+tt.fraction {
			t.Fatalf("fraction %d: tempo %d", tt.fraction, legacy.tempo)
		}
		extended := uint32(980)
		legacy, err = mptLegacySequenceDefaults(125, 6, &extended, ext.defaultTempoFraction)
		if err != nil || legacy.tempo != 9800000+tt.fraction {
			t.Fatalf("extended tempo: %+v, %v", legacy, err)
		}
		override := uint32(1005000)
		result, err := resolveMPTSequenceDefaults(mptSequence{orders: []uint16{0}, defaultTempo: &override}, legacy)
		if err != nil || result.tempo != override {
			t.Fatalf("sequence override: %+v, %v", result, err)
		}
	}
	for _, field := range [][]byte{nil, {1}, {1, 2, 3}, {1, 2, 3, 4, 5}} {
		if _, err := parseMPTExtensions(append([]byte("STPM"), mptExtensionField("DTFR", uint16(len(field)), field)...), 0); err == nil {
			t.Fatalf("accepted fraction width %d", len(field))
		}
	}
}

func TestMPTFractionAndOPLRetriggerLoader(t *testing.T) {
	fixture := func(opl bool) []byte {
		b := mptmPlaybackFixture()
		if !opl {
			ptr := int(binary.LittleEndian.Uint32(b[198:]))
			b[ptr+18], b[ptr+46] = 1, 1
		}
		pos := bytesIndex(t, b, ".FSM")
		binary.LittleEndian.PutUint16(b[pos+4:], 14)
		tail := int(binary.LittleEndian.Uint32(b[len(b)-4:]))
		fields := make([]byte, 7)
		fields[6] = 1 << (mptOPLRealRetrigBit - 104)
		fields = append(fields, mptExtensionField("DTFR", 4, binary.LittleEndian.AppendUint32(nil, 3425))...)
		out := append([]byte(nil), b[:tail]...)
		out = append(out, fields...)
		out = append(out, b[tail:len(b)-4]...)
		return binary.LittleEndian.AppendUint32(out, uint32(tail+len(fields)))
	}
	loaded, err := loadMPTM(fixture(false))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.InitialTempo != 125.3425 {
		t.Fatalf("initial tempo = %v", loaded.InitialTempo)
	}
	if _, err := loadMPTM(fixture(true)); err == nil || !strings.Contains(err.Error(), "OPL retrigger") {
		t.Fatalf("expected OPL retrigger rejection, got %v", err)
	}
}

func TestMPTMTempoSlideReadChunking(t *testing.T) {
	flags := mptSavedPlayBehaviours((1<<mptKnownPlayBehaviourCount)-1) &^ (1 << mptMODVBlankTiming)
	b := mptSharedEffectFixture(t, []byte{
		0x81, 11, 60, 1, 20, 0x13, 0,
		0x81, 8, 20, 0, 0,
		0x81, 8, 20, 137, 0,
		0x81, 8, 20, 0, 0,
		0x81, 1, 254, 0,
	}, flags)
	b[50] = 7
	b[bytesIndex(t, b, "..MT")+6] = 2
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	const frames = 20000
	render := func(chunk int) (*Tracker, int, []float32) {
		t.Helper()
		w := NewTracker(loaded.Assets, referenceRate, 1024)
		w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
		out := make([]float32, frames*2)
		for pos := 0; pos < frames; {
			n := min(chunk, frames-pos)
			if got := w.Read(out[2*pos:2*(pos+n)], n); got != n {
				t.Fatalf("chunk %d: short render at %d: %d", chunk, pos, got)
			}
			pos += n
		}
		last := -1
		for i := frames - 1; i >= 0; i-- {
			if math.Abs(float64(out[2*i])) > 1e-6 || math.Abs(float64(out[2*i+1])) > 1e-6 {
				last = i
				break
			}
		}
		return w, last, out
	}
	want, wantCut, wantAudio := render(1024)
	if wantCut <= 0 || wantCut >= frames-1 || want.TrackCursors[0].ActiveVoice != -1 {
		t.Fatalf("missing note cut at frame %d", wantCut)
	}
	for _, chunk := range []int{1, 127} {
		got, gotCut, gotAudio := render(chunk)
		if got.Clock != want.Clock || gotCut != wantCut || got.TrackCursors[0].ActiveVoice != -1 {
			t.Fatalf("chunk %d: cut %d, want %d; clock %+v, want %+v", chunk, gotCut, wantCut, got.Clock, want.Clock)
		}
		for i := range wantAudio {
			if gotAudio[i] != wantAudio[i] {
				t.Fatalf("chunk %d sample %d: got %g, want %g", chunk, i, gotAudio[i], wantAudio[i])
			}
		}
	}
}
