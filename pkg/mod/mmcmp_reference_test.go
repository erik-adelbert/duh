package mod

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenMPTMMCMPS3M(t *testing.T) {
	packed, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mmcmp", "astraying_voyages.s3m"))
	if err != nil {
		t.Fatal(err)
	}
	unpacked, err := unpackITContainer(packed)
	if err != nil {
		t.Fatal(err)
	}
	if len(unpacked) < 0x30 || string(unpacked[0x2c:0x30]) != "SCRM" {
		t.Fatal("unpacked archive is not an S3M")
	}
	decoder, err := Decode(bytes.NewReader(packed))
	if err != nil {
		t.Fatal(err)
	}
	defer decoder.Close() //nolint:errcheck // can't do anything if it fails
	samples := make([]byte, 48000*8)
	n, err := decoder.Read(samples)
	if err != nil || n != len(samples) || bytes.Equal(samples, make([]byte, len(samples))) {
		t.Fatalf("decoder output: %d bytes, error %v", n, err)
	}
	for _, subsong := range []string{"0", "1"} {
		t.Run("subsong="+subsong, func(t *testing.T) {
			reference := renderMPTSequenceReference(t, packed, "--subsong", subsong, "--end-time", "15")
			actual := renderMPTSequenceReference(t, unpacked, "--subsong", subsong, "--end-time", "15")
			if len(reference) == 0 || len(reference) != len(actual) {
				t.Fatalf("reference length %d, unpacked length %d", len(reference), len(actual))
			}
			nonzero := false
			for i := range reference {
				nonzero = nonzero || reference[i] != 0
				if reference[i] != actual[i] {
					t.Fatalf("sample %d: packed %g, unpacked %g", i, reference[i], actual[i])
				}
			}
			if !nonzero {
				t.Fatal("reference audio is silent")
			}
		})
	}
}
