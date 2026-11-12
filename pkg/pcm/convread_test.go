// duh pcm package
//
// Based on the UCB release of Plan 9 pcmconv.
//
// Copyright (C) 2026 <Your Name>
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package pcm

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

func TestConvReader_CopyPath(t *testing.T) {
	f := mustReaderFormat(t, SLE16Stereo44k)
	conv, err := NewConverter(f, f)

	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	input := []byte{0, 1, 2, 3, 4, 5, 6, 7}

	output, err := io.ReadAll(OpenConv(bytes.NewReader(input), conv))

	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("ReadAll error: %v", err)
	}

	if !bytes.Equal(output, input) {
		t.Fatalf("output = %v, want %v", output, input)
	}
}

func TestConvReader_FlushesPartialFrameAtEOF(t *testing.T) {
	ifmt := mustReaderFormat(t, SLE16Mono44k)
	ofmt := mustReaderFormat(t, U8Mono8k)

	ofmt.sampleRate = ifmt.sampleRate

	conv, err := NewConverter(ofmt, ifmt)

	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	output, err := io.ReadAll(OpenConv(bytes.NewReader([]byte{0x34}), conv))

	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}

	if !bytes.Equal(output, []byte{0x80}) {
		// 0x34 -> 0x34 0x00 -> 0x80 (unsigned 8-bit)
		t.Fatalf("output = %v, want [0x80]", output)
	}
}

type dataErrorReader struct {
	data []byte
	err  error
}

func (r *dataErrorReader) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, r.err
	}
	return n, nil
}

func TestConvReader_DataAndEOF(t *testing.T) {
	ifmt := mustReaderFormat(t, SLE16Mono44k)
	ofmt := mustReaderFormat(t, U8Mono8k)
	ofmt.sampleRate = ifmt.sampleRate

	for _, readSize := range []int{1, 8} {
		t.Run(fmt.Sprintf("read-size-%d", readSize), func(t *testing.T) {
			conv, err := NewConverter(ofmt, ifmt)
			if err != nil {
				t.Fatal(err)
			}
			reader := OpenConv(&dataErrorReader{data: []byte{0, 0x40, 0x34}, err: io.EOF}, conv)
			var got []byte
			buf := make([]byte, readSize)
			for {
				n, err := reader.Read(buf)
				got = append(got, buf[:n]...)
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if want := []byte{0xc0, 0x80}; !bytes.Equal(got, want) {
				t.Fatalf("output = %v, want %v", got, want)
			}
		})
	}
}

func TestConvReader_DataAndNonEOFError(t *testing.T) {
	ifmt := mustReaderFormat(t, SLE16Mono44k)
	ofmt := mustReaderFormat(t, U8Mono8k)
	ofmt.sampleRate = ifmt.sampleRate
	readErr := errors.New("source failure")

	for _, readSize := range []int{1, 8} {
		t.Run(fmt.Sprintf("read-size-%d", readSize), func(t *testing.T) {
			conv, err := NewConverter(ofmt, ifmt)
			if err != nil {
				t.Fatal(err)
			}
			reader := OpenConv(&dataErrorReader{data: []byte{0, 0x40, 0, 0xc0}, err: readErr}, conv)
			var got []byte
			buf := make([]byte, readSize)
			for {
				n, err := reader.Read(buf)
				got = append(got, buf[:n]...)
				if err != nil {
					if !errors.Is(err, ErrRead) || !strings.Contains(err.Error(), readErr.Error()) {
						t.Fatalf("error = %v, want %v", err, readErr)
					}
					break
				}
			}
			if want := []byte{0xc0, 0x40}; !bytes.Equal(got, want) {
				t.Fatalf("output = %v, want %v", got, want)
			}
		})
	}
}

func TestConvReader_SeekResetsPendingOutput(t *testing.T) {
	f := mustReaderFormat(t, SLE16Stereo44k)

	conv, err := NewConverter(f, f)

	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	input := []byte{0, 1, 2, 3, 4, 5, 6, 7}

	reader := OpenConv(bytes.NewReader(input), conv)

	buffer := make([]byte, 2)

	if _, err := reader.Read(buffer); err != nil {
		t.Fatalf("Read error: %v", err)
	}

	offset, err := reader.Seek(4, io.SeekStart)

	if err != nil {
		t.Fatalf("Seek error: %v", err)
	}

	if offset != 4 {
		t.Fatalf("Seek returned %d, want 4", offset)
	}

	output, err := io.ReadAll(reader)

	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}

	if !bytes.Equal(output, input[4:]) {
		t.Fatalf("output = %v, want %v", output, input[4:])
	}
}

