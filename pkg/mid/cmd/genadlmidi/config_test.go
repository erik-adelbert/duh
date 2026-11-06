package main

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	want := Config{
		General: General{BankCount: 12, AutoID: true},
		Banks: map[string]BankInfo{
			"bank-custom-gm-bank": {
				Format:  "WOPLX",
				FileM:   "gm.wop",
				FileD:   "gm-p.wop",
				PrefixM: "GM",
				PrefixD: "GMP",
				FilterM: "melodic",
				FilterD: "percussion",
				// NoRhythmMode: true,
				IsMT32: true,
			},
			"bank-user-42": {Format: "WOPLX", FileM: "user.wop"},
		},
	}
	content, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := cfg.Open(path); err == nil {
		t.Fatalf("Open() should fail, got nil error")
	}

	if cfg.General != want.General {
		t.Errorf("General = %+v, want banks=12 auto_id=true", cfg.General)
	}

	if len(cfg.Banks) != len(want.Banks) {
		t.Fatalf("len(Banks) = %d, want %d", len(cfg.Banks), len(want.Banks))
	}

	for key, wantBank := range want.Banks {
		if got := cfg.Banks[key]; got != wantBank {
			t.Errorf("Banks[%q] = %+v, want %+v", key, got, wantBank)
		}
	}
}

func TestConfigOpenOmittedFieldsUseZeroValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"general":{},"Banks":{"empty":{}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := cfg.Open(path); err == nil {
		t.Fatalf("Open() should fail, got nil error")
	}

	if cfg.General != (General{}) {
		t.Errorf("General = %+v, want zero value", cfg.General)
	}
	if got := cfg.Banks["empty"]; got != (BankInfo{}) {
		t.Errorf("Banks[empty] = %+v, want zero value", got)
	}
}

func TestConfigOpenErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		var cfg Config
		if err := cfg.Open(filepath.Join(t.TempDir(), "missing.json")); err == nil {
			t.Fatal("Open() error = nil, want file read error")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(`{"general":`), 0o600); err != nil {
			t.Fatal(err)
		}

		var cfg Config
		if err := cfg.Open(path); err == nil {
			t.Fatal("Open() error = nil, want JSON syntax error")
		}
	})
}
