package mod

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erik-adelbert/duh/pkg/mod/internal/testfixtures"
)

func TestITPatternVolumeCommandRanges(t *testing.T) {
	for _, mptm := range []bool{false, true} {
		for volume := 0; volume <= 255; volume++ {
			pattern, err := unpackITPatternMode([]byte{0x81, 4, byte(volume), 0}, 1, mptm)
			supported := volume <= 124 || volume >= 128 && volume <= 202 ||
				volume == 203 || !mptm && volume >= 204 && volume <= 212 ||
				mptm && volume >= 224 && volume <= 232
			if (err == nil) != supported {
				t.Fatalf("mptm=%t volume=%d: unexpected error %v", mptm, volume, err)
			}
			if supported && (!pattern.Rows[0][0].HasVolume || pattern.Rows[0][0].Volume != byte(volume)) {
				t.Fatalf("mptm=%t volume=%d: unexpected command %+v", mptm, volume, pattern.Rows[0][0])
			}
		}
	}
}

func TestITPatternEffectProfiles(t *testing.T) {
	tests := []struct {
		effect, arg byte
		it, mptm    byte
	}{
		{9, 0x12, 0x22, 0x22}, {11, 0x12, 0x11, 0x11}, {12, 0x12, 0x1f, 0x1f},
		{14, 0x12, 0x1e, 0x1e}, {16, 0x12, 0x1d, 0x1d}, {17, 0, 0x1b, 0x1a},
		{17, 0x12, 0x1b, 0x1b}, {17, 0x63, 0x1b, 0x1b}, {17, 0xc2, 0x1b, 0x1b}, {18, 0x12, 0x23, 0x23},
		{19, 0, 0x15, 0x15}, {19, 0x0f, 0x15, 0}, {19, 0x10, 0x15, 0x15},
		{19, 0x1f, 0x15, 0x15}, {19, 0x20, 0x15, 0x15}, {19, 0x30, 0x15, 0x15}, {19, 0x3f, 0x15, 0x15},
		{19, 0x40, 0x15, 0x15}, {19, 0x4f, 0x15, 0x15}, {19, 0x50, 0x15, 0x15}, {19, 0x5f, 0x15, 0x15}, {19, 0x60, 0, 0x15},
		{19, 0x73, 0x15, 0x15}, {19, 0x76, 0x15, 0x15}, {19, 0x77, 0x15, 0x15}, {19, 0x78, 0x15, 0x15}, {19, 0x7a, 0x15, 0x15}, {19, 0x7b, 0x15, 0x15}, {19, 0x7c, 0x15, 0x15}, {19, 0x7d, 0, 0}, {19, 0x80, 0x15, 0x15}, {19, 0x9f, 0, 0x15},
		{19, 0xe0, 0x15, 0x15}, {19, 0xef, 0x15, 0x15}, {19, 0xf0, 0x15, 0x15}, {19, 0xff, 0x15, 0x15},
		{20, 0x12, 0x20, 0x20}, {20, 0x20, 0x20, 0x20},
		{21, 0x12, 0x21, 0x21}, {23, 0x12, 0x1c, 0x1c},
		{26, 0x7f, 0x19, 0x19}, {26, 0x80, 0x19, 0x19}, {26, 0xff, 0x19, 0x19},
		{28, 0x7f, 0x26, 0x26}, {28, 0x80, 0x26, 0x26}, {28, 0xff, 0x26, 0x26},
	}
	for _, tt := range tests {
		for _, mptm := range []bool{false, true} {
			want := tt.it
			if mptm {
				want = tt.mptm
			}
			pattern, err := unpackITPatternMode([]byte{0x81, 8, tt.effect, tt.arg, 0, 0x81, 0x80, 0}, 2, mptm)
			if want == 0 {
				if err == nil {
					t.Fatalf("mptm=%t effect=%d arg=%02x: expected rejection", mptm, tt.effect, tt.arg)
				}
				continue
			}
			if err != nil {
				t.Fatalf("mptm=%t effect=%d arg=%02x: %v", mptm, tt.effect, tt.arg, err)
			}
			for row := range 2 {
				cmd := pattern.Rows[row][0]
				if cmd.EffectCmd != want || cmd.EffectArg != tt.arg {
					t.Fatalf("mptm=%t effect=%d arg=%02x row=%d: unexpected command %+v", mptm, tt.effect, tt.arg, row, cmd)
				}
			}
		}
	}
}

