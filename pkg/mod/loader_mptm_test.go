package mod

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadITRejectsMPTM(t *testing.T) {
	modern := testIT()
	binary.LittleEndian.PutUint16(modern[40:], 0x891)
	start := len(modern)
	modern = append(modern, []byte("228mptm")...)
	modern = binary.LittleEndian.AppendUint32(modern, uint32(start))

	legacy := testIT()
	copy(legacy, "tpm.")

	for _, tt := range []struct {
		name string
		data []byte
	}{
		{"modern", modern},
		{"legacy", legacy},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadIT(tt.data)
			if err == nil || !strings.Contains(err.Error(), "MPTM modules are not yet supported") {
				t.Fatalf("got %v, want explicit MPTM error", err)
			}
			for _, wrapped := range []bool{false, true} {
				data := tt.data
				if wrapped {
					data = uncompressedMMCMP(data)
				}
				path := filepath.Join(t.TempDir(), "module.it")
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := LoadIT(path); err == nil || !strings.Contains(err.Error(), "MPTM modules are not yet supported") {
					t.Errorf("wrapped=%v: got %v, want explicit MPTM error", wrapped, err)
				}
			}
		})
	}
}

func TestMPTMDetectionDoesNotConfuseIT(t *testing.T) {
	for _, tt := range []struct {
		name string
		edit func([]byte)
	}{
		{"ordinary IT", func([]byte) {}},
		{"MPTM version without extensions", func(b []byte) {
			binary.LittleEndian.PutUint16(b[40:], 0x891)
		}},
		{"out of bounds extension offset", func(b []byte) {
			binary.LittleEndian.PutUint16(b[40:], 0x891)
			binary.LittleEndian.PutUint32(b[len(b)-4:], uint32(len(b)-2))
		}},
		{"extension signature without version", func(b []byte) {
			copy(b[256:], "228")
			binary.LittleEndian.PutUint32(b[len(b)-4:], 256)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := testIT()
			tt.edit(b)
			if isMPTM(b) {
				t.Fatal("ordinary or malformed IT data detected as MPTM")
			}
		})
	}
}
