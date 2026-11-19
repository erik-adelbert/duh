package mod

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"strconv"
	"testing"
	"time"
)

func durationITFixture(rows []byte, count uint16) []byte {
	b := testIT()
	b[50] = 3
	binary.LittleEndian.PutUint16(b[336:], uint16(len(rows)))
	binary.LittleEndian.PutUint16(b[338:], count)
	copy(b[344:], rows)
	return b
}

func TestDurationMatchesPlayback(t *testing.T) {
	it := durationITFixture([]byte{
		0x81, 11, 60, 1, 19, 0xb0, 0,
		0x81, 8, 20, 0x11, 0,
		0x81, 8, 19, 0xb2, 0,
		0x81, 8, 19, 0xe1, 0,
		0,
		0x81, 8, 2, 0, 0,
	}, 6)
	mptm := mptmPlaybackFixture()
	binary.LittleEndian.PutUint16(mptm[338:], 4)
	s3m := referenceS3M(2, 0, bytes.Repeat([]byte{0, 64, 128, 192}, 128))
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{"it loop, tempo slide and delay", it},
		{"s3m loop", s3m},
		{"mptm modern tempo", mptm},
		{"mmcmp it", uncompressedMMCMP(it)},
		{"it special memory", durationITFixture([]byte{
			0x81, 8, 19, 0xb0, 0,
			0x81, 8, 19, 0, 0,
			0x81, 8, 19, 0xb2, 0,
			0x81, 8, 2, 0, 0,
		}, 4)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d, err := Decode(bytes.NewReader(tt.data), DecoderOptions{NoLoop: true})
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close() //nolint:errcheck
			est, err := d.NewDurationEstimator()
			if err != nil {
				t.Fatal(err)
			}
			var duration time.Duration
			var done bool
			for !done {
				duration, done, err = est.Advance(3)
				if err != nil {
					t.Fatal(err)
				}
			}
			pcm, err := io.ReadAll(d)
			if err != nil {
				t.Fatal(err)
			}
			frames := int64(len(pcm) / 8)
			want := time.Duration(frames/decoderSampleRate)*time.Second +
				time.Duration(frames%decoderSampleRate)*time.Second/decoderSampleRate
			if duration != want {
				t.Fatalf("duration %s, rendered %s (%d frames)", duration, want, frames)
			}
			again, done, err := est.Advance(1)
			if err != nil || !done || again != want {
				t.Fatalf("completed estimate = %s, %v, %v", again, done, err)
			}
		})
	}
}

func TestDurationSelectedSequence(t *testing.T) {
	b := multiSequencePlaybackFixture(t)
	for _, seq := range []int{0, 1} {
		d, err := Decode(bytes.NewReader(b), DecoderOptions{Sequence: seq})
		if err != nil {
			t.Fatal(err)
		}
		est, err := d.NewDurationEstimator()
		if err != nil {
			t.Fatal(err)
		}
		var got time.Duration
		var done bool
		for !done {
			got, done, err = est.Advance(100)
			if err != nil {
				t.Fatal(err)
			}
		}
		pcm, err := io.ReadAll(d)
		if err != nil {
			t.Fatal(err)
		}
		frames := int64(len(pcm) / 8)
		want := time.Duration(frames/decoderSampleRate)*time.Second +
			time.Duration(frames%decoderSampleRate)*time.Second/decoderSampleRate
		if got != want {
			t.Fatalf("sequence %d: duration %s, rendered %s", seq, got, want)
		}
		ref := renderMPTSequenceReference(t, b, "--subsong", strconv.Itoa(seq))
		refFrames := (len(ref) - 9600) / 2
		if math.Abs(float64(frames-int64(refFrames))) > 1 {
			t.Fatalf("sequence %d: tracker %d frames, OpenMPT %d", seq, frames, refFrames)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDurationEstimatorErrors(t *testing.T) {
	var uninitialized DurationEstimator
	if _, _, err := uninitialized.Advance(1); err == nil {
		t.Fatal("expected an uninitialized estimator error")
	}
	d, err := Decode(bytes.NewReader(testIT()))
	if err != nil {
		t.Fatal(err)
	}
	est, err := d.NewDurationEstimator()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := est.Advance(0); err == nil {
		t.Fatal("expected invalid budget error")
	}
	est.ticks = durationMaxTicks
	if _, _, err := est.Advance(1); err == nil {
		t.Fatal("expected complexity error")
	}
	if _, _, err := est.Advance(1); err == nil {
		t.Fatal("complexity error was not retained")
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.NewDurationEstimator(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed decoder: %v", err)
	}
}