func TestITSampleMetadata(t *testing.T) {
	h := make([]byte, 80)
	h[17], h[19], h[47] = 100, 100, 128|32
	h[76], h[77], h[78], h[79] = 4, 0x85, 6, 0x83
	for _, tt := range []struct {
		stored, want uint32
	}{
		{0, 8363}, {1, 256}, {48000, 48000},
	} {
		binary.LittleEndian.PutUint32(h[60:], tt.stored)
		got := itSampleMetadata(h)
		if got.C5Speed != tt.want || got.Volume != 64 || got.ITGlobalVolume != 64 ||
			got.ITPanning != 32 || !got.ITHasPanning || got.ITVibratoRate != 4 ||
			got.ITVibratoDepth != 5 || got.ITVibratoSweep != 6 || got.ITVibratoType != 3 {
			t.Errorf("stored speed %d: unexpected metadata %+v", tt.stored, got)
		}
	}
}

func uncompressedMMCMP(data []byte) []byte {
	const block = 28
	const payload = block + 20 + 8
	b := make([]byte, payload+len(data))
	copy(b, "ziRCONia")
	binary.LittleEndian.PutUint16(b[8:], 14)
	binary.LittleEndian.PutUint16(b[10:], 0x1300)
	binary.LittleEndian.PutUint16(b[12:], 1)
	binary.LittleEndian.PutUint32(b[14:], uint32(len(data)))
	binary.LittleEndian.PutUint32(b[18:], 24)
	binary.LittleEndian.PutUint32(b[24:], block)
	binary.LittleEndian.PutUint32(b[block:], uint32(len(data)))
	binary.LittleEndian.PutUint32(b[block+4:], uint32(len(data)))
	binary.LittleEndian.PutUint16(b[block+12:], 1)
	binary.LittleEndian.PutUint32(b[block+24:], uint32(len(data)))
	copy(b[payload:], data)
	return b
}

func TestLoadITMMCMP(t *testing.T) {
	for _, tt := range []struct {
		name    string
		data    []byte
		wantErr string
	}{
		{"valid", uncompressedMMCMP(testIT()), ""},
		{"invalid header", []byte("ziRCONia"), "invalid MMCMP container"},
		{"truncated block", uncompressedMMCMP(testIT())[:60], "decompressing MMCMP container"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "module.it")
			if err := os.WriteFile(path, tt.data, 0600); err != nil {
				t.Fatal(err)
			}
			loaded, err := LoadIT(path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("got error %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := loaded.Assets.Patterns[0].Rows[0][0]; got.Note != 61 || got.Instrument != 1 {
				t.Fatalf("unexpected decoded command: %+v", got)
			}
		})
	}
}

func TestLoadITMMCMPGrow(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "mmcmp", "grow.it")
	loaded, err := LoadIT(path)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 1024)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	buf := make([]float32, 1024*2)
	nonzero := false
	for remaining := 48000 * 10; remaining > 0; {
		frames := w.Read(buf, min(remaining, 1024))
		if frames == 0 {
			t.Fatal("song ended before 10 seconds")
		}
		for _, sample := range buf[:frames*2] {
			if math.IsNaN(float64(sample)) || math.IsInf(float64(sample), 0) {
				t.Fatal("song produced non-finite audio")
			}
			nonzero = nonzero || sample != 0
		}
		remaining -= frames
	}
	if !nonzero {
		t.Fatal("song produced only silence")
	}
}

func setTestEnvelope(h []byte, offset int, flags byte, values []int8, ticks []uint16) {
	h[offset], h[offset+1] = flags, byte(len(values))
	for i, value := range values {
		h[offset+6+i*3] = byte(value)
		binary.LittleEndian.PutUint16(h[offset+7+i*3:], ticks[i])
	}
}

