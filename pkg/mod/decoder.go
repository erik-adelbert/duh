package mod

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

const (
	decoderSampleRate  = 48000
	decoderBlockFrames = 1024
)

// Decoder renders a supported S3M, IT, or MPTM module as f32le stereo PCM.
// Playback follows the tracker's existing end-of-song and loop behavior.
// Decoder is not safe for concurrent use.
type Decoder struct {
	tracker  *Tracker
	warnings []string
	samples  []float32
	pcm      []byte
	pending  []byte
}

// DecoderOptions configures sequence selection and playback termination.
type DecoderOptions struct {
	// Sequence is the zero-based MPTM sequence. Other formats only support zero.
	Sequence int
	// NoLoop stops when a song playback state repeats. Finite pattern and sample loops are preserved.
	// MPTM already uses this behavior by default.
	NoLoop bool
}

// Decode reads the module from in and prepares a decoder. The input reader is
// not closed. By default, MPTM sequence zero is selected. Feature support
// depends on the module format and loader.
func Decode(in io.Reader, options ...DecoderOptions) (*Decoder, error) {
	if in == nil {
		return nil, errors.New("module decode: nil input")
	}

	if len(options) > 1 {
		return nil, errors.New("module decode: multiple decoder options")
	}

	var opts DecoderOptions
	if len(options) == 1 {
		opts = options[0]
	}

	sequence := opts.Sequence
	if sequence < 0 {
		return nil, fmt.Errorf("module decode: invalid sequence %d", sequence)
	}

	data, err := io.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("module decode: %w", err)
	}

	data, err = unpackITContainer(data)
	if err != nil {
		return nil, fmt.Errorf("module decode: %w", err)
	}

	var loaded *S3MLoadResult
	switch {
	case isMPTM(data):
		loaded, err = loadMPTMSequence(data, sequence)
	case len(data) >= 4 && string(data[:4]) == "IMPM":
		if sequence != 0 {
			return nil, errors.New("module decode: IT supports only sequence 0")
		}
		loaded, err = loadIT(data)
	case len(data) >= 0x30 && string(data[0x2c:0x30]) == "SCRM":
		if sequence != 0 {
			return nil, errors.New("module decode: S3M supports only sequence 0")
		}
		loaded, err = loadS3M(data)
	default:
		return nil, errors.New("module decode: unsupported or invalid module format")
	}
	if err != nil {
		return nil, fmt.Errorf("module decode: %w", err)
	}

	tracker := NewTracker(loaded.Assets, decoderSampleRate, decoderBlockFrames)
	if opts.NoLoop && tracker.mptVisitedRows == nil {
		tracker.mptVisitedRows = make(map[mptVisitedRow]struct{})
	}
	tracker.Clock.Speed, tracker.Clock.Tempo, tracker.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1

	return &Decoder{
		tracker: tracker, warnings: loaded.Warnings,
		samples: make([]float32, decoderBlockFrames*2),
		pcm:     make([]byte, decoderBlockFrames*2*4),
	}, nil
}

// Read accepts arbitrary byte lengths and returns interleaved stereo PCM.
func (d *Decoder) Read(p []byte) (int, error) {
	if d.tracker == nil {
		return 0, os.ErrClosed
	}

	if len(p) == 0 {
		return 0, nil
	}

	n := 0
	for n < len(p) {
		if len(d.pending) == 0 {
			frames := d.tracker.Read(d.samples, decoderBlockFrames)
			if frames == 0 {
				if n == 0 {
					return 0, io.EOF
				}

				return n, nil
			}

			d.pending = d.pcm[:frames*8]
			for i, sample := range d.samples[:frames*2] {
				binary.LittleEndian.PutUint32(d.pending[i*4:], math.Float32bits(sample))
			}
		}

		copied := copy(p[n:], d.pending)
		d.pending = d.pending[copied:]

		n += copied
	}

	return n, nil
}

// Format reports the decoder's output PCM format.
func (d *Decoder) Format() pcm.Format {
	format, _ := pcm.NewFormat(pcm.Float, 32, 2, decoderSampleRate)
	return format
}

// Warnings returns playback limitations reported by the module loader.
func (d *Decoder) Warnings() []string {
	return append([]string(nil), d.warnings...)
}

// Close releases the decoder without closing the input reader.
func (d *Decoder) Close() error {
	d.tracker, d.samples, d.pcm, d.pending = nil, nil, nil, nil
	return nil
}
