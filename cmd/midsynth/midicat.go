package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"gitlab.com/gomidi/midi/v2/drivers/midicat"
)

type midiInput struct {
	number int
	name   string
}

func midiInputs(ctx context.Context) ([]midiInput, error) {
	version, err := exec.CommandContext(ctx, "midicat", "version", "-s").Output()

	if err != nil {
		return nil, fmt.Errorf("run midicat version (install midicat 0.9.x): %w", err)
	}

	if err := checkMIDICatVersion(string(version)); err != nil {
		return nil, err
	}

	output, err := exec.CommandContext(ctx, "midicat", "ins", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("run midicat ins: %w", err)
	}
	return parseMIDIInputs(output)
}

func checkMIDICatVersion(version string) error {
	parts := strings.Split(strings.TrimSpace(version), ".")

	if len(parts) != 3 || parts[0] != "0" || parts[1] != "9" {
		return fmt.Errorf("unsupported midicat version %q, want 0.9.x", strings.TrimSpace(version))
	}

	if _, err := strconv.ParseUint(parts[2], 10, 16); err != nil {
		return fmt.Errorf("invalid midicat version %q", strings.TrimSpace(version))
	}

	return nil
}

func parseMIDIInputs(output []byte) ([]midiInput, error) {
	var names map[string]string

	if err := json.Unmarshal(output, &names); err != nil || names == nil {
		return nil, fmt.Errorf("invalid midicat input list: %q", output)
	}

	ports := make([]midiInput, 0, len(names))
	for key, name := range names {
		number, err := strconv.Atoi(key)
		if err != nil || number < 0 {
			return nil, fmt.Errorf("invalid midicat input index %q", key)
		}
		ports = append(ports, midiInput{number, name})
	}

	sort.Slice(ports, func(i, j int) bool { return ports[i].number < ports[j].number })

	return ports, nil
}

func listenMIDI(ctx context.Context, port int, events chan<- []byte, failures chan<- error) (func(), error) {
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, "midicat", "in", fmt.Sprintf("--index=%d", port))
	cmd.Stderr = os.Stderr

	stop, err := startMIDIInput(ctx, cancel, cmd, events, failures)
	if err != nil {
		cancel()
		return nil, err
	}

	return stop, nil
}

func startMIDIInput(ctx context.Context, cancel context.CancelFunc, cmd *exec.Cmd, events chan<- []byte, failures chan<- error) (func(), error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		reader := bufio.NewReader(stdout)
		var readErr error
		for {
			msg, _, err := midicat.ReadAndConvert(reader)
			if err != nil {
				readErr = err

				if err != io.EOF {
					readErr = fmt.Errorf("read midicat input: %w", err)
				}

				break
			}

			select {
			case events <- msg:
			default:
				readErr = errors.New("MIDI event queue overflow")
			}

			if readErr != nil {
				break
			}
		}

		reportFailure := ctx.Err() == nil
		if reportFailure {
			cancel()
		}

		waitErr := cmd.Wait()
		if !reportFailure {
			return
		}

		if readErr == io.EOF {
			readErr = errors.Join(errors.New("midicat input terminated unexpectedly"), waitErr)
		}

		select {
		case failures <- readErr:
		default:
		}
	}()

	return func() {
		cancel()
		<-done
	}, nil
}