func TestITPanPitchEnvelopePlayback(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	h[304] = 0
	setTestEnvelope(h, 386, 5, []int8{-32, 0, 32}, []uint16{0, 2, 4})
	setTestEnvelope(h, 468, 3, []int8{32, 0, -32}, []uint16{0, 2, 4})
	h[390], h[391] = 1, 1
	h[470], h[471] = 0, 1
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	for tick, want := range []struct{ pan, pitch float32 }{{-32, 32}, {-16, 16}, {0, 0}, {0, 32}} {
		VoiceLifecycleSystem(w)
		VoiceKinematicsSystem(w)
		life, amp := w.VoiceLives[v], w.VoiceAmps[v]
		if life.ITPanEnvelopeValue != want.pan || life.ITPitchEnvelopeValue != want.pitch {
			t.Fatalf("tick %d: pan=%v pitch=%v", tick, life.ITPanEnvelopeValue, life.ITPitchEnvelopeValue)
		}
		balance := (amp.TargetRight - amp.TargetLeft) / (amp.TargetRight + amp.TargetLeft)
		if math.Abs(float64(balance-want.pan/32)) > 1e-6 {
			t.Fatalf("tick %d: balance %v", tick, balance)
		}
		if ratio := w.VoicePitches[v].StepIncrement; math.Abs(ratio-math.Pow(2, float64(want.pitch)/24)) > 1e-9 {
			t.Fatalf("tick %d: pitch ratio %v", tick, ratio)
		}
	}
	w.PendingCmds[0] = ModCommand{Note: 255}
	VoiceAllocationSystem(w)
	VoiceLifecycleSystem(w)
	VoiceLifecycleSystem(w)
	if got := w.VoiceLives[v].ITPanEnvelopeValue; got != 16 {
		t.Fatalf("pan sustain did not release: %v", got)
	}
}

func TestITPanPitchEnvelopeCarry(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	h[304] = 0
	setTestEnvelope(h, 386, 9, []int8{-32, 32}, []uint16{0, 8})
	setTestEnvelope(h, 468, 9, []int8{32, -32}, []uint16{0, 8})
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	old := w.TrackCursors[0].ActiveVoice
	VoiceLifecycleSystem(w)
	VoiceLifecycleSystem(w)
	w.PendingCmds[0] = ModCommand{Note: 62}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	VoiceLifecycleSystem(w)
	if old == v || w.VoiceLives[old].ITPanEnvelopeValue != -16 || w.VoiceLives[v].ITPanEnvelopeValue != -16 || w.VoiceLives[v].ITPitchEnvelopeValue != 16 {
		t.Fatalf("envelope carry: old=%+v new=%+v", w.VoiceLives[old], w.VoiceLives[v])
	}
}

func TestOpenMPTITPanPitchEnvelopes(t *testing.T) {
	for _, tt := range []struct {
		name       string
		pan, pitch int8
	}{
		{"left", -16, 0}, {"right", 16, 0},
		{"pitch up", 0, 16}, {"pitch down", 0, -16},
		{"both", 16, 16},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := testITInstrument()
			h := b[len(b)-554:]
			h[304] = 0
			setTestEnvelope(h, 386, 1, []int8{tt.pan}, []uint16{0})
			setTestEnvelope(h, 468, 1, []int8{tt.pitch}, []uint16{0})
			ref, actual := referencePCM(t, b)
			balance := func(pcm []float32) float64 {
				l, r := channelRMS(pcm, 30, 80, 0), channelRMS(pcm, 30, 80, 1)
				return (r - l) / (r + l)
			}
			if want, got := balance(ref), balance(actual); math.Abs(want-got) > .035 {
				t.Errorf("balance: OpenMPT %.3f, tracker %.3f", want, got)
			}
			crossings := func(pcm []float32) int {
				count := 0
				for i := 30*48 + 1; i < 80*48; i++ {
					if pcm[(i-1)*2] <= 0 && pcm[i*2] > 0 {
						count++
					}
				}
				return count
			}
			if want, got := crossings(ref), crossings(actual); absInt(want-got) > 1 {
				t.Errorf("pitch crossings: OpenMPT %d, tracker %d", want, got)
			}
		})
	}
}

