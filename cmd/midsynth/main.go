// midsynth renders live MIDI input as stereo PCM on stdout.
package main

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/erik-adelbert/duh/pkg/mid"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

const (
	blockFrames = mid.PCMRate / 100
	eventBuffer = 1024
)

func main() {
	a := getArgs()
	err := run(a)
	die(err, "MIDI synthesis failed", 1)
}

func run(a args) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	ports, err := midiInputs(ctx)
	if err != nil {
		return fmt.Errorf("list MIDI inputs: %w", err)
	}

	if a.list {
		for _, port := range ports {
			fmt.Fprintf(os.Stderr, "%d: %s\n", port.number, port.name)
		}
		return nil
	}

	if a.portNumber < 0 {
		return errors.New("select a MIDI input with -in; use -list to see ports")
	}

	if a.cards < 1 || a.cards > 100 || a.fourOps < -1 || a.fourOps > 6*a.cards {
		return errors.New("cards must be 1..100 and four-ops must be -1 or 0..6 per card")
	}

	if mid.ErrBuiltin != nil {
		return fmt.Errorf("load built-in banks: %w", mid.ErrBuiltin)
	}

	if a.bank < 0 || a.bank >= len(mid.BankNames()) {
		return fmt.Errorf("invalid ADL bank %d", a.bank)
	}

	if a.fourOps == -1 {
		nquad, total, err := mid.Melodic4OpCount(a.bank)
		if err != nil {
			return fmt.Errorf("inspect ADL bank: %w", err)
		}
		switch {
		case total == 0 || nquad*8 < total:
			a.fourOps = 0
		case nquad*8 >= total*7:
			a.fourOps = a.cards * 6
		case a.cards == 1:
			a.fourOps = 1
		default:
			a.fourOps = a.cards * 4
		}
	}

	var port *midiInput
	for _, candidate := range ports {
		if candidate.number == a.portNumber {
			port = &candidate
			break
		}
	}

	if port == nil {
		return fmt.Errorf("MIDI input %d not found", a.portNumber)
	}

	synth := mid.NewSynth(a.cards, a.fourOps, 0)
	if err := synth.SetBank(a.bank); err != nil {
		return err
	}

	events := make(chan []byte, eventBuffer)
	failures := make(chan error, 1)

	stop, err := listenMIDI(ctx, port.number, events, failures)
	if err != nil {
		return fmt.Errorf("open MIDI input: %w", err)
	}
	defer stop()

	fmt.Fprintf(os.Stderr, "midsynth: listening to %d: %s\n", port.number, port.name)
	return render(ctx, synth, os.Stdout, events, failures)
}

func render(ctx context.Context, synth *mid.Synth, out io.Writer, events <-chan []byte, failures <-chan error) error {
	samples := make([]int16, blockFrames*2)
	pcm := make([]byte, len(samples)*2)

	period := time.Second / 100
	nxt := time.Now()
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case err := <-failures:
			return fmt.Errorf("MIDI input: %w", err)

		case msg := <-events:
			if err := handleMIDI(synth, msg); err != nil {
				return fmt.Errorf("MIDI event % X: %w", msg, err)
			}

		case <-timer.C:
			if _, err := synth.RTGenerate(samples); err != nil {
				return fmt.Errorf("render PCM: %w", err)
			}
			for i, sample := range samples {
				binary.LittleEndian.PutUint16(pcm[2*i:], uint16(sample))
			}

			if err := writeAll(out, pcm); err != nil {
				return fmt.Errorf("write PCM: %w", err)
			}

			nxt = nxt.Add(period)
			now := time.Now()
			if nxt.Before(now) {
				nxt = now
			}
			timer.Reset(time.Until(nxt))
		}
	}
}

func writeAll(out io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := out.Write(data)

		switch {
		case n > len(data):
			return errors.New("invalid write count")
		case err != nil:
			return err
		case n == 0:
			return io.ErrShortWrite
		}

		data = data[n:]
	}

	return nil
}

func handleMIDI(synth *mid.Synth, msg []byte) error {
	if len(msg) == 0 {
		return errors.New("empty MIDI message")
	}

	if msg[0] == 0xf0 {
		_, err := synth.RTSystemExclusive(msg)
		return err
	}

	status, channel := msg[0]&0xf0, msg[0]&0x0f
	if status < 0x80 || status >= 0xf0 {
		return nil
	}

	want := 3
	if status == 0xc0 || status == 0xd0 {
		want = 2
	}

	if len(msg) != want {
		return fmt.Errorf("invalid message length %d, want %d", len(msg), want)
	}

	for _, value := range msg[1:] {
		if value > 127 {
			return fmt.Errorf("invalid MIDI data byte %#x", value)
		}
	}

	switch status {
	case 0x80:
		return synth.RTNoteOff(channel, msg[1])
	case 0x90:
		_, err := synth.RTNoteOn(channel, msg[1], msg[2])
		return err
	case 0xa0:
		return synth.RTNoteAfterTouch(channel, msg[1], msg[2])
	case 0xb0:
		return synth.RTControllerChange(channel, msg[1], msg[2])
	case 0xc0:
		return synth.RTPatchChange(channel, msg[1])
	case 0xd0:
		return synth.RTChannelAfterTouch(channel, msg[1])
	case 0xe0:
		return synth.RTPitchBendML(channel, msg[2], msg[1])
	}

	return nil
}

type args struct {
	list       bool
	portNumber int
	bank       int
	cards      int
	fourOps    int
}

type flags struct {
	set        *flag.FlagSet
	list       *bool
	portNumber *int
	bank       *int
	cards      *int
	fourOps    *int
}

func parseFlags() flags {
	set := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	set.Usage = func() {
		fmt.Fprintf(set.Output(), "Usage: %s -in PORT [options] > output.pcm\n", os.Args[0])
		fmt.Fprintln(set.Output(), "Output: 48000 Hz, stereo, signed 16-bit little-endian PCM.")
		set.PrintDefaults()
	}

	fs := flags{
		set:        set,
		list:       set.Bool("list", false, "list MIDI input ports on stderr"),
		portNumber: set.Int("in", -1, "MIDI input port number (required)"),
		bank:       set.Int("bank", 0, "built-in ADL bank number"),
		cards:      set.Int("cards", 4, "number of OPL3 cards (1..100)"),
		fourOps:    set.Int("four-ops", -1, "hardware 4-op channels (-1 selects automatically)"),
	}

	err := set.Parse(os.Args[1:])
	die(err, "Invalid flags", 2)

	return fs
}

func getArgs() args {
	fs := parseFlags()
	if fs.set.NArg() != 0 {
		die(errors.New("unexpected positional arguments"), "Invalid arguments", 2)
	}
	return args{
		list:       *fs.list,
		portNumber: *fs.portNumber,
		bank:       *fs.bank,
		cards:      *fs.cards,
		fourOps:    *fs.fourOps,
	}
}

var die = xflags.MkDie(os.Stderr)
