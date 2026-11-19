package mod

import (
	"encoding/binary"
	"fmt"
	"slices"
	"testing"
)

func mptVolumeSlidePropagationFixture(t *testing.T, rows []byte, enabled bool, version uint32) []byte {
	t.Helper()
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	b := mptSharedEffectFixture(t, rows, flags)
	binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], version)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	field := bytesIndex(t, b, ".FSM")
	length := int(binary.LittleEndian.Uint16(b[field+4:]))
	data := make([]byte, 17)
	copy(data, b[field+6:field+6+length])
	if enabled {
		data[mptITVolColNoSlidePropagationBit/8] |= 1 << (mptITVolColNoSlidePropagationBit % 8)
	}
	out := append([]byte(nil), b[:field]...)
	out = append(out, mptExtensionField(".FSM", uint16(len(data)), data)...)
	out = append(out, b[field+6+length:bounds.serializedStart]...)
	tail := len(out)
	out = append(out, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(out, uint32(tail))
}

func TestOpenMPTVolumeColumnSlidePropagation(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"up then effect memory", []byte{0x81, 15, 60, 1, 48, 4, 2, 0, 0x81, 4, 88, 0, 0x81, 8, 4, 0, 0}},
		{"down then effect memory", []byte{0x81, 15, 60, 1, 48, 4, 0x20, 0, 0x81, 4, 98, 0, 0x81, 8, 4, 0, 0}},
		{"volume memory", []byte{0x81, 7, 60, 1, 48, 0, 0x81, 4, 88, 0, 0x81, 4, 95, 0, 0x81, 8, 4, 0, 0}},
		{"empty volume memory", []byte{0x81, 15, 60, 1, 48, 4, 3, 0, 0x81, 4, 95, 0, 0x81, 8, 4, 0, 0}},
		{"same row effect zero", []byte{0x81, 15, 60, 1, 48, 4, 3, 0, 0x81, 12, 88, 4, 0, 0}},
		{"shared parameter", []byte{0x81, 7, 60, 1, 48, 0, 0x81, 4, 108, 0, 0x81, 4, 95, 0, 0x81, 8, 4, 0, 0}},
		{"delayed volume slide", []byte{0x81, 7, 60, 1, 48, 0, 0x81, 12, 88, 19, 0xd1, 0, 0x81, 8, 4, 0, 0}},
	}
	for _, tt := range tests {
		for _, enabled := range []bool{false, true} {
			for _, shared := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/enabled=%t/shared=%t", tt.name, enabled, shared), func(t *testing.T) {
					fixture := func(rows []byte) []byte {
						b := mptVolumeSlidePropagationFixture(t, rows, enabled, 0x01320400)
						if shared {
							b[bytesIndex(t, b, ".FSM")+6+int(mptITVolColMemory)/8] &^= 1 << (mptITVolColMemory % 8)
						}
						return b
					}
					ref, actual := mptSharedEffectRenderForDuration(t, fixture(tt.rows), 200)
					baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture([]byte{0x81, 7, 60, 1, 48, 0}), 200)
					for tick := range 10 {
						assertMPTMVolumeRatio(t, ref, actual, baseRef, baseActual, tick, .04)
					}
				})
			}
		}
	}
}

func TestMPTVolumeColumnSlidePropagationVersion(t *testing.T) {
	for _, tt := range []struct {
		version    uint32
		propagates bool
	}{
		{0x01310000, true},
		{0x01320000, false},
		{0x01320202, true},
		{0x01320203, false},
		{0x01320400, false},
	} {
		b := mptVolumeSlidePropagationFixture(t, nil, true, tt.version)
		loaded, err := loadMPTM(b)
		if err != nil {
			t.Fatal(err)
		}
		if got := loaded.Assets.ITCompatibility.mptVolumeSlidePropagation; got != tt.propagates {
			t.Errorf("version %#x: propagates=%t, want %t", tt.version, got, tt.propagates)
		}
	}
}

func TestMPTVolumeColumnSlidePropagationState(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		b := mptVolumeSlidePropagationFixture(t, nil, enabled, 0x01320400)
		loaded, err := loadMPTM(b)
		if err != nil {
			t.Fatal(err)
		}
		w := NewTracker(loaded.Assets, referenceRate, 256)
		w.TrackMemories[0].LastVolSlide = 2
		w.PendingCmds[0] = ModCommand{HasVolume: true, Volume: 88}
		VolumeEffectSystem(w)
		want := uint8(0x30)
		if enabled {
			want = 2
		}
		if got := w.TrackMemories[0].LastVolSlide; got != want {
			t.Errorf("enabled=%t: effect memory %#x, want %#x", enabled, got, want)
		}
		if got := w.TrackMemories[0].ITVolSlide; got != 3 {
			t.Errorf("enabled=%t: volume memory %d, want 3", enabled, got)
		}
	}
}

func TestMPTVolumeColumnSlidePropagationChunking(t *testing.T) {
	rows := []byte{0x81, 15, 60, 1, 48, 4, 2, 0, 0x81, 4, 88, 0, 0x81, 8, 4, 0, 0}
	for _, enabled := range []bool{false, true} {
		loaded, err := loadMPTM(mptVolumeSlidePropagationFixture(t, rows, enabled, 0x01320400))
		if err != nil {
			t.Fatal(err)
		}
		render := func(chunk int) []float32 {
			w := NewTracker(loaded.Assets, referenceRate, 256)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
			pcm := make([]float32, 9600*2)
			for frame := 0; frame < len(pcm)/2; frame += chunk {
				n := min(chunk, len(pcm)/2-frame)
				if got := w.Read(pcm[frame*2:(frame+n)*2], n); got != n {
					t.Fatalf("short render at frame %d: %d", frame, got)
				}
			}
			return pcm
		}
		if !slices.Equal(render(256), render(37)) {
			t.Errorf("enabled=%t: output depends on chunk size", enabled)
		}
	}
}