func TestOpenMPTITEnvelopeMovement(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	h[304] = 0
	setTestEnvelope(h, 386, 1, []int8{-32, 32}, []uint16{0, 2})
	setTestEnvelope(h, 468, 1, []int8{0, 16}, []uint16{0, 2})
	ref, actual := referencePCM(t, b)
	for _, window := range [][2]int{{5, 15}, {65, 75}} {
		balance := func(pcm []float32) float64 {
			l, r := channelRMS(pcm, window[0], window[1], 0), channelRMS(pcm, window[0], window[1], 1)
			return (r - l) / (r + l)
		}
		want, got := balance(ref), balance(actual)
		if math.Abs(want-got) > .05 {
			t.Errorf("%dms: OpenMPT balance %.3f, tracker %.3f", window[0], want, got)
		}
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func testITInstrument() []byte {
	b := testIT()
	instrument := len(b)
	b = append(b, make([]byte, 554)...)
	binary.LittleEndian.PutUint16(b[34:], 1)
	b[44] |= 4
	binary.LittleEndian.PutUint32(b[194:], uint32(instrument))
	binary.LittleEndian.PutUint32(b[198:], 256)
	binary.LittleEndian.PutUint32(b[202:], 336)
	h := b[instrument:]
	copy(h, "IMPI")
	h[17], h[24], h[25] = 1, 128, 128
	for i := range 120 {
		h[64+i*2], h[65+i*2] = byte(i), 1
	}
	h[304], h[305] = 1, 3
	h[310], h[313], h[316] = 64, 32, 0
	binary.LittleEndian.PutUint16(h[314:], 2)
	binary.LittleEndian.PutUint16(h[317:], 4)
	b[256+18] |= 0x10
	binary.LittleEndian.PutUint32(b[256+56:], 4096)
	return b
}

func TestITInstrumentMappingAndEnvelope(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	h[64+60*2] = 72
	h[304], h[308], h[309] = 5, 1, 1
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	if v < 0 || w.VoicePitches[v].Note != 73 || w.VoiceSamples[v].SampleID != 0 {
		t.Fatalf("mapping: voice=%d", v)
	}
	for _, want := range []float32{1, .75, .5, .5} {
		VoiceLifecycleSystem(w)
		if got := w.VoiceLives[v].ITEnvelopeVolume; got != want {
			t.Fatalf("envelope = %v, want %v", got, want)
		}
	}
	w.PendingCmds[0] = ModCommand{Note: 255}
	VoiceAllocationSystem(w)
	VoiceLifecycleSystem(w)
	if !w.VoiceLives[v].Active || w.VoiceLives[v].ITEnvelopeVolume != .5 {
		t.Fatal("key-off did not release sustain")
	}
	VoiceLifecycleSystem(w)
	VoiceLifecycleSystem(w)
	if w.VoiceLives[v].Active {
		t.Fatal("envelope should end after release")
	}
}

func TestITInstrumentPitchPan(t *testing.T) {
	for _, tt := range []struct {
		name       string
		separation int8
		note       byte
		want       float32
	}{
		{"center", 16, 61, 0},
		{"higher", 16, 73, .75},
		{"lower", 16, 49, -.75},
		{"negative separation", -16, 73, -.75},
		{"clamp right", 16, 100, 1},
		{"clamp left", -16, 100, -1},
		{"integer rounding", 5, 62, 2.0 / 128},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := testITInstrument()
			h := b[len(b)-554:]
			h[22], h[23] = byte(tt.separation), 60
			loaded, err := loadIT(b)
			if err != nil {
				t.Fatal(err)
			}
			if got := loaded.Assets.ITInstruments[0].PitchPanSeparation; got != tt.separation {
				t.Fatalf("separation = %d, want %d", got, tt.separation)
			}
			w := NewTracker(loaded.Assets, 48000, 256)
			w.PendingCmds[0] = ModCommand{Note: tt.note, Instrument: 1}
			VoiceAllocationSystem(w)
			v := w.TrackCursors[0].ActiveVoice
			if got := w.VoiceAmps[v].Panning; got != tt.want {
				t.Fatalf("panning = %v, want %v", got, tt.want)
			}
			if got := w.TrackCursors[0].Panning; got != 0 {
				t.Fatalf("base panning changed to %v", got)
			}
		})
	}
}

func TestITPitchPanOverrideAndBackgroundVoice(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	h[22], h[23] = 16, 60
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 73, Instrument: 1}
	VoiceAllocationSystem(w)
	old := w.TrackCursors[0].ActiveVoice
	w.PendingCmds[0] = ModCommand{Note: 49, HasVolume: true, Volume: 160}
	VoiceAllocationSystem(w)
	current := w.TrackCursors[0].ActiveVoice
	if w.VoiceAmps[old].Panning != .75 || w.VoiceAmps[current].Panning != 0 {
		t.Fatalf("old pan %v, overridden pan %v", w.VoiceAmps[old].Panning, w.VoiceAmps[current].Panning)
	}
	w.PendingCmds[0] = ModCommand{Note: 61}
	VoiceAllocationSystem(w)
	if pan := w.VoiceAmps[w.TrackCursors[0].ActiveVoice].Panning; pan != 0 {
		t.Fatalf("new note accumulated pitch-pan offset: %v", pan)
	}
}

