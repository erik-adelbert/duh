package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestModdecHelper(t *testing.T) {
	if os.Getenv("MODDEC_TEST_HELPER") != "1" {
		return
	}
	os.Args = []string{"moddec", "-noloop"}
	if path := os.Getenv("MODDEC_TEST_PATH"); path != "" {
		os.Args = append(os.Args, path)
	}
	main()
	os.Exit(0)
}

func TestNoLoopFlag(t *testing.T) {
	const instrument, pattern, sample = 0x100, 0x150, 0x200
	data := make([]byte, sample+512)
	data[0x1c], data[0x1d] = 0x1a, 0x10
	binary.LittleEndian.PutUint16(data[0x20:], 2)
	binary.LittleEndian.PutUint16(data[0x22:], 1)
	binary.LittleEndian.PutUint16(data[0x24:], 1)
	binary.LittleEndian.PutUint16(data[0x28:], 0x1320)
	binary.LittleEndian.PutUint16(data[0x2a:], 2)
	copy(data[0x2c:], "SCRM")
	data[0x30], data[0x31], data[0x32], data[0x33] = 64, 1, 125, 0xb0
	for i := range 32 {
		data[0x40+i] = 0xff
	}
	data[0x40], data[0x60], data[0x61] = 0, 0, 0xff
	binary.LittleEndian.PutUint16(data[0x62:], instrument/16)
	binary.LittleEndian.PutUint16(data[0x64:], pattern/16)
	data[instrument] = 1
	binary.LittleEndian.PutUint16(data[instrument+0x0e:], sample/16)
	binary.LittleEndian.PutUint32(data[instrument+0x10:], 512)
	data[instrument+0x1c] = 64
	binary.LittleEndian.PutUint32(data[instrument+0x20:], 8363)
	copy(data[instrument+0x4c:], "SCRS")
	binary.LittleEndian.PutUint16(data[pattern:], 69)
	copy(data[pattern+2:], []byte{0xa0, 0x40, 1, 2, 0})
	for _, mode := range []string{"stdin", "file"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestModdecHelper$")
			cmd.Env = append(os.Environ(), "MODDEC_TEST_HELPER=1")
			if mode == "stdin" {
				cmd.Stdin = bytes.NewReader(data)
			} else {
				path := filepath.Join(t.TempDir(), "song.s3m")
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				cmd.Env = append(cmd.Env, "MODDEC_TEST_PATH="+path)
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			pcm, err := cmd.Output()
			if err != nil {
				t.Fatalf("moddec: %v: %s", err, stderr.String())
			}
			if len(pcm) != 960*8 {
				t.Fatalf("got %d bytes, want %d", len(pcm), 960*8)
			}
		})
	}
}
