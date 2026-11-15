package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMIDICatVersion(t *testing.T) {
	for _, tt := range []struct {
		version string
		valid   bool
	}{
		{"0.9.5\n", true},
		{"0.9.0", true},
		{"0.8.2", false},
		{"0.10.0", false},
		{"0.9.invalid", false},
	} {
		if err := checkMIDICatVersion(tt.version); (err == nil) != tt.valid {
			t.Errorf("version %q: %v", tt.version, err)
		}
	}
}

func TestParseMIDIInputs(t *testing.T) {
	got, err := parseMIDIInputs([]byte(`{"10":"ten","2":"two"}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []midiInput{{2, "two"}, {10, "ten"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ports = %v, want %v", got, want)
	}
	for _, input := range []string{`null`, `{`, `{"-1":"bad"}`, `{"x":"bad"}`} {
		if _, err := parseMIDIInputs([]byte(input)); err == nil {
			t.Errorf("accepted invalid input list %q", input)
		}
	}
}

func TestMIDIInputHelper(t *testing.T) {
	switch os.Getenv("MIDSYNTH_TEST_INPUT") {
	case "message":
		fmt.Fprintln(os.Stdout, "0 903C64")
		time.Sleep(time.Minute)
		os.Exit(0)
	case "exit":
		os.Exit(0)
	case "invalid":
		fmt.Fprintln(os.Stdout, "0 ZZ")
		time.Sleep(time.Minute)
		os.Exit(0)
	case "overflow":
		fmt.Fprintln(os.Stdout, "0 903C64")
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}

func TestMIDIInputProcess(t *testing.T) {
	for _, tt := range []struct {
		name        string
		buffer      int
		wantMessage bool
		wantError   string
	}{
		{"message", 1, true, ""},
		{"exit", 1, false, "terminated unexpectedly"},
		{"invalid", 1, false, "read midicat input"},
		{"overflow", 0, false, "queue overflow"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMIDIInputHelper$")
			cmd.Env = append(os.Environ(), "MIDSYNTH_TEST_INPUT="+tt.name)
			events := make(chan []byte, tt.buffer)
			failures := make(chan error, 1)
			stop, err := startMIDIInput(ctx, cancel, cmd, events, failures)
			if err != nil {
				t.Fatal(err)
			}
			defer stop()

			if tt.wantMessage {
				select {
				case msg := <-events:
					if !bytes.Equal(msg, []byte{0x90, 60, 100}) {
						t.Fatalf("message = % X", msg)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("timed out waiting for MIDI message")
				}
				stop()
				select {
				case err := <-failures:
					t.Fatalf("unexpected failure after stop: %v", err)
				default:
				}
			} else {
				select {
				case err := <-failures:
					if !strings.Contains(err.Error(), tt.wantError) {
						t.Fatalf("error = %v, want %q", err, tt.wantError)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("timed out waiting for input failure")
				}
			}
		})
	}
}