func TestOpenMPTITPitchPan(t *testing.T) {
	for _, tt := range []struct {
		name       string
		separation int8
		note       byte
		mapped     byte
		volume     byte
		legacy     bool
	}{
		{"positive", 16, 72, 72, 32, false},
		{"negative", -16, 72, 72, 32, false},
		{"lower", 16, 48, 48, 32, false},
		{"clamped", 16, 90, 90, 32, false},
		{"mapped", 16, 72, 60, 32, false},
		{"pan override", 16, 72, 72, 160, false},
		{"legacy mapped", 16, 72, 60, 32, true},
		{"legacy positive", 16, 72, 72, 32, true},
		{"legacy negative", -5, 72, 72, 32, true},
		{"legacy pan", 16, 72, 72, 160, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := testITInstrument()
			h := b[len(b)-554:]
			h[22], h[23] = byte(tt.separation), 60
			h[64+int(tt.note)*2] = tt.mapped
			b[344+2], b[344+4] = tt.note, tt.volume
			if tt.legacy {
				binary.LittleEndian.PutUint16(b[40:], 0x217)
				binary.LittleEndian.PutUint16(b[42:], 0x200)
			}
			ref, actual := referencePCM(t, b)
			balance := func(pcm []float32) float64 {
				var left, right float64
				for frame := 1920; frame < 2640; frame++ {
					left += float64(pcm[frame*2]) * float64(pcm[frame*2])
					right += float64(pcm[frame*2+1]) * float64(pcm[frame*2+1])
				}
				left, right = math.Sqrt(left), math.Sqrt(right)
				if left+right == 0 {
					t.Fatal("silent comparison")
				}
				return (right - left) / (right + left)
			}
			want, got := balance(ref), balance(actual)
			if math.Abs(got-want) > .035 {
				t.Fatalf("stereo balance tracker %.3f, OpenMPT %.3f", got, want)
			}
		})
	}
}

func TestITInstrumentNewNoteActions(t *testing.T) {
	for _, action := range []byte{0, 1, 2, 3} {
		b := testITInstrument()
		b[len(b)-554+17] = action
		loaded, err := loadIT(b)
		if err != nil {
			t.Fatal(err)
		}
		w := NewTracker(loaded.Assets, 48000, 256)
		w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
		VoiceAllocationSystem(w)
		old := w.TrackCursors[0].ActiveVoice
		w.PendingCmds[0].Note = 62
		VoiceAllocationSystem(w)
		if life := w.VoiceLives[old]; action == 0 && life.Active && old != w.TrackCursors[0].ActiveVoice ||
			action != 0 && (!life.Active || old == w.TrackCursors[0].ActiveVoice) ||
			action == 2 && !life.KeyOff || action == 3 && !life.ITFade {
			t.Fatalf("NNA %d: old voice=%d new=%d lifecycle=%+v", action, old, w.TrackCursors[0].ActiveVoice, life)
		}
	}
}

func TestITInstrumentRejectsUnsupportedFeatures(t *testing.T) {
	for _, tt := range []struct {
		name string
		edit func([]byte)
		want string
	}{
		{"invalid new-note action", func(h []byte) { h[17] = 4 }, "duplicate-note"},
		{"invalid duplicate check", func(h []byte) { h[18] = 4 }, "duplicate-note"},
		{"invalid duplicate action", func(h []byte) { h[18], h[19] = 1, 3 }, "duplicate-note"},
		{"invalid panning envelope", func(h []byte) { h[386] = 1 }, "panning envelope"},
		{"invalid pitch value", func(h []byte) {
			setTestEnvelope(h, 468, 1, []int8{-33}, []uint16{0})
		}, "pitch envelope: invalid point"},
		{"invalid filter envelope", func(h []byte) { h[468] = 0x81 }, "pitch envelope"},
		{"invalid sample mapping", func(h []byte) { h[65] = 2 }, "invalid note mapping"},
		{"invalid envelope loop", func(h []byte) { h[304], h[307] = 3, 10 }, "volume envelope: invalid loop"},
		{"invalid envelope node", func(h []byte) { h[313] = 65 }, "volume envelope: invalid point"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := testITInstrument()
			tt.edit(b[len(b)-554:])
			if _, err := loadIT(b); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got error %v, want %q", err, tt.want)
			}
		})
	}
}

