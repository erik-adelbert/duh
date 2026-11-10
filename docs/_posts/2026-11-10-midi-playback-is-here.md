---
layout: single
title: "OPL3 general MIDI in duh"
date: 2026-09-21
categories: [meta]
---

MIDI playback is here! It was tedious, but in the end it turned out quite well.

Basically, it's a port/remix of [`libADLMIDI`](https://github.com/Wohlstand/libADLMIDI) and [`adlMIDI`](https://github.com/bisqwit/adlmidi). The best-sounding banks come from [`DMXOPL`](https://github.com/sneakernets/DMXOPL).

duh features a new MIDI decoder and a new [WASM+JS demo](https://erik-adelbert.github.io/duh/midweb). In the new scripts folder, you'll find a first SoX-based reverb bash script that's easy to drop into duh's workflows.

The midi pkg will improve over time—stay tuned!

P.S. My personal favs are banks 68 and 79–81 for General MIDI.
