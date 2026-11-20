// moddec decodes tracker modules to raw stereo PCM.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/erik-adelbert/duh/pkg/mod"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

type args struct {
	input    io.ReadCloser
	sequence int
	noLoop   bool
}

func main() {
	a := getArgs()

	in := a.input
	defer in.Close() //nolint:errcheck // can't do anything if it fails

	src, err := mod.Decode(in, mod.DecoderOptions{Sequence: a.sequence, NoLoop: a.noLoop})
	die(err, "Failed to decode module", 1)

	defer src.Close() //nolint:errcheck // can't do anything if it fails

	for _, warning := range src.Warnings() {
		fmt.Fprintf(os.Stderr, "Warning: %s.\n", warning)
	}

	_, err = io.Copy(os.Stdout, src)
	die(err, "Failed to write decoded audio", 1)
}

func getArgs() args {
	fs := parseFlags()

	switch len(flag.Args()) {
	case 0:
		// no positional arguments, do nothing
	case 1:
		if *fs.path != "-" {
			die(errors.New("already specified"), "Invalid input file argument", 2)
		}
		*fs.path = flag.Args()[0]
	default:
		die(errors.New("too many positional arguments"), "Invalid input file argument", 2)
	}

	if *fs.path == "-" {
		return args{input: io.NopCloser(os.Stdin), sequence: *fs.sequence, noLoop: *fs.noLoop}
	}

	file, err := os.Open(*fs.path)
	die(err, "Failed to open module", 1)
	return args{input: file, sequence: *fs.sequence, noLoop: *fs.noLoop}
}

type flags struct {
	path     *string
	sequence *int
	noLoop   *bool
}

func parseFlags() flags {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Decode S3M, IT, experimental MPTM and basic ProTracker M.K. MOD to f32le stereo PCM at 48000 Hz")
		fmt.Fprintf(os.Stderr, "\nUsage: %s [flags] [file]\n\nFlags:\n", os.Args[0])
		flag.PrintDefaults()
	}

	fs := flags{
		path:     flag.String("f", "-", "path to module file (default: stdin)"),
		sequence: flag.Int("seq", 0, "zero-based MPTM sequence index"),
		noLoop:   flag.Bool("noloop", false, "stop when song playback repeats (sample and finite pattern loops are preserved)"),
	}
	flag.Parse()
	return fs
}

var die = xflags.MkDie(os.Stderr)