func TestITInstrumentRandomVariation(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	h[17], h[24], h[26], h[27] = 1, 64, 100, 64
	b[256+19] = 32
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	inst := loaded.Assets.ITInstruments[0]
	if inst.VolumeSwing != 100 || inst.PanSwing != 64 {
		t.Fatalf("loaded swing = %d, %d", inst.VolumeSwing, inst.PanSwing)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	w2 := NewTracker(loaded.Assets, 48000, 256)
	var panMin, panMax, volMin, volMax int16
	for range 128 {
		for _, world := range []*Tracker{w, w2} {
			world.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
			VoiceAllocationSystem(world)
		}
		v, v2 := w.TrackCursors[0].ActiveVoice, w2.TrackCursors[0].ActiveVoice
		a, a2 := w.VoiceAmps[v], w2.VoiceAmps[v2]
		if a.ITPanSwing != a2.ITPanSwing || a.ITVolumeSwing != a2.ITVolumeSwing {
			t.Fatal("independent players did not produce the same deterministic sequence")
		}
		panMin, panMax = min(panMin, a.ITPanSwing), max(panMax, a.ITPanSwing)
		volMin, volMax = min(volMin, a.ITVolumeSwing), max(volMax, a.ITVolumeSwing)
		w.VoiceLives[v].Active, w2.VoiceLives[v2].Active = false, false
	}
	if panMin > -220 || panMax < 200 || volMin > -28 || volMax < 27 {
		t.Fatalf("unexpected variation range: pan %d..%d volume %d..%d", panMin, panMax, volMin, volMax)
	}
}

func TestITRandomVariationPanningOverride(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		b := testITInstrument()
		h := b[len(b)-554:]
		h[27] = 64
		if legacy {
			binary.LittleEndian.PutUint16(b[40:], 0x217)
			binary.LittleEndian.PutUint16(b[42:], 0x200)
		}
		loaded, err := loadIT(b)
		if err != nil {
			t.Fatal(err)
		}
		w := NewTracker(loaded.Assets, 48000, 256)
		w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
		VoiceAllocationSystem(w)
		old := w.TrackCursors[0].ActiveVoice
		swing := w.VoiceAmps[old].ITPanSwing
		if swing == 0 {
			t.Fatal("fixture did not generate pan variation")
		}
		w.PendingCmds[0] = ModCommand{Note: 61, HasVolume: true, Volume: 160}
		VoiceAllocationSystem(w)
		newVoice := w.TrackCursors[0].ActiveVoice
		if w.VoiceAmps[old].ITPanSwing != swing || (w.VoiceAmps[newVoice].ITPanSwing == 0) == legacy {
			t.Fatalf("legacy=%v old swing=%d new swing=%d", legacy, w.VoiceAmps[old].ITPanSwing, w.VoiceAmps[newVoice].ITPanSwing)
		}
	}
}

