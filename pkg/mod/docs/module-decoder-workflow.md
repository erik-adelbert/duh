# Module decoder workflow

The module decoder turns a supported S3M, IT, or experimental MPTM file into
interleaved, 32-bit little-endian float stereo PCM at 48 kHz. The tracker
resamples PCM voices and mixes in OPL audio when present.

```mermaid
flowchart TD
    Input["Module file or input stream"] --> Decode["mod.Decode"]
    Options["Decoder options: sequence, no loop"] --> Decode

    subgraph Init["Initialization"]
        Decode --> Buffer["Buffer input and unpack MMCMP if present"]
        Buffer --> Detect{"Detect module format"}
        Detect -->|S3M or IT| Load["Load module assets"]
        Detect -->|MPTM| Sequence["Load module assets for the selected sequence"]
        Load --> Tracker["Create tracker and set initial speed and tempo"]
        Sequence --> Tracker
    end

    Tracker --> Read["Decoder.Read"]
    Read --> Pending{"PCM bytes pending?"}
    Pending -->|Yes| Output["Copy bytes to caller"]
    Pending -->|No| Render["Tracker.Read: request up to 1024 frames"]

    subgraph Playback["Tracker rendering"]
        Render --> Schedule["Advance scheduler if the current tick is exhausted"]
        Schedule --> State{"Playback state"}
        State -->|New tick| Row["Fetch row on tick zero and apply song flow"]
        Row --> Voices["Allocate or release voices"]
        Voices --> Effects["Apply pitch and volume effects, then filter macros"]
        Effects --> Update["Update voice lifecycles, pitch and gains"]
        Update --> Chunk["Choose a frame chunk up to the tick boundary"]
        State -->|Current tick continues| Chunk
        Chunk --> Resample["Resample PCM voices"]
        Resample --> Mix["Mix PCM voices and render and add OPL voices if present"]
        Mix --> Advance["Consume the rendered frames"]
        Advance -->|More frames needed| Schedule
    end

    Advance -->|Block complete| Frames{"Any frames rendered?"}
    State -->|Song finished| Frames
    Frames -->|Yes| PCM["Encode float32 samples as little-endian PCM"]
    PCM --> Output
    Frames -->|No| End["Return bytes already copied, or EOF if none"]
    Output -->|Caller buffer not full| Pending
    Output -->|Caller buffer full| Done["Return bytes copied"]
```

`Decode` reads the whole input, but it does not scan or render the song to
calculate a duration. Audio is rendered on demand into reusable buffers.
`Read` accepts any buffer length, even one that splits a stereo frame, and
keeps leftover bytes for the next copy. `Close` releases playback state
without closing the input reader.

MPTM selects sequence zero by default; S3M and IT only accept sequence zero.
`NoLoop` stops playback when the same order, row and pattern-loop counters
recur, without disabling sample loops or finite pattern loops. MPTM uses this
termination behavior by default; some S3M and IT songs can otherwise loop
indefinitely.

Key implementation entry points:

- [`Decode`, `Read`, and `DecoderOptions`](../decoder.go)
- [S3M loader](../loader_s3m.go), [IT loader and MMCMP unpacking](../loader_it.go),
  and [MPTM sequence loader](../loader_mptm.go)
- [`NewTracker`](../tracker.go)
- [Tick scheduling and voice allocation](../system_controls.go)
- [Pitch and volume effects](../system_effect.go) and [filter macros](../mpt_filter_macro.go)
- [`Tracker.Read`, resampling and mixing](../system_audio.go)
