<p align="left">
  <img src="duh.svg" alt="duh" width="100" height="56">
</p>

# duh?

[![Go Reference](https://pkg.go.dev/badge/github.com/erik-adelbert/duh.svg)](https://pkg.go.dev/github.com/erik-adelbert/duh)
> An audio toolkit. Pure Go, no cgo in the toolkit, nothing you didn't ask for.

`duh` is a developer library for Go. It is also a toolkit for working with audio from the shell — decoding, encoding, resampling, and the plumbing in between. The toolkit is written in Go; audio playback and external tools have their own platform requirements.

## Watch a demo on YouTube

[![duh demo](https://img.youtube.com/vi/ifc32HdhSic/maxresdefault.jpg)](https://www.youtube.com/watch?v=ifc32HdhSic)

Note. The demo uses `oto` for audio output.

## Try a demo in your browser

- [MIDI demo](https://erik-adelbert.github.io/duh/midweb/)
- [Tracker demo](https://erik-adelbert.github.io/duh/modweb/)
- [VGM demo](https://erik-adelbert.github.io/duh/vgmweb/)

## Status

**Work in progress. Not yet tagged. No release.**

The API is not stable. Things will move, get renamed, and probably get deleted. Don't depend on it yet — but do kick the tires and file issues (except for `/internal`).

The first tag (`v0.1.0`) will land once the core path is settled. Until then, `main` is the only branch that matters.

[Read the blog](https://erik-adelbert.github.io/duh/)

## Goals

- **Pure Go.** Keep the toolkit buildable and testable with `CGO_ENABLED=0`, subject to platform and dependency requirements.
- **Small surface area.** Do a few things well instead of everything badly.
- **Predictable.** Unix-compliant, no hidden magic.
- **Portable.** Aim for Linux, macOS, Windows, and other Go targets where the dependencies are supported.
- **Efficient.** Fast enough to stay out of the way, honest enough to publish benchmarks instead of claims.
- **Robust.** Should not fall on its face under any circumstances.

## Non-goals

- Being a DAW, a full-featured synth, or a general-purpose media player
- Replacing `ffmpeg` for video
- Chasing every codec under the sun
- Winning a benchmark war against a C library

## Install

The module requires Go 1.27 or newer. To add a package to your Go project:

```sh
go get github.com/erik-adelbert/duh/pkg/pcm@main
```

There is no package at the module root. See the [package reference](https://pkg.go.dev/github.com/erik-adelbert/duh) for the available packages.

To build the commands from a checkout:

```sh
make build
```

This writes binaries to `bin/` and generates their man pages in `man/`.
Use `make BINARY=pcmconv build` to build one command, or
`go install ./cmd/pcmconv` to install it into Go's binary installation directory.
Each command accepts `-h`; the [man pages](man/) contain the same usage information.
Run `go test ./...` to test the project. The browser builds are separate:
`make midweb`, `make modweb`, and `make vgmweb`.

## Toolkit

- `pcmconv` converts and resamples raw PCM; `a440` generates test signals.
- `wavdec`, `wavenc`, `wavinfo`, `mp3dec`, `mp3enc`, `mp3info`,
  `aacdec`, `aacenc`, and `aacinfo` work with WAV, MP3 and AAC.
- `middec` renders MIDI files, `midsynth` handles live MIDI input, and
  `moddec` renders S3M, IT, experimental MPTM and basic ProTracker MOD modules.
- `vgmdec`, `vgminfo`, and `vgmwav` work with VGM files.
- `pcmtui`, `wavtui`, and `vgmtui` play audio with terminal interfaces.
- `ppunpack` and `unmmcmp` decompress PP20 and MMCMP files.

The OPL3 register view in vgmweb and `vgmtui -opl3` shows both operators on
all 18 channels. It samples register state, so very brief changes can occur
between display updates; the full view also needs a tall terminal.

The corresponding library packages live under [`pkg/`](pkg/), including
`pcm`, `wav`, `mp3`, `tphakala/aac`, `mid`, `mod`, `vgm`, and `opl3`.

## Live MIDI synthesis

`midsynth` reads an existing MIDI input port and writes real-time PCM to
stdout. It requires the `midicat` executable (version 0.9.x) in `PATH`;
`midicat` uses native MIDI drivers and is a separate program from `duh`.
Run `make midicat` to install a compatible version
with Go; this requires a C/C++ toolchain and platform MIDI development libraries.
Ensure Go's binary installation directory is in `PATH`. Alternatively, download a
[binary](https://gitlab.com/gomidi/tools/-/releases).

```sh
go run ./cmd/midsynth -list
go run ./cmd/midsynth -in 0 -bank 79 > output.pcm
```

Output is raw signed 16-bit little-endian stereo at 48 kHz. Pipe it to a
player configured for that format; player and pipe buffering add latency.
Use Ctrl-C to stop. `-cards` and `-four-ops` configure the OPL3 voices.
The program cannot create a virtual MIDI input port; connect an existing
hardware or virtual input exposed by `midicat`. `midsynth` reports
unexpected input-process termination.

There are 82 built-in ADL banks, numbered 0 through 81. Banks can also be
changed via DUH SysEx; doing so does not recalculate the `-four-ops` setting.

## Tracker decoding

`moddec` auto-detects S3M, IT, experimental MPTM and basic ProTracker MOD.
MOD support currently covers classic four-channel `M.K.` files: notes,
samples and sample loops play, with selected timing, volume, pitch, and sample
effects. Unsupported effects are skipped with warnings.
See the [tracker compatibility inventory](documentation/tracker/tracker-compatibility.md)
for details, including MMCMP support.

```sh
go run ./cmd/moddec /path/to/song.it | ffplay -nodisp -autoexit -f f32le -sample_rate 48000 -ch_layout stereo -
cat /path/to/song.mptm | go run ./cmd/moddec > song.pcm
```

Without a path, `moddec` reads stdin. Output is raw 32-bit little-endian float
stereo PCM at 48 kHz. Use `-noloop` to stop when playback repeats, and `-seq N`
to select an MPTM sequence.

`modweb` estimates the selected sequence's duration without rendering audio.
With NoLoop disabled, the displayed time is the first pass before playback
repeats. Complex modules can exceed the calculation limit; playback continues
without a duration in that case.

Support is incomplete and playback can differ from other players. See the
[tracker compatibility inventory](documentation/tracker/tracker-compatibility.md)
for limitations and the [tracker development guide](documentation/tracker/tracker-development.md)
for implementation details.

## Dependencies

Go module dependencies are listed in [go.mod](go.mod). Run `go list -m all`
for the full module graph. Several dependencies come from `bubbletea`, used by
`wavtui`; the intent is to remove it in time.

Audio playback uses `oto` and requires a working platform audio backend.
On Linux and BSD, Oto uses PulseAudio or loads ALSA at runtime. See
[Oto's platform notes](https://github.com/ebitengine/oto#platforms) for
requirements and cross-compilation details. Live MIDI input additionally
requires the external `midicat` program described above.

## Credits

`duh` includes ports and uses libraries and references from other projects:

- pcm: plan9 [libpcm](https://git.9front.org/plan9front/plan9front/4ed03dff937d78f81be88b40cb4cb46372ba9344/sys/src/libpcm)
- opl3: [Nuked-OPL3](https://github.com/nukeykt/Nuked-OPL3), [Nuked-OPL3-Fast](https://github.com/tgies/Nuked-OPL3-fast)
- aac: [tphakala](https://github.com/tphakala/go-aac)
- mp3: [go-mp3](https://github.com/hajimehoshi/go-mp3) + [shine-mp3](https://github.com/braheezy/shine-mp3)
- midi: [libADLMIDI](https://github.com/Wohlstand/libADLMIDI), [adlMIDI](https://github.com/bisqwit/adlmidi), [DMXOPL](https://github.com/sneakernets/DMXOPL)
- tracker: [OpenMPT](https://openmpt.org/) is the main playback reference and provides material including the random modulation table and playback definitions

The AAC and MP3 packages use external Go modules; see [go.mod](go.mod).

Note. Dear Mr. Cinap-Lenrek, `pcmconv` is back and it has new friends!

## License

See [LICENCE](LICENCE) for the project license and the notices in
[`pkg/pcm`](pkg/pcm/LICENCE), [`pkg/opl3`](pkg/opl3/LICENCE), and
[`pkg/mid`](pkg/mid/LICENCE) for incorporated code. The original tracker
code is covered by the project license; see its [notice](pkg/mod/NOTICE)
and the retained [OpenMPT license](pkg/mod/openmpt-LICENSE) for third-party
material. Tracker binaries also include LGPL-licensed dependencies.
See the [MIDI notice](pkg/mid/NOTICE) for bank attribution and unresolved
provenance. Other dependencies may have their own license requirements.

## llm

$10/month Copilot writes a lot of the comments and even some tests, proofreads, generates skeletons for social files, gives me on-demand links about the state of the art or consensus on programming questions, and helps with C deobfuscation. That's pretty much it. Otherwise, `duh` is minimalist, hand-typed and carefully curated — you should be able to tell.

$29/month Copilot heavily assisted me with the tracker. More on this later.
