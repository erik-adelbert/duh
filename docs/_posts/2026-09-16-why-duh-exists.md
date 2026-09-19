---
layout: single
title: "Why duh exists"
date: 2026-09-16
categories: [meta]
---

During the early 00s, I was studying at [Paris8 AI Lab](https://www.researchgate.net/publication/48445152_A_Parallel_General_Game_Player), with a focus on programming languages, while also being involved in the European electronic art scene with my group, [Les Virtualistes](https://youtu.be/sFNZyTzKAT8). I reckon it was a very creative time, and it was not uncommon for coders to explore fields outside their main discipline. At the time, computing resources were scarce, and every bit counted.

Today, I am craving to put a team together and produce a demo or installation once again. I have decided
to build the audio toolkit first. It revives a handful of legacy file formats while bringing current ones
together. Not only are some of these formats from the past, but parts of underlying model are too. In `duh`,
everything `pcm` is inherited from the plan9 `pcmconv` tool. These old-school formats and models have already
proven themselves and are key to our success.

`duh` is designed to support a wide range of use cases, but the one I think will interest most is its ability to
target **small platforms via `TinyGo` and browsers via [`WASM`]({{ "/vgmweb/" | relative_url }})**. With [OPL3 synthesis](https://www.youtube.com/watch?v=wr7E6oKZQhQ) (MIDI soon to be ready) and a MOD tracker (in the pipe)
that supports editing and professional studio capabilities it could prove highly useful for games right out of
the box. Ultimately, though, my goal is to make duh a tool for live performance.

> [JS/WASM demo]({{ "/vgmweb/" | relative_url }}) (first grab the VGZ file here: [Shop Hard by Televicious]({{ "/assets/vgz/shophard.vgz" | relative_url }}))

## Why the long route of Go?

`duh` - and the bigger picture it is part of - is a way for me to summarize and revisit my [practice](https://en.wikipedia.org/wiki/The_Practice_of_Programming). Along the way, I also want to deepen my understanding of [Tony Hoare](https://fr.wikipedia.org/wiki/Charles_Antony_Richard_Hoare)'s ideas and bring them into the project whenever I can. For example, I'd like to implement an [`ecs`](https://www.computerenhance.com/p/the-big-oops-anatomy-of-a-thirty) leveraging Go's strengths.

The [Go Team](https://go.dev/blog/io2013-chat), and [Commander Pike](https://en.wikipedia.org/wiki/Rob_Pike) in particular, have built not only a working implementation of CSP and composition, but also a standard library that forms an entire [programming model](https://en.wikipedia.org/wiki/Programming_model), ready to be extended in any direction we want. Compared to C, for example, writing concurrent audio processing code on top of Go `io` is a breeze. Effiency is a consequence.

But at the end of the day, I like the aesthetics of the language, and it is a great match for my [minimalist core style](https://github.com/erik-adelbert/aoc/blob/main/2025/1/aoc1.go).

## What's here

The repo is public, the README describes the goals, and the API is still moving. The first
tag lands when the core features I envision are done.

If you want to follow along, [watch the repo](https://github.com/erik-adelbert/duh).

## What's next

- Lock down the core interface

More soon.
