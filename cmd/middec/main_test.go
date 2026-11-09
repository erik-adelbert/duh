package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	mid "github.com/erik-adelbert/duh/pkg/mid"
)

func TestBuiltinBankFourOpCounts(t *testing.T) {
	for i := range bnknames {
		quad, total, err := mid.Melodic4OpCount(i)
		if err != nil {
			t.Errorf("bank %d: %v", i, err)
		} else if quad > total || total > 128 {
			t.Errorf("bank %d: invalid four-op count %d/%d", i, quad, total)
		}
	}
}

func TestCommandUsesDecoderPCM(t *testing.T) {
	track := []byte{0, 0x90, 60, 100, 96, 0x80, 60, 0, 0, 0xff, 0x2f, 0}
	data := []byte{'M', 'T', 'h', 'd', 0, 0, 0, 6, 0, 0, 0, 1, 0, 96, 'M', 'T', 'r', 'k', 0, 0, 0, 0}
	binary.BigEndian.PutUint32(data[18:], uint32(len(track)))
	data = append(data, track...)
	path := filepath.Join(t.TempDir(), "song.mid")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	nquad, total, err := mid.Melodic4OpCount(0)
	if err != nil {
		t.Fatal(err)
	}
	decoder, err := mid.Decode(bytes.NewReader(data), mid.DecoderOptions{
		CardCount: 1, FourOps: defaultFourOps(nquad, total, 1),
	})
	if err != nil {
		t.Fatal(err)
	}
	want, err := io.ReadAll(decoder)
	if err != nil {
		t.Fatal(err)
	}
	run := func(input io.Reader, args ...string) []byte {
		t.Helper()
		cmdArgs := append([]string{"run", "."}, args...)
		cmd := exec.Command("go", cmdArgs...)
		cmd.Stdin = input
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		got, err := cmd.Output()
		if err != nil {
			t.Fatalf("middec %v: %v: %s", args, err, stderr.String())
		}
		return got
	}

	for _, args := range [][]string{
		{path, "0", "1", "0"},
		{"-f", path, "0", "1", "0"},
	} {
		got := run(nil, args...)
		if !bytes.Equal(got, want) {
			t.Errorf("CLI output for %v differs from decoder: got %d bytes, want %d", args, len(got), len(want))
		}
	}

	fileOutput := run(nil, "-f", path)
	stdinOutput := run(bytes.NewReader(data))
	if !bytes.Equal(stdinOutput, fileOutput) {
		t.Errorf("stdin output differs from -f output: got %d bytes, want %d", len(stdinOutput), len(fileOutput))
	}
}