func TestReader_ReadAtDoesNotAffectSequentialRead(t *testing.T) {
	f := mustReaderFormat(t, SLE16Stereo44k)

	conv, err := NewConverter(f, f)

	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	input := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	reader := OpenConv(bytes.NewReader(input), conv)
	output := make([]byte, 4)

	n, err := reader.ReadAt(output, 4)
	if err != nil {
		t.Fatalf("ReadAt error: %v", err)
	}
	if n != len(output) || !bytes.Equal(output, input[4:]) {
		t.Fatalf("ReadAt = %v, want %v", output, input[4:])
	}

	n, err = io.ReadFull(reader, output)
	if err != nil {
		t.Fatalf("sequential Read error: %v", err)
	}
	if n != len(output) || !bytes.Equal(output, input[:4]) {
		t.Fatalf("sequential output = %v, want %v", output, input[:4])
	}
}

func TestConvReader_ReadAtConvertedStream(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{"same-rate", "s16c1r44100", "u8c1r44100"},
		{"resampling", "s16c1r44100", "s16c1r48000"},
		{"dithering", "s16c1r44100", "s8c1r44100"},
	}
	input := make([]byte, 40001)
	for i := range input {
		input[i] = byte(i*37 + 5)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ifmt := mustReaderFormat(t, tt.in)
			ofmt := mustReaderFormat(t, tt.out)
			newReader := func() *ConvReader {
				t.Helper()
				conv, err := NewConverter(ofmt, ifmt)
				if err != nil {
					t.Fatal(err)
				}
				return OpenConv(bytes.NewReader(input), conv)
			}
			want, err := io.ReadAll(newReader())
			if err != nil {
				t.Fatal(err)
			}
			reader := newReader()
			prefix := make([]byte, 7)
			if _, err := io.ReadFull(reader, prefix); err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				offset int
				size   int
			}{
				{0, 9}, {1, 13}, {257, 29}, {18001, 31}, {len(want) - 3, 8}, {len(want), 1}, {len(want) + 10, 1},
			} {
				buf := make([]byte, tc.size)
				n, err := reader.ReadAt(buf, int64(tc.offset))
				start := min(tc.offset, len(want))
				end := min(start+tc.size, len(want))
				expected := want[start:end]
				if n != len(expected) || !bytes.Equal(buf[:n], expected) {
					t.Errorf("ReadAt(%d, %d) = %x, want %x", tc.offset, tc.size, buf[:n], expected)
				}
				if len(expected) < tc.size {
					if err != io.EOF {
						t.Errorf("ReadAt(%d, %d) error = %v, want EOF", tc.offset, tc.size, err)
					}
				} else if err != nil {
					t.Errorf("ReadAt(%d, %d) error = %v", tc.offset, tc.size, err)
				}
			}
			rest, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if got := append(prefix, rest...); !bytes.Equal(got, want) {
				t.Fatal("ReadAt changed sequential converted output")
			}
			if n, err := reader.ReadAt(make([]byte, 1), -1); n != 0 || !errors.Is(err, ErrRead) {
				t.Errorf("negative offset = (%d, %v), want ErrRead", n, err)
			}
			if n, err := reader.ReadAt(nil, 0); n != 0 || err != nil {
				t.Errorf("empty ReadAt = (%d, %v), want (0, nil)", n, err)
			}
		})
	}
}

func TestConvReader_ConcurrentReadAt(t *testing.T) {
	ifmt := mustReaderFormat(t, SLE16Mono44k)
	ofmt := mustReaderFormat(t, "s16c1r48000")
	input := make([]byte, 8000)
	for i := range input {
		input[i] = byte(i * 19)
	}
	conv, err := NewConverter(ofmt, ifmt)
	if err != nil {
		t.Fatal(err)
	}
	reader := OpenConv(bytes.NewReader(input), conv)
	want, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			off := i * 127
			buf := make([]byte, 128)
			n, err := reader.ReadAt(buf, int64(off))
			if err != nil || n != len(buf) || !bytes.Equal(buf, want[off:off+len(buf)]) {
				t.Errorf("ReadAt(%d) = (%d, %v), output mismatch: %v", off, n, err, !bytes.Equal(buf, want[off:off+len(buf)]))
			}
		}()
	}
	wg.Wait()
}

func TestConvReader_ReadAtWithoutReaderAt(t *testing.T) {
	format := mustReaderFormat(t, SLE16Mono44k)
	conv, err := NewConverter(format, format)
	if err != nil {
		t.Fatal(err)
	}
	source := struct{ io.Reader }{bytes.NewReader([]byte{0, 0})}
	reader := OpenConv(source, conv)
	if n, err := reader.ReadAt(make([]byte, 1), 0); n != 0 || !errors.Is(err, ErrESPIPE) {
		t.Fatalf("ReadAt = (%d, %v), want ErrESPIPE", n, err)
	}
}

func mustReaderFormat(t *testing.T, text string) Format {
	t.Helper()

	format, err := ParseFormat(text)

	if err != nil {
		t.Fatalf("FormatFromString(%q) error: %v", text, err)
	}

	return format
}
