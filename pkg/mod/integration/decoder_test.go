package integration

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/erik-adelbert/duh/pkg/mod"
	"github.com/erik-adelbert/duh/pkg/mod/internal/testfixtures"
)

type decoderFailReader struct{}

func (decoderFailReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

type decoderReadCloser struct {
	*bytes.Reader
	closed bool
}

func (r *decoderReadCloser) Close() error {
	r.closed = true
	return nil
}

func TestDecoderDoesNotCloseInput(t *testing.T) {
	input := &decoderReadCloser{Reader: bytes.NewReader(testfixtures.IT())}
	d, err := mod.Decode(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if input.closed {
		t.Fatal("decoder closed input")
	}
}

func TestDecoderErrors(t *testing.T) {
	for _, input := range []io.Reader{nil, decoderFailReader{}, strings.NewReader("unknown"), strings.NewReader("ziRCONia")} {
		if _, err := mod.Decode(input); err == nil {
			t.Errorf("accepted invalid input %T", input)
		}
	}
	s3m := testfixtures.S3M(0, 0, nil)
	for _, order := range []byte{0xff, 3} {
		s3m[0x60] = order
		if _, err := mod.Decode(bytes.NewReader(s3m)); err == nil {
			t.Errorf("accepted invalid order %d", order)
		}
	}
	for _, sixteenBit := range []bool{false, true} {
		s3m := testfixtures.S3M(0, 0, []byte{1, 2})
		if sixteenBit {
			s3m[0x11f] = 4
		}
		binary.LittleEndian.PutUint32(s3m[0x110:], ^uint32(0))
		if _, err := mod.Decode(bytes.NewReader(s3m)); err == nil || !strings.Contains(err.Error(), "truncated S3M sample") {
			t.Errorf("16-bit %v: got %v", sixteenBit, err)
		}
	}
	if _, err := mod.Decode(decoderFailReader{}); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("input error not preserved: %v", err)
	}
}

func TestDecoderNoLoop(t *testing.T) {
	it := testfixtures.IT()
	it[50] = 1
	it[256+18] |= 0x10
	binary.LittleEndian.PutUint32(it[256+52:], 0)
	binary.LittleEndian.PutUint32(it[256+56:], 128)
	itRows := []byte{0x81, 11, 60, 1, 2, 0, 0}
	binary.LittleEndian.PutUint16(it[336:], uint16(len(itRows)))
	binary.LittleEndian.PutUint16(it[338:], 1)
	copy(it[344:], itRows)

	s3m := testfixtures.S3M(2, 0, bytes.Repeat([]byte{0, 64, 128, 192}, 128))
	s3m[0x31] = 1
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{"IT", it},
		{"S3M", s3m},
	} {
		t.Run(tt.name, func(t *testing.T) {
			once, err := mod.Decode(bytes.NewReader(tt.data), mod.DecoderOptions{NoLoop: true})
			if err != nil {
				t.Fatal(err)
			}
			defer once.Close() //nolint:errcheck // can't do anything if it fails
			output, err := io.ReadAll(once)
			if err != nil {
				t.Fatal(err)
			}
			const cycleBytes = 960 * 8
			if len(output) != cycleBytes {
				t.Fatalf("got %d bytes, want %d", len(output), cycleBytes)
			}
			if tt.name == "IT" && bytes.Equal(output[len(output)-1024:], make([]byte, 1024)) {
				t.Fatal("sample loop stopped during the first song cycle")
			}
			if n, err := once.Read(make([]byte, 1)); n != 0 || err != io.EOF {
				t.Fatalf("read after end: %d, %v", n, err)
			}
			repeating, err := mod.Decode(bytes.NewReader(tt.data))
			if err != nil {
				t.Fatal(err)
			}
			defer repeating.Close() //nolint:errcheck // can't do anything if it fails
			twoCycles := make([]byte, 2*cycleBytes)
			if _, err := io.ReadFull(repeating, twoCycles); err != nil {
				t.Fatalf("default playback did not continue: %v", err)
			}
			if !bytes.Equal(output, twoCycles[:cycleBytes]) {
				t.Fatal("no-loop changed the first cycle")
			}
		})
	}
}

func TestDecoderNoLoopPreservesFinitePatternLoops(t *testing.T) {
	it := testfixtures.IT()
	it[50] = 1
	rows := []byte{
		0x81, 11, 60, 1, 19, 0xb0, 0,
		0x81, 8, 19, 0xb2, 0,
		0x81, 8, 2, 0, 0,
	}
	binary.LittleEndian.PutUint16(it[336:], uint16(len(rows)))
	binary.LittleEndian.PutUint16(it[338:], 3)
	copy(it[344:], rows)
	d, err := mod.Decode(bytes.NewReader(it), mod.DecoderOptions{NoLoop: true})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close() //nolint:errcheck // can't do anything if it fails
	pcm, err := io.ReadAll(d)
	if err != nil {
		t.Fatal(err)
	}
	if want := 7 * 960 * 8; len(pcm) != want {
		t.Fatalf("got %d bytes, want %d", len(pcm), want)
	}
}

func TestDecoderNoLoopPreservesNaturalEnd(t *testing.T) {
	b := testfixtures.IT()
	binary.LittleEndian.PutUint16(b[338:], 4)
	render := func(opts mod.DecoderOptions) []byte {
		t.Helper()
		d, err := mod.Decode(bytes.NewReader(b), opts)
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
	if plain, noLoop := render(mod.DecoderOptions{}), render(mod.DecoderOptions{NoLoop: true}); !bytes.Equal(plain, noLoop) {
		t.Fatal("no-loop changed naturally ending playback")
	}
}
