//go:build !windows

// adlmidi-go: A Go port of ADLMIDI (OPL3 emulation MIDI player)
// Streams raw PCM (S16LE, stereo) to stdout.
//
// Usage:
//   adlmidi-go [options] <midifilename> [<banknumber> [<numcards> [<numfourops>]]]
//
// Options are parsed with Go's flag package. Since Go's flag package stops
// parsing at the first non-flag argument, all options must come BEFORE the
// MIDI filename.

package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	mid "github.com/erik-adelbert/duh/pkg/mid"
)

// ---------------------------------------------------------------------------
// Audio / timing configuration
// ---------------------------------------------------------------------------

const (
	// Maximum number of OPL3 cards that can be emulated.
	MaxCards = 100

	// Maximum number of samples we allow to accumulate at once.
	MaxSamplesAtTime = mid.PCMRate / 40
)

// ---------------------------------------------------------------------------
// Global player state (replaces the globals from the original C++ file)
// ---------------------------------------------------------------------------

var (
	// Options
	AdlPercussionMode  bool
	HighVibratoMode    bool
	HighTremoloMode    bool
	ScaleModulators    bool
	SoftPanning        bool
	QuitWithoutLooping bool

	AdlBank    int
	NumCards   int
	NumFourOps int
	// DoingInstrumentTesting bool
	// QuitFlag    bool
	SkipForward int
)

var VolumeModel int

