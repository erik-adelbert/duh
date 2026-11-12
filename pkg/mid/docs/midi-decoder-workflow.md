# MIDI decoder workflow

The MIDI decoder turns a supported music file into interleaved, signed 16-bit
little-endian stereo PCM. It uses an OPL synthesizer to render the audio.

```mermaid
flowchart TD
    Input["Music file or input stream"] --> Decode["mid.Decode"]
    Options["Decoder options: bank, cards, 4-op, loop"] --> Decode

    subgraph Init["Initialization"]
        Decode --> Buffer["Buffer input"]
        Buffer --> Reset["Create synth and MIDI player"]
        Reset --> Open["Detect format and load tracks"]
        Open --> Scan["Scan one non-looping pass to calculate output size"]
        Scan --> ResetPlayback["Reset player with requested loop setting"]
    end

    ResetPlayback --> Read["Decoder.Read"]
    Read --> Next["nextFrames: determine next audio chunk"]
    Next --> Tick["MIDIPlayer.Tick"]

    subgraph Sequence["Sequencing"]
        Tick --> Process["Process ready events across tracks"]
        Process --> Event{"Event type"}
        Event -->|MIDI| MIDI["handleMIDIEvent"]
        Event -->|Meta| Meta["Tempo, track end, loop markers, device"]
        Event -->|SysEx| SysEx["SysEx handling"]
        Event -->|OPL event| Direct["Direct OPL register write"]
        Process --> Delay["Calculate wait until next events"]
    end

    subgraph Synth["Synthesizer"]
        MIDI --> Dispatch{"MIDI message"}
        Dispatch -->|Note on| On["Select instrument and OPL channels"]
        On --> Prepare["Prepare or share voices"]
        Prepare --> Update["Update note and OPL registers"]
        Dispatch -->|Note off| Off["Release or sustain note"]
        Off --> Update
        Dispatch -->|Controller| Control["Update controllers and held notes"]
        Dispatch -->|Pitch bend| Pitch["Update pitch"]
        Pitch --> Update
        Dispatch -->|Program / pressure| State["Update channel or note state"]
        Tick --> Advance["When playing: advance voice timers, glide, vibrato and arpeggio"]
    end

    Delay --> Render["Render samples from OPL cards"]
    Update --> Render
    Control --> Render
    Advance --> Render
    SysEx --> Render
    Direct --> Render
    Render --> Mix["Mix cards and clamp samples"]
    Mix --> PCM["Encode signed 16-bit stereo PCM"]
    PCM --> Output["Copy bytes to caller"]
    Output -->|More data requested| Read
```

`Decode` scans the sequence to calculate its size, then resets the player;
it does not render PCM during that scan. Audio is rendered as the caller reads.
At the end of a non-looping pass, the decoder renders a one-second tail before
returning EOF. Looping follows the sequencer's loop markers; reported size and
duration still describe one non-looping pass.

Key implementation entry points:

- [`Decode`, `nextFrames`, and `Read`](../decoder.go)
- [`MIDIPlayer.Tick`](../player.go)
- [`ProcessEvents` and `HandleEvent`](../sequencer.go)
- [`handleMIDIEvent` and MIDI event handlers](../events.go)
- [Voice allocation and synth updates](../synth.go)
