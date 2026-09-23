---
layout: single
title: "A Note About Syncing in duh"
date: 2026-09-21
categories: [meta]
---

Since duh is out, I’ve been asked how it manages to keep everything in sync.

The short answer is that it isn’t particularly difficult: every process that accesses the PCM bus is frame-locked—or, if you prefer, frame-synchronized. We don’t really care about the latency further down the line, in the various backends. But we must care about having for example the tui sampling the right pcm frames at 30FPS and also the housekeeping at its' own rate. The PCM-level decoupling is mainly done by the `pcm.Tap` abstraction and I believe it worth having a look at it. The TUI level decoupling is done the usual way in Go by running various `time.Timer` concurently.

Finally, my point is that, in the real world, being synchronized gives the viewer’s brain a chance to enjoy the experience by minimizing discrepancies rather than exaggerating them.
