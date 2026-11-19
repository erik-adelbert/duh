package mod

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"
)

func multiSequencePlaybackFixture(t *testing.T) []byte {
	t.Helper()
	b := mptRestartPlaybackFixture(t, 0, 0, nil)
	tail := int(binary.LittleEndian.Uint32(b[len(b)-4:]))
	first := mptTestBlock("mptSeq", []mptTestEntry{
		{"l", []byte{1, 0}}, {"a", []byte{0, 0}},
		{"t", binary.LittleEndian.AppendUint32(nil, 1000000)},
		{"s", binary.LittleEndian.AppendUint32(nil, 3)},
	})
	second := mptTestBlock("mptSeq", []mptTestEntry{
		{"l", []byte{1, 0}}, {"a", []byte{1, 0}},
		{"t", binary.LittleEndian.AppendUint32(nil, 1500000)},
		{"s", binary.LittleEndian.AppendUint32(nil, 6)},
	})
	set := mptTestBlock("mptSeqC", []mptTestEntry{
		{"n", []byte{2}}, {"c", []byte{1}}, {"\x00", first}, {"\x01", second},
	})
	out := append([]byte(nil), b[:tail]...)
	out = append(out, mptTestBlock("mptm", []mptTestEntry{{"mptSeqC", set}})...)
	return binary.LittleEndian.AppendUint32(out, uint32(tail))
}

func TestMPTSequencePlayback(t *testing.T) {
	b := multiSequencePlaybackFixture(t)
	for _, tt := range []struct {
		sequence int
		tempo    float64
		speed    int
	}{
		{0, 100, 3}, {1, 150, 6},
	} {
		loaded, err := loadMPTMSequence(b, tt.sequence)
		if err != nil {
			t.Fatal(err)
		}
		if loaded.InitialTempo != tt.tempo || loaded.InitialSpeed != tt.speed ||
			len(loaded.Assets.OrderList) != 1 || loaded.Assets.OrderList[0] != tt.sequence {
			t.Fatalf("sequence %d: unexpected defaults or orders", tt.sequence)
		}
		d, err := Decode(bytes.NewReader(b), DecoderOptions{Sequence: tt.sequence})
		if err != nil {
			t.Fatal(err)
		}
		pcm, err := io.ReadAll(d)
		if err != nil {
			t.Fatal(err)
		}
		d.Close() //nolint:errcheck // can't do anything if it fails
		if len(pcm) == 0 {
			t.Fatalf("sequence %d: no audio", tt.sequence)
		}
		ref := renderMPTSequenceReference(t, b, "--subsong", strconv.Itoa(tt.sequence))
		// openmpt123 appends 100 ms of post-song output.
		if math.Abs(float64(len(ref)-9600-len(pcm)/4)) > 2 {
			t.Fatalf("sequence %d: reference=%d samples decoder=%d", tt.sequence, len(ref), len(pcm)/4)
		}
		actual := make([]float32, len(pcm)/4)
		for i := range actual {
			actual[i] = math.Float32frombits(binary.LittleEndian.Uint32(pcm[i*4:]))
		}
		want, got := referenceTickFrequency(t, ref, 1), referenceTickFrequency(t, actual, 1)
		if math.Abs(got/want-1) > .02 {
			t.Fatalf("sequence %d: OpenMPT frequency %g, decoder %g", tt.sequence, want, got)
		}
		if tt.sequence == 0 {
			d, err := Decode(bytes.NewReader(b))
			if err != nil {
				t.Fatal(err)
			}
			defaultPCM, err := io.ReadAll(d)
			d.Close() //nolint:errcheck // can't do anything if it fails
			if err != nil || !bytes.Equal(defaultPCM, pcm) {
				t.Fatalf("default sequence differs from sequence 0: %v", err)
			}
		}
	}
}

func TestDecoderSequenceErrors(t *testing.T) {
	b := multiSequencePlaybackFixture(t)
	for _, tt := range []struct {
		data []byte
		opts []DecoderOptions
		want string
	}{
		{b, []DecoderOptions{{Sequence: -1}}, "invalid sequence"},
		{b, []DecoderOptions{{Sequence: 2}}, "out of range"},
		{b, []DecoderOptions{{}, {}}, "multiple decoder options"},
		{testIT(), []DecoderOptions{{Sequence: 1}}, "IT supports only"},
		{referenceS3M(0, 0, nil), []DecoderOptions{{Sequence: 1}}, "S3M supports only"},
	} {
		if _, err := Decode(bytes.NewReader(tt.data), tt.opts...); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("expected %q, got %v", tt.want, err)
		}
	}
}
