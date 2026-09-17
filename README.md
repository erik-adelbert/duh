<p align="left">
  <img src="duh.svg" alt="duh" width="100" height="56">
</p>

# duh?

[![Go Reference](https://pkg.go.dev/badge/github.com/erik-adelbert/duh.svg)](https://pkg.go.dev/github.com/erik-adelbert/duh)
> An audio toolkit. Pure Go, no cgo, nothing you didn't ask for.

`duh` is a dev library for Go. It is also a toolkit for working with audio from the shell — decoding, encoding, resampling, and the plumbing in between. Written in pure Go so it builds anywhere Go builds, with no C toolchain, no surprises.

## Watch a demo on YouTube

[![duh demo](https://img.youtube.com/vi/ifc32HdhSic/maxresdefault.jpg)](https://www.youtube.com/watch?v=ifc32HdhSic)

Note. Single dependency on `oto`

## Status

**Work in progress. Not yet tagged. No release.**

The API is not stable. Things will move, get renamed, and probably get deleted. Don't depend on it yet — but do kick the tires and file issues (except for `/internal`).

The first tag (`v0.1.0`) will land once the core path is settled. Until then, `main` is the only branch that matters.

[Read the blog](https://erik-adelbert.github.io/duh/)

## Goals

- **Pure Go.** `CGO_ENABLED=0` should always build and pass tests.
- **Small surface area.** Do a few things well instead of everything badly.
- **Predictable.** Unix compliant, no hidden magic.
- **Portable.** Linux, macOS, Windows, and whatever `GOOS` you're cross-compiling to at 2am.
- **Efficient.** Fast enough to stay out of the way, honest enough to publish benchmarks instead of claims.
- **Robust.** Should not fall on the face under any circumstances.

## Non-goals

- Being a DAW, a synth, or a media player
- Replacing `ffmpeg` for video
- Chasing every codec under the sun
- Winning a benchmark war against a C library

## Install

```sh
go get github.com/erik-adelbert/duh
```

## Dependencies

```sh
❯ cat go.mod | grep -v duh | grep github | cut -d "/" -f 2,3 | sort | cut -d " " -f 1
aymanbagabas/go-osc52
braheezy/shine-mp3
charmbracelet/bubbletea
charmbracelet/colorprofile
charmbracelet/lipgloss
charmbracelet/x
charmbracelet/x
charmbracelet/x
clipperhouse/displaywidth
clipperhouse/uax29
ebitengine/oto
ebitengine/purego
erikgeiser/coninput
hajimehoshi/go-mp3
jfreymuth/pulse
lucasb-eyer/go-colorful
mattn/go-isatty
mattn/go-localereader
mattn/go-runewidth
muesli/ansi
muesli/cancelreader
muesli/termenv
rivo/uniseg
tphakala/go-aac
tphakala/simd
xo/terminfo
```

Note. Most of it is from `bubbletea` used only in `wavtui` and should be removed in time.

## Credits

`duh` features Go ports more or less reworked but frame exact. Except for [Tomi Hakala's](https://github.com/tphakala) selected works that are treated as a submodule in the repo, the rest is directly integrated at the package level:

- pcm: plan9 [libpcm](https://git.9front.org/plan9front/plan9front/4ed03dff937d78f81be88b40cb4cb46372ba9344/sys/src/libpcm)
- opl3: [Nuked-OPL3](https://github.com/nukeykt/Nuked-OPL3), [Nuked-OPL3-Fast](https://github.com/tgies/Nuked-OPL3-fast)
- aac: [tphakala](https://github.com/tphakala/go-aac)
- mp3: [go-mp3](https://github.com/hajimehoshi/go-mp3) + [shine-mp3](https://github.com/braheezy/shine-mp3)

Note. Dear Mr. Cinap-Lenrek, `pcmconv` is back, it has new friends!

## llm

$10/mth Copilot writes a lot of the comments and even some tests, proofreads, generates social files skeletons, give me on-demand links about state-of-the-art or consensus on programming questions and C deobfuscation and that's pretty much it. Otherwise, `duh` is minimalist, handtyped and carefully curated - You should be able to tell.
