package mp3

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

// A single Write larger than the encoder's internal buffer must not lose audio.
func TestEncoderLargeWriteIsNotTruncated(t *testing.T) {
	for _, nchan := range []int{1, 2} {
		format, err := pcm.NewFormat('s', 16, nchan, 44100)
		if err != nil {
			t.Fatal(err)
		}

		input := make([]byte, 2*44100*int(format.FrameSize())) // 2s, far beyond 200ms
		for i := 0; i < len(input)/2; i++ {
			value := int16((i*997)%65536 - 32768)
			binary.LittleEndian.PutUint16(input[i*2:], uint16(value))
		}

		encode := func(size int) []byte {
			t.Helper()

			var output bytes.Buffer
			encoder, err := Encode(&output, format)
			if err != nil {
				t.Fatal(err)
			}

			for rest := input; len(rest) > 0; {
				chunk := rest[:min(size, len(rest))]
				if n, err := encoder.Write(chunk); err != nil || n != len(chunk) {
					t.Fatalf("Write = %d, %v; want %d, nil", n, err, len(chunk))
				}
				rest = rest[len(chunk):]
			}

			if err := encoder.Close(); err != nil {
				t.Fatal(err)
			}

			return output.Bytes()
		}

		whole, chunked := encode(len(input)), encode(32*1024)
		if len(chunked) == 0 {
			t.Fatalf("%d channels: encoder produced no output", nchan)
		}
		if !bytes.Equal(whole, chunked) {
			t.Errorf("%d channels: one big Write gave %d bytes, 32KiB writes gave %d", nchan, len(whole), len(chunked))
		}
	}
}
