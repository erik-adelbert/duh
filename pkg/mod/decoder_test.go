package mod

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"strings"
	"testing"
)

func TestDecoderPlayback(t *testing.T) {
	it := testIT()
	binary.LittleEndian.PutUint16(it[338:], 4)
	mptm := mptmPlaybackFixture()
	binary.LittleEndian.PutUint16(mptm[338:], 4)
	s3m := referenceS3M(0, 0, bytes.Repeat([]byte{0, 64, 128, 192}, 128))
	for _, tt := range []struct {
		name string
		data []byte
		load func([]byte) (*S3MLoadResult, error)
	}{
		{"s3m", s3m, loadS3M},
		{"it", it, loadIT},
		{"mmcmp it", uncompressedMMCMP(it), func(b []byte) (*S3MLoadResult, error) {
			data, err := unpackITContainer(b)
			if err != nil {
				return nil, err
			}
			return loadIT(data)
		}},
		{"mptm", mptm, loadMPTM},
	} {
		t.Run(tt.name, func(t *testing.T) {
			loaded, err := tt.load(tt.data)
			if err != nil {
				t.Fatal(err)
			}
			d, err := Decode(bytes.NewReader(tt.data))
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close() //nolint:errcheck // can't do anything if it fails
			if got := d.Format().String(); got != "f32c2r48000" {
				t.Fatalf("format %s", got)
			}
			if n, err := d.Read(nil); n != 0 || err != nil {
				t.Fatalf("empty read = %d, %v", n, err)
			}
			tracker := NewTracker(loaded.Assets, decoderSampleRate, decoderBlockFrames)
			tracker.Clock.Speed, tracker.Clock.Tempo, tracker.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
			samples := make([]float32, decoderBlockFrames*2)
			var want []byte
			for {
				n := tracker.Read(samples, decoderBlockFrames)
				if n == 0 {
					break
				}
				for _, sample := range samples[:n*2] {
					want = binary.LittleEndian.AppendUint32(want, math.Float32bits(sample))
				}
			}
			var got []byte
			buf := make([]byte, 137)
			for {
				n, err := d.Read(buf)
				got = append(got, buf[:n]...)
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if len(want) == 0 || !bytes.Equal(got, want) {
				t.Fatalf("output mismatch: got %d bytes, want %d", len(got), len(want))
			}
			if n, err := d.Read(buf); n != 0 || err != io.EOF {
				t.Fatalf("read after EOF = %d, %v", n, err)
			}
			if n, err := d.Read(nil); n != 0 || err != nil {
				t.Fatalf("empty read = %d, %v", n, err)
			}
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := d.Read(buf); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("read after close: %v", err)
			}
		})
	}
}

func TestDecoderWarnings(t *testing.T) {
	b := mptSongResamplingFixture(t, 1, 5, true)
	b[bytesIndex(t, b, "...C")+6] = 2
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseMPTSerializedBlock(b[bounds.serializedStart:bounds.serializedEnd], "mptm")
	if err != nil {
		t.Fatal(err)
	}
	patternData := append([]byte{0x82, 0x23, 0xfb, 1, 80}, make([]byte, 64)...)
	pattern := mptTestBlock("mptP", []mptTestEntry{{"data", patternData}})
	patterns := mptTestBlock("mptPc", []mptTestEntry{{"num", []byte{1, 0}}, {"\x00\x00", pattern}})
	tail := mptTestBlock("mptm", []mptTestEntry{{"mptSeqC", root["mptSeqC"]}, {"mptPc", patterns}})
	b = append(append([]byte(nil), b[:bounds.serializedStart]...), tail...)
	b = binary.LittleEndian.AppendUint32(b, uint32(bounds.serializedStart))
	d, err := Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close() //nolint:errcheck // can't do anything if it fails
	warnings := d.Warnings()
	if !strings.Contains(strings.Join(warnings, "\n"), "parameter automation is bypassed") {
		t.Fatalf("warnings: %v", warnings)
	}
	warnings[0] = "changed"
	if d.Warnings()[0] == "changed" {
		t.Fatal("warnings expose decoder state")
	}
}

func TestDecoderNoLoopMPTMDefault(t *testing.T) {
	b := mptS70Fixture(t, []byte{0x81, 11, 60, 2, 2, 0, 0})
	b[50] = 1
	render := func(opts DecoderOptions) []byte {
		t.Helper()
		d, err := Decode(bytes.NewReader(b), opts)
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close() //nolint:errcheck // can't do anything if it fails
		pcm, err := io.ReadAll(d)
		if err != nil {
			t.Fatal(err)
		}
		return pcm
	}
	plain, noLoop := render(DecoderOptions{}), render(DecoderOptions{NoLoop: true})
	if len(plain) != 960*8 || !bytes.Equal(plain, noLoop) {
		t.Fatalf("MPTM outputs differ: %d / %d bytes", len(plain), len(noLoop))
	}
}