func TestITRandomVariationClamping(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	h[26], h[27] = 255, 255
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	if inst := loaded.Assets.ITInstruments[0]; inst.VolumeSwing != 100 || inst.PanSwing != 64 {
		t.Fatalf("swing limits = %d, %d", inst.VolumeSwing, inst.PanSwing)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	w.VoiceAmps[v].ITVolumeSwing = -128
	w.VoiceAmps[v].ITPanSwing = 256
	VoiceKinematicsSystem(w)
	if amp := w.VoiceAmps[v]; amp.TargetLeft != 0 || amp.TargetRight != 0 {
		t.Fatalf("negative volume should clamp: %+v", amp)
	}
	w.VoiceAmps[v].ITVolumeSwing = 128
	VoiceKinematicsSystem(w)
	if amp := w.VoiceAmps[v]; amp.TargetLeft != 0 || amp.TargetRight <= 0 {
		t.Fatalf("pan should clamp right: %+v", amp)
	}
}

func TestOpenMPTITRandomVariationRange(t *testing.T) {
	for _, tt := range []struct {
		name        string
		legacy      bool
		volumeSwing byte
		panSwing    byte
	}{
		{"IT volume", false, 100, 0},
		{"IT panning", false, 0, 64},
		{"ModPlug volume", true, 64, 0},
		{"ModPlug panning", true, 0, 64},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := testITInstrument()
			h := b[len(b)-554:]
			h[24], h[26], h[27] = 64, tt.volumeSwing, tt.panSwing
			b[256+19] = 32
			b[344+4] = 32
			if tt.legacy {
				binary.LittleEndian.PutUint16(b[40:], 0x217)
				binary.LittleEndian.PutUint16(b[42:], 0x200)
			}
			zero := append([]byte(nil), b...)
			zero[len(zero)-554+26], zero[len(zero)-554+27] = 0, 0
			refBase, actualBase := referencePCM(t, zero)
			baseL := channelRMS(refBase, 40, 55, 0)
			baseR := channelRMS(refBase, 40, 55, 1)
			baseActual := channelRMS(actualBase, 40, 55, 0)
			if baseL == 0 || baseR == 0 || baseActual == 0 {
				t.Fatal("silent comparison baseline")
			}
			loaded, err := loadIT(b)
			if err != nil {
				t.Fatal(err)
			}
			w := NewTracker(loaded.Assets, 48000, 256)
			low, high := 10.0, -10.0
			for range 256 {
				w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1, HasVolume: true, Volume: 32}
				VoiceAllocationSystem(w)
				v := w.TrackCursors[0].ActiveVoice
				w.VoiceLives[v].ITEnvelopeVolume = .75
				VoiceKinematicsSystem(w)
				a := &w.VoiceAmps[v]
				value := float64(a.TargetLeft + a.TargetRight)
				if tt.panSwing != 0 {
					value = float64((a.TargetRight - a.TargetLeft) / (a.TargetRight + a.TargetLeft))
				} else {
					value /= float64(.75 * .5 * .5 * loaded.Assets.ITMixGain)
				}
				low, high = math.Min(low, value), math.Max(high, value)
				w.VoiceLives[v].Active = false
			}
			for range 12 {
				ref, _ := referencePCM(t, b)
				l, r := channelRMS(ref, 40, 55, 0), channelRMS(ref, 40, 55, 1)
				value := (l + r) / (baseL + baseR)
				if tt.panSwing != 0 {
					if l+r == 0 {
						t.Fatal("silent pan fixture")
					}
					value = (r - l) / (r + l)
				}
				if value < low-.08 || value > high+.08 {
					t.Fatalf("OpenMPT variation %.3f outside tracker range %.3f..%.3f", value, low, high)
				}
			}
		})
	}
}

func channelRMS(pcm []float32, startMS, endMS, channel int) float64 {
	var sum float64
	start, end := startMS*referenceRate/1000, endMS*referenceRate/1000
	for frame := start; frame < end; frame++ {
		sample := float64(pcm[frame*2+channel])
		sum += sample * sample
	}
	return math.Sqrt(sum / float64(end-start))
}

func TestITInstrumentKeyOffFade(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	h[304] = 0
	binary.LittleEndian.PutUint16(h[20:], 128)
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	w.PendingCmds[0] = ModCommand{Note: 255}
	VoiceAllocationSystem(w)
	VoiceLifecycleSystem(w)
	if !w.VoiceLives[v].Active || w.VoiceLives[v].FadeOutVol != .875 {
		t.Fatalf("key-off fade = %+v", w.VoiceLives[v])
	}
}

func TestOpenMPTITInstrumentEnvelope(t *testing.T) {
	ref, actual := referencePCM(t, testITInstrument())
	for _, ms := range []int{45, 65, 85} {
		refBase, actualBase := rmsWindow(ref, 25, 35), rmsWindow(actual, 25, 35)
		if refBase == 0 || actualBase == 0 {
			t.Fatal("silent instrument baseline")
		}
		want := rmsWindow(ref, ms, ms+10) / refBase
		got := rmsWindow(actual, ms, ms+10) / actualBase
		if math.Abs(got-want) > .12 {
			t.Errorf("%dms: relative envelope level tracker %.3f OpenMPT %.3f", ms, got, want)
		}
	}
}

func testIT() []byte {
	return testfixtures.IT()
}