var (
	banknames = func() []string {
		names := make([]string, len(mid.Banks))
		for i, bank := range mid.Banks {
			names[i] = bank.Name
		}
		return names
	}()
)

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	// ---- Flag definitions -------------------------------------------------
	// All flags MUST appear before the positional arguments.

	flag.BoolVar(&AdlPercussionMode, "p", false,
		"Enable adlib percussion instrument mode (use with CMF files)")
	flag.BoolVar(&HighTremoloMode, "t", false,
		"Enable tremolo amplification mode")
	flag.BoolVar(&HighVibratoMode, "v", false,
		"Enable vibrato amplification mode")
	flag.BoolVar(&ScaleModulators, "s", false,
		"Enable scaling of modulator volumes")
	flag.BoolVar(&SoftPanning, "sp", false,
		"Enable soft panning (full stereo positioning instead of left/right/both)")
	flag.IntVar(&VolumeModel, "vm", 0,
		"Volume model: 0 = from bank, 1 generic, 2 native, 3 DMX, 4 Apogee, 5 Win9x SB16,\n"+
			"6 DMX fixed, 7 Apogee fixed, 8 AIL, 9 Win9x generic FM, 10 HMI, 11 HMI old,\n"+
			"12 MS AdLib, 13 IMF Creator, 14 O'Connell")
	flag.BoolVar(&QuitWithoutLooping, "nl", true,
		"Quit without looping")

	flag.BoolVar(&WritePCMfile, "w", false,
		"Write WAV/PCM to stdout rather than playing (default: always stdout)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"Usage: %s [options] <midifilename> [ <banknumber> [ <numcards> [ <numfourops>] ] ]\n"+
				"       %s [options] <midifilename> -1   To enter instrument tester\n\n",
			os.Args[0], os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nBanks:\n")
		for a, name := range banknames {
			fmt.Fprintf(os.Stderr, "%10s%2d = %s\n", "", a, name)
		}
		fmt.Fprintf(os.Stderr,
			"\n"+
				"     Use banks 2-5 to play Descent \"q\" soundtracks.\n"+
				"     Look up the relevant bank number from descent.sng.\n\n"+
				"     <numfourops> can be used to specify the number\n"+
				"     of four-op channels to use. Each four-op channel eats\n"+
				"     the room of two regular channels. Use as many as required.\n"+
				"     The Doom & Hexen sets require one or two, while\n"+
				"     Miles four-op set requires the maximum of numcards*6.\n\n"+
				"     When playing Creative Music Files (CMF), try the\n"+
				"     -p and -v options if it sounds wrong otherwise.\n\n")
	}

	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(0)
	}

	midiFile := flag.Args()[0]
	posArgs := flag.Args()[1:]

	// ---- Bank / cards / fourops ------------------------------------------
	AdlBank = 0
	NumCards = 4
	NumFourOps = 7

	if len(posArgs) >= 1 {
		nbank := len(banknames)

		bankno, _ := strconv.Atoi(posArgs[0])

		if bankno == -1 {
			bankno = 0
			DoingInstrumentTesting = true
		}

		AdlBank = bankno
		if AdlBank < 0 || AdlBank >= nbank {
			fmt.Fprintf(os.Stderr, "bank number may only be 0..%d.\n", nbank-1)
			os.Exit(0)
		}
	}

	if len(banknames) == 0 {
		fmt.Fprintln(os.Stderr, "no ADL banks are available")
		os.Exit(1)
	}

	// Count four-op melodic patches in the selected bank.
	var nquad, sums [2]uint
	if len(mid.Banks[AdlBank].Melos) > 0 {
		midiBank := mid.Banks[AdlBank].Melos[0]
		for _, metaID := range midiBank.Programs {
			if metaID == 0 || int(metaID) >= len(mid.ADLMetas) {
				continue
			}
			sums[0]++
			meta := mid.ADLMetas[metaID]
			if meta.ADLID0 != meta.ADLID1 {
				nquad[0]++
			}
		}
	}

	if len(posArgs) >= 2 {
		NumCards, _ = strconv.Atoi(posArgs[1])

		if NumCards < 1 || NumCards > MaxCards {
			fmt.Fprintf(os.Stderr, "number of cards may only be 1..%d.\n", MaxCards)
			os.Exit(0)
		}
	}

	if len(posArgs) >= 3 {
		NumFourOps, _ = strconv.Atoi(posArgs[2])

		if NumFourOps < 0 || NumFourOps > 6*NumCards {
			fmt.Fprintf(os.Stderr,
				"number of four-op channels may only be 0..%d when %d OPL3 cards are used.\n",
				6*NumCards, NumCards)
			os.Exit(0)
		}
	} else if sums[0] != 0 {
		switch {
		case DoingInstrumentTesting:
			NumFourOps = 2
		case nquad[0] >= sums[0]*7/8:
			NumFourOps = NumCards * 6
		case nquad[0] < sums[0]*1/8:
			NumFourOps = 0
		case NumCards == 1:
			NumFourOps = 1
		default:
			NumFourOps = NumCards * 4
		}
	}

	if nquad[0] >= sums[0]*15/16 && NumFourOps == 0 {
		fmt.Fprintf(os.Stderr,
			"ERROR: You have selected a bank that consists almost exclusively of four-op patches.\n"+
				"       The results (silence + much cpu load) would be probably\n"+
				"       not what you want, therefore ignoring the request.\n")
		os.Exit(0)
	}

	mode := 0
	if AdlPercussionMode {
		mode |= mid.ModeADLPercussion
	}

	if HighTremoloMode {
		mode |= mid.ModeHighTremolo
	}

	if HighVibratoMode {
		mode |= mid.ModeHighVibrato
	}

	if ScaleModulators {
		mode |= mid.ModeScaleModulators
	}

	if SoftPanning {
		mode |= mid.ModeSoftPanning
	}

	// ---- Initialise player ------------------------------------------------
	mid.BankSelect = uint(AdlBank)
	player := mid.NewMIDIDecoder(NumCards, NumFourOps, mode)
	player.ChooseDevice("")
	player.SetVolumeModel(VolumeModel)

	// ---- Signals ----------------------------------------------------------
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigchan
		// QuitFlag = true
		player.Quit()
	}()

	err := player.Open(midiFile)

	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open MIDI file: %v\n", err)
		os.Exit(2)
	}

	player.SetNoLooping(QuitWithoutLooping)

	// ---- Instrument tester -------------------------------------------------
	// var InstrumentTester Tester
	// if DoingInstrumentTesting {
	// 	InstrumentTester = mid.NewTester(player.opl)
	// }

	// ---- Main loop --------------------------------------------------------
	// mindelay := time.Second / time.Duration(PCMRate)
	// maxdelay := time.Second * time.Duration(MaxSamplesAtTime) / time.Duration(PCMRate)

	// delay := mindelay
	// carry := 0 * time.Second

	var dt time.Duration

	buf := make([]int16, MaxSamplesAtTime*2)
	tmp := make([]int16, MaxSamplesAtTime*2)
	acc := make([]int32, MaxSamplesAtTime*2)

	for !player.IsFinished() {
		wait := player.Tick(dt)

		// dt is exactly what gets rendered, so the sequencer never drifts.
		frames := max(1, min(int(wait.Seconds()*mid.PCMRate), MaxSamplesAtTime))
		dt = time.Duration(frames) * time.Second / time.Duration(mid.PCMRate)

		n := frames * 2
		clear(acc[:n])

		for card := range NumCards {
			player.OPL.Cards[card].Stream2Ch(tmp[:n], n)

			for i, s := range tmp[:n] {
				acc[i] += int32(s)
			}
		}

		for i, s := range acc[:n] {
			buf[i] = clip16(s) // int16(max(-32768, min(32767, s)))
		}

		SendStereoAudio(buf[:n])
	}
}

// ---------------------------------------------------------------------------
// Helpers and stubs for ported types
// ---------------------------------------------------------------------------

// -------- Configuration / globals mirrored from the C code --------

var (
	DoingInstrumentTesting bool
	WritePCMfile           bool   // set true to emit PCM
	PCMfilepath            string // "-" means stdout
)

// -------- Ported SendStereoAudio --------

func SendStereoAudio(samples []int16) {
	for len(samples) > 0 {
		frames := min(len(samples)/2, MaxSamplesAtTime)

		if frames == 0 {
			return
		}

		chunk := samples[:frames*2]
		samples = samples[frames*2:]
		sendChunkDirect(chunk)
	}
}

func sendChunkDirect(samples []int16) {
	buf := make([]byte, len(samples)*2)

	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}

	os.Stdout.Write(buf)
}

type SampleFmt interface {
	~int32 | ~float32
}

func clip16[T SampleFmt](val T) int16 {
	switch {
	case val < -32768:
		return -32768
	case val > 32767:
		return 32767
	}
	return int16(val)
}