func TestLoadITPatternAndSample(t *testing.T) {
	loaded, err := loadIT(testIT())
	if err != nil {
		t.Fatal(err)
	}
	first := loaded.Assets.Patterns[0].Rows[0][0]
	second := loaded.Assets.Patterns[0].Rows[1][0]
	if first.Note != 61 || first.Instrument != 1 || !first.HasVolume || first.Volume != 32 || first != second {
		t.Fatalf("pattern fields not decoded/reused: first=%+v second=%+v", first, second)
	}
	sample := loaded.Assets.Samples[0]
	if len(sample.Data) != 4096 || sample.C5Speed != 48000 || sample.Data[24] < 0.48 {
		t.Fatalf("incorrect sample: length=%d C5=%d sample[24]=%g", len(sample.Data), sample.C5Speed, sample.Data[24])
	}
	w := NewTracker(loaded.Assets, 48000, 128)
	w.PendingCmds[0] = first
	VoiceAllocationSystem(w)
	VoiceKinematicsSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	if v < 0 || math.Abs(w.VoicePitches[v].StepIncrement-1) > 1e-10 {
		t.Fatalf("IT C-5 pitch: voice=%d", v)
	}
}

func TestLoadITRejectsUnsupportedAndTruncated(t *testing.T) {
	for _, tt := range []struct {
		name string
		edit func([]byte) []byte
	}{
		{"truncated header", func(b []byte) []byte { return b[:100] }},
		{"truncated pointer table", func(b []byte) []byte { return b[:198] }},
		{"instrument mode", func(b []byte) []byte { b[44] |= 4; return b }},
		{"invalid compressed sample", func(b []byte) []byte { b[256+18] |= 8; return b }},
		{"truncated sample", func(b []byte) []byte { return b[:len(b)-1] }},
		{"unsupported volume", func(b []byte) []byte { b[336+8+4] = 125; return b }},
		{"truncated pattern", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[336:], 1); return b }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := loadIT(tt.edit(testIT())); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestITPatternBreakUsesBinaryRow(t *testing.T) {
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 1, OrderList: []int{0, 1},
		Patterns: []PatternAsset{{Rows: make([][]ModCommand, 64)}, {Rows: make([][]ModCommand, 100)}},
	}, 48000, 1)
	w.Clock.Speed = 1
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x0d, EffectArg: 0x42}
	GlobalFlowSystem(w)
	w.Clock.SamplesRemaining = 0
	if !TickSchedulerSystem(w) || w.Clock.Order != 1 || w.Clock.Row != 66 {
		t.Fatalf("break position = %d:%d", w.Clock.Order, w.Clock.Row)
	}
}

func TestITBreakFinishesCurrentTick(t *testing.T) {
	w := NewTracker(SongAssets{
		IT: true, ITMixGain: 1, ITGlobalVolume: 128, ITChannelVolume: [MaxTracks]uint8{64},
		NumTracks: 1, OrderList: []int{0},
		Patterns: []PatternAsset{{Rows: [][]ModCommand{{{Note: 61, Instrument: 1, EffectCmd: 0x0d}}}}},
		Samples:  []SampleAsset{{Data: make([]float32, 1024), C5Speed: 8363, Volume: 64, ITGlobalVolume: 64}},
	}, 48000, 1000)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 1, 125, -1
	out := make([]float32, 2000)
	if n := w.Read(out, 1000); n != 960 {
		t.Fatalf("last tick rendered %d frames, want 960", n)
	}
}

func TestOpenMPTITPitch(t *testing.T) {
	ref, actual := referencePCM(t, testIT())
	var xy, xx, yy float64
	for frame := 480; frame < 1440; frame++ {
		x, y := float64(ref[frame*2]), float64(actual[frame*2])
		xy += x * y
		xx += x * x
		yy += y * y
	}
	if correlation := xy / math.Sqrt(xx*yy); math.IsNaN(correlation) || correlation < 0.95 {
		t.Fatalf("IT C-5 waveform correlation with OpenMPT = %g", correlation)
	}
}

func TestOpenMPTITLegacyMixGain(t *testing.T) {
	b := testIT()
	binary.LittleEndian.PutUint16(b[40:], 0x217)
	binary.LittleEndian.PutUint16(b[42:], 0x200)
	packed := []byte{0x81, 7, 60, 1, 32, 0x94, 4, 0, 0}
	packed = append(packed, make([]byte, 63)...)
	binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
	copy(b[344:], packed)
	ref, actual := referencePCM(t, b)
	refLevel, actualLevel := rmsWindow(ref, 10, 30), rmsWindow(actual, 10, 30)
	if actualLevel == 0 || refLevel == 0 || math.Abs(actualLevel/refLevel-1) > 0.15 {
		t.Fatalf("legacy IT RMS: tracker %g, OpenMPT %g", actualLevel, refLevel)
	}
}
