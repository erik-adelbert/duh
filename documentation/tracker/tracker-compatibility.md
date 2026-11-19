# Tracker compatibility inventory

This is an inventory of the current `pkg/mod` decoder, not a format
conformance claim. Loader acceptance does not guarantee OpenMPT-equivalent
playback. In particular, MPTM support is experimental and coverage of saved
playback flags, older versions, interactions and full-song mixing is incomplete.
`mod` is the package name; ProTracker MOD and XM are not supported.

The tables describe current implementation and known restrictions. **Partial**
means some behavior is implemented or compared with OpenMPT, not that every
argument, profile or interaction is verified. **Rejected** means the loader
reports an error for the specified feature. **Bypassed** means it can be
accepted without performing the feature; warnings are noted where available.
Malformed data can be rejected independently of these tables. For implementation
and reference-test details, see the
[tracker development guide](tracker-development.md).

The duration estimator follows this decoder's timing and repeated-state
behavior without mixing audio. It estimates the selected sequence's first
pass, not an unlimited repeat or every subsong. Results can differ from other
players, and particularly complex playback returns an error instead of an
estimate.

## Formats, samples and instruments

| Area | S3M | IT | MPTM |
|---|---|---|---|
| Input and selection | S3M, including MMCMP-wrapped S3M, sequence 0 | IT and MMCMP-wrapped IT, sequence 0 | Experimental MPTM, including MMCMP-wrapped MPTM, zero-based serialized sequence selection |
| Orders and loops | Partial: skip markers retained for order jumps; basic pattern loops and row delays; some order loops can play indefinitely | Skip markers retained for order jumps; some order loops can play indefinitely | Stops on a repeated order/row/pattern-loop state; individual subsong selection and general repeat playback are not supported |
| Timing signatures | Classic tempo | Classic tempo | Song and pattern rows-per-beat control modern tempo; song and pattern swing supported experimentally, with pattern swing overriding song swing; song swing with classic tempo rejected; rows-per-measure has no audio effect |
| Samples | Partial: mono/stereo PCM, 8/16-bit, forward loops; packed samples rejected | Mono/stereo PCM, uncompressed or IT 2.14/2.15 compressed; forward and ping-pong normal loops and instrument-mode sustain loops | IT PCM plus OPL patches; forward and restricted ping-pong sustain loops in instrument mode on 1.26+ profiles, with optional normal loops |
| Instruments | Sample-based; AdLib instruments skipped with a warning | Modern instrument headers, note mapping, NNA, duplicate checks, volume/pan/pitch or filter envelopes, static filter and auto-vibrato; old instrument headers and legacy random auto-vibrato rejected | Partial: IT-based instruments, selected duplicate checks and OPL behavior; plug-in instruments bypassed with a warning |
| Resampling | Player default linear | Player default linear | Nearest, linear, cubic and sinc; cubic/sinc with ping-pong loops rejected |
| External output | Not implemented | MIDI macro subset below; no general MIDI output | Plug-in assignments and PC/PCS automation bypassed with a warning; MIDI output unsupported |

S3M is particularly important to treat cautiously: unknown effect commands
map to no effect, and unsupported special commands are not generally rejected.
A successful S3M load is
therefore not evidence that every command was played.

S3M sample-note pitch uses integer periods for ST3 profiles and integer
frequencies for Impulse Tracker and recent Schism profiles. The header master
volume affects PCM mixing. For files older than ST3.20, SAx is interpreted as
SoundBlaster stereo control rather than a high sample offset; GUS files
ignore that command, following OpenMPT's sample-address detection.
Ordinary and fine pitch slides, tone portamento, glissando, arpeggio and
vibrato memory have focused comparisons across ST3, IT and Schism profiles;
other pitch-effect combinations and limits remain partial.

The tracker uses OpenMPT as a playback reference and includes material such as
its fixed random modulation table. See the [tracker notice](../../pkg/mod/NOTICE)
and [OpenMPT license](../../pkg/mod/openmpt-LICENSE) for attribution and
distribution requirements.

For IT and MPTM header channel panning, values above 64 other than surround
(100) leave the channel centered. A channel setting of `FF` uses centered,
unmuted defaults with channel volume 64, ignoring its stored channel volume.

## Effect-column commands

Commands are written as tracker letters; `x` denotes a hexadecimal digit.
Unless stated otherwise, IT and MPTM entries are partial: the implementation
handles the command but specific profile and combination gates still apply.

| Effect | S3M | IT | MPTM |
|---|---|---|---|
| A speed, B order jump, C pattern break | Partial | Partial | Partial; sequence and loop limits above |
| D volume slide | Partial | Partial | Partial; note fade with Dxx requires instrument mode and no OPL samples anywhere in the module |
| E/F pitch slides, G tone portamento, H vibrato, J arpeggio | Partial | Partial | Partial; legacy linear-period restrictions below |
| I tremor, N channel-volume slide, P pan slide, R tremolo, W global-volume slide | Partial | Partial | Partial, OpenMPT 1.26+ |
| U fine vibrato | Partial | Partial | Partial, OpenMPT 1.26+ |
| K vibrato + volume slide | Partial | Partial | Partial, OpenMPT 1.26+ |
| L tone portamento + volume slide | Partial | Partial | Partial, OpenMPT 1.26+ |
| M channel volume | Partial | Partial | Partial |
| V global volume | Partial | Partial | Partial |
| O sample offset | Partial | Partial | Partial |
| Q retrigger | Partial | Partial, including volume changes | Partial: Qxx timing and volume changes, ordinary notes, instruments, direct volume and volume slides; selected special-note cases and separate-cell Qxx on delayed rows. Other combinations rejected; can warn about ended short samples |
| T tempo | Partial, including tempo slides | Partial, including tempo slides | Partial; tempo slides require 1.26+; VBlank and clamp flags affect behavior |
| X panning | Partial | Partial | Partial |
| Y panbrello | Partial | Partial | Partial |
| Z macros | Bypassed: unmapped | Partial: supported cutoff and resonance macros; legacy ModPlug resonance macros and unsupported used macros rejected | Partial: supported cutoff/resonance macros; other used MIDI and plug-in macros rejected |
| `\xx` smooth macros | Not implemented | Partial: supported cutoff and resonance macros; legacy ModPlug profiles and pattern-delay combinations rejected | Partial: supported filter macros on compatible 1.26+ profiles; pattern delays and old filter swing with variation rejected |
| `^xx` fine tuning | Bypassed: unmapped | Rejected | Partial; instrument pitch-wheel depth and selected transitions compared with OpenMPT |
| `]xy` note delay and cut | Bypassed: unmapped | Rejected | Partial: ordinary notes, instruments and direct volume; special notes, volume effects, instrument-only cells with `]x0` and pattern-delay combinations rejected |
| `[` extended parameter, `_` smooth fine tuning and other unmapped commands | Bypassed: unmapped | Rejected | Rejected |

### Sxx special commands

| Command | S3M | IT | MPTM |
|---|---|---|---|
| S00 memory | Partial | Partial | Partial, subject to S6x/SEx delay restriction |
| S01–S0F filter | Not generally implemented | Accepted and ignored | Rejected |
| S1x glissando | Partial; tested tone-portamento and selected transitions | Partial | 1.26+ and frequency-based pitch only |
| S2x finetune | Partial | Partial | Partial |
| S3x–S5x waveform selection | Partial; includes OpenMPT's fixed random waveform | Partial; legacy random waveform rejected | 1.26+; legacy random waveform rejected |
| S6x fine pattern delay | Partial | Rejected | Partial: ordinary notes with optional instruments and instrument-only cells with no effect or a delay effect, plus Dxx, Exx, Fxx, Gxx, Hxx, Kxx, Lxx, Nxx, Pxx, Qxx, Uxx, Wxx, Txx, S1x and `^xx` on other cells; special notes, volume-column commands, other interactions and S00 memory on delay channels rejected |
| S70–S7C instrument controls | Not generally implemented | Partial: past notes, NNA and envelope controls | Partial: S70, S73–S7C; S71/S72 rejected |
| S7D–S7F | Not generally implemented | Rejected | Rejected |
| S8x panning, S90/S91 surround | Partial | Partial | Partial |
| S9E forward, S9F reverse | Not generally implemented | Rejected | S9F partial; S9E rejected |
| SAx high offset | Partial; older ST3 files use stereo control instead | Partial | Partial |
| SBx loop, SCx cut, SDx note delay | Partial | Partial | Partial |
| SEx row delay | Partial | Partial: ordinary notes, instrument selections, and instrument-mode note-off without an instrument; no volume-column commands, other effects, or S00 memory on delay channels | Partial: all values, with the same interaction restrictions as S6x |
| SFx macro selection | Not generally implemented | Partial, for supported cutoff and resonance macros | Partial, 1.26+; supported default and embedded filter macros only |

## IT and MPTM volume column

These are the serialized numeric values, not the displayed tracker syntax.
Same-cell interactions and saved-profile checks apply after parsing.

| Values | Behavior | IT | MPTM |
|---|---|---|---|
| 0–64 | Set volume | Partial | Partial |
| 65–84 | Fine volume slide | Partial | Partial |
| 85–104 | Volume slide | Partial | Partial |
| 105–124 | Pitch slide | Partial | Partial |
| 125–127 | Unsupported | Rejected | Rejected |
| 128–192 | Set panning | Partial | Partial |
| 193–202 | Tone portamento | Partial | Partial |
| 203–212 | Vibrato | Partial | 203 only; 204–212 rejected |
| 213–223 | Unsupported | Rejected | Rejected |
| 224–232 | Cue-point sample offset | Rejected | Partial: `o1`–`o9` on ordinary notes without an effect, 1.26+ profiles |
| 233–255 | Unsupported | Rejected | Rejected |

Volume-column pitch slides combined with effect-column portamento or note
delay, vibrato combined with an effect other than S3x–S5x, and volume-column
tone portamento combined with Lxx are rejected. MPTM cue offsets reject `o0`
and Oxx anywhere on the same channel because offset memory is not shared yet.

## MPTM song and instrument extensions

| Feature | Current status |
|---|---|
| Clock and tempo | Classic tempo or modern tempo with experimental song and pattern swing; integer and fractional default tempo. Song swing with classic tempo rejected |
| Mix and channels | Mix levels 3 and 4; sample preamp, plug-in volume and extended channel settings; up to 127 channels |
| Playback flags | Selected known flags projected with version-dependent restrictions; unsupported bits rejected. This is not an audited compatibility profile |
| Instrument properties | Extended fadeout, global volume, pitch-wheel depth, cutoff/resonance swing, filter modes and resampling implemented with restrictions |
| Instrument panning | Extended values from 0 to 256, including fine positions |
| Release nodes | Volume release node for OpenMPT 1.28.00.43+; active pan and pitch release nodes rejected |
| Instrument ramp-up, pitch-to-tempo lock, MIDI channel/program/bank | Accepted only when unset or zero |
| Plug-in properties | Plug-in routing and automation bypassed with warning; velocity and volume handling parsed but not executed |
| Other fields and layouts | Unknown song/instrument fields and unsupported serialized layouts rejected; metadata may be parsed without affecting playback |

Additional MPTM gates include pre-1.26 restrictions on I/K/L/N/P/R/U/W,
waveform selection and tempo slides; pre-1.24 legacy linear-period pitch
effects and related envelope/auto-vibrato combinations are rejected. Filter
macros, OPL flags, duplicate actions, note fade and note-off also have
profile- or combination-specific restrictions. See
[loader_mptm.go](../pkg/mod/loader_mptm.go),
[loader_it.go](../pkg/mod/loader_it.go), and the
[detailed IT and MPTM behavior](#detailed-it-and-mptm-behavior)
for the detailed checks.

### MPTM saved playback flags

The loader recognizes serialized bits 0–50, named in
[mpt_play_behaviour.go](../pkg/mod/mpt_play_behaviour.go). Recognition is
**not** a guarantee that every bit's behavior or every OpenMPT version upgrade
is implemented. Selected projections cover, for example, tempo, slides,
looping, envelopes, filtering, panbrello and retriggering. Their tests cover
specific profiles, not the full cross-product.

| Additional bit | Recognized behavior and current limit |
|---|---|
| 87 | Multi-sample instrument behavior; selected instrument-only/portamento cases implemented |
| 88 | Row delay with note delay recognized; delay validation still rejects those row combinations |
| 99 | Flexible OPL note-off; initial reference coverage |
| 100 | Old-effects note-off with instrument: affected combination rejected |
| 102 | Channel-pan restoration; initial normal-note reference coverage |
| 103 | Pattern loop with jumps; projected by the loop profile |
| 104 | Duplicate-check behavior; restricted supported duplicate actions |
| 105 | OPL new-note actions; initial reference coverage |
| 108 | OPL real retrigger; rejected when OPL samples are present |
| 109 | OPL envelope-end no-reset; selected stopped-channel transitions covered |
| 115 | Experimental IT pitch-pan separation, including selected panning, portamento and NNA comparisons; unavailable saved versions with nonzero separation rejected |
| 119 | Filter reset on portamento; applied when available for the saved version |
| 120 | Initial note memory; applied when available for the saved version |
| 122 | Sustain-loop selection during portamento, including key-off, instrument changes and sample swaps |
| 123 | Partial: explicit notes mapped to empty slots ignore the entire cell. Pending instrument selections, instrumentless notes in files with ignored empty-map cells, and ignored empty-map cells with plug-in routing are rejected |
| 124 | Offset commands with an instrument number and remembered note |
| 127 | Portamento-memory ordering on supported rows; simultaneous volume pitch and effect-column pitch slides remain rejected |
| 132 | Envelope carry after note-off, including Compatible Gxx portamento |
| 134 | Partial: SCx with ordinary notes and volume-column portamento; files combining SCx with instrument-only cells or Qxx retrigger are rejected, as are unsupported pattern-delay combinations |
| 135 | Volume-column slide memory isolation; applied when available for the saved version |
| 136 | Filter envelope stopped at its initial position; selected S7B/S7C comparisons |
| Other set bits above 50 | Rejected |

If playback flags are absent, only a limited OpenMPT 1.20–1.25 legacy
profile is accepted. See [mpt_behaviour_upgrade.go](../pkg/mod/mpt_behaviour_upgrade.go)
for the deliberately limited version-upgrade projection.

## Detailed IT and MPTM behavior

### IT

The decoder can load a limited subset of IT:

```sh
go run ./cmd/moddec /path/to/song.it | ffplay -nodisp -autoexit -f f32le -sample_rate 48000 -ch_layout stereo -
```

IT sample mixing uses the header mixing volume, with legacy ModPlug files using
their channel-count-dependent preamp. The float output is not limited and can
exceed full scale.

IT files may also be wrapped in an MMCMP container. IT order skip markers are
preserved for order jumps. IT support is incomplete:
only mono or stereo PCM (uncompressed or IT 2.14/2.15 compressed), forward and ping-pong sample loops, volume and
panning columns, volume-column fine and regular volume slides, pitch slides up and down,
vibrato and tone portamento, and the A, B, C, D (volume slide), E/F (pitch slides),
G (tone portamento), M (channel volume), N (channel-volume slide),
H (vibrato), I (tremor), J (arpeggio), K (vibrato and volume slide), L (tone portamento and volume slide), O (sample offset), P (panning slide), Q (retrigger and volume change), R (tremolo),
S01–S0F (ignored filter commands), S1x (glissando), S2x (finetune), S3x–S5x (vibrato, tremolo and panbrello waveforms), S70–S7C (past-note, new-note-action and envelope controls), S8x (panning), S90/S91 (surround), SBx (pattern loop), SCx (note cut), SDx (note delay),
SAx (high sample offset), T (tempo and tempo slides), U (fine vibrato), V (global volume), X (set panning) and Y (panbrello) effects are
supported. Basic instrument mappings,
volume, panning, pitch and low-pass filter envelopes, pitch-pan separation, random volume/pan
variation, sample auto-vibrato, static instrument low-pass filters, new-note actions and
duplicate-note checks (note, sample and instrument) with cut, off or fade, and
instrument-only rows are supported;
legacy ModPlug random auto-vibrato and random S3x–S5x waveforms,
other loop modes and other effects are rejected. Mixing, resampling, and tracker
compatibility are not yet equivalent to OpenMPT. XM is not supported.
IT and MPTM volume envelopes that reach a non-looping, nonzero endpoint start
the instrument fadeout; a zero fadeout leaves the voice sounding.
Out-of-range instrument numbers on ordinary notes are remembered without
triggering a new note; unsupported combinations are still rejected.
Volume-column pitch slides combined with effect-column portamento or note delay,
volume-column vibrato with an effect command other than Gxx, Jxx, S3x–S5x or S77, and volume-column
tone portamento with Lxx are rejected. Random IT waveforms use the player's
random generator, so they will not reproduce OpenMPT's exact random sequence.
IT global-volume slides (Wxx) are supported, including fine slides and effect memory.
IT SFx selection and Zxx cutoff and resonance macros are supported, including
compatible fixed and parametered embedded filter macros. Empty macros do
nothing; other used MIDI macros and legacy ModPlug resonance macros are rejected.
On non-legacy IT profiles, `\xx` uses the same filter macros as `Zxx` but
interpolates toward the target on each tick. Combinations with row delay are
not supported.
In IT instrument mode, a forward or ping-pong sustain loop takes precedence until
note-off, then playback uses the normal loop or continues through the sample tail.
Loop direction flags do not enable a loop on their own.
The IT note-fade encoding is supported in instrument mode with Gxx tone portamento, Hxx vibrato,
direct volume with or without Exx pitch slide, ordinary volume-column slides,
and fine volume-column slides with Axx speed or direct Txx tempo commands,
or other explicitly supported combinations; fade smoothing can differ from OpenMPT.

### MPTM

The decoder also attempts experimental MPTM playback without an extra flag:

```sh
go run ./cmd/moddec "/path/to/song.mptm" | ffplay -nodisp -autoexit -f f32le -sample_rate 48000 -ch_layout stereo -
```

The loader supports a narrow subset, not general MPTM compatibility. Saved
versions and recognized playback flag combinations are not gated by an audited
profile; unsupported behavior bits, layouts, extension fields and effects may
still be rejected. It supports classic tempo or modern tempo with experimental
song and pattern swing, fractional default tempo, mix level 3 or 4 and sequence
selection. Song swing with classic tempo remains rejected.
The OPL real-retrigger flag is accepted only when no OPL samples are present.
Nonzero restart positions before the first end marker are supported for
no-repeat playback; general subsong selection and repeat playback are not supported.
Up to 127 pattern channels are supported, including the extended channel
panning, surround, mute and volume settings for channels 65–127.
When the sample-preamp extension is absent, its value comes from the IT header.
Playback can sound wrong even when accepted: many saved
behaviors, version-dependent upgrades, nonlinear slides and full-song mixing
are not yet verified, and duh's
Nuked OPL3-fast-derived synthesizer differs from OpenMPT's Opal. `LoadIT` continues to reject
MPTM; applications can use `LoadMPTM` for this experimental subset.
Files saved with OpenMPT 1.20 through 1.25 without playback flags use a
limited legacy profile. Before 1.24, pitch effects and modulation with linear
slides are rejected because legacy period-based pitch is not implemented.
Instrument cutoff and resonance variation are supported for PCM and OPL notes,
including filter envelopes. Without compatible filter behavior, resonance
variation requires an instrument cutoff below 127 and a nonzero explicit resonance.
The random sequence differs from OpenMPT's. Extended
integer song tempo (`..TD`) is supported within 32–1000 BPM. The default MIDI
macro configuration is supported experimentally: SF0 selects cutoff for
Z00–Z7F, SF1–SFF select empty macros, Z80–Z8F set resonance, and Z90–ZFF
are empty. SFx selection and fixed Z80–ZFF macros require OpenMPT 1.26 or
later; Z80–Z8F also require the compatible filter behavior. Embedded macro
configurations saved with OpenMPT 1.26 or later support parametered cutoff
(`F0F000z`) and resonance (`F0F001z`) and fixed cutoff or resonance
(`F0F000xx` / `F0F001xx`, with a 7-bit value). When a parametered Zxx or
`\xx` occurs, all SFx slots must be empty or supported filter forms; fixed
slots are checked when used. Resonance requires compatible filter behavior.
Other used macros, including MIDI and plug-in macros, are rejected rather than
executed.
Smooth filter macros (`\xx`) use the same configuration and interpolate on
each tick. They require a 1.26+ compatible filter profile; pattern-delay
combinations and old filter-swing behavior with filter variation are rejected.
Song-wide and per-instrument resampling support nearest-neighbor, linear,
experimental cubic and 8-tap anti-aliased sinc modes. Nearest-neighbor supports
ping-pong loops; cubic and sinc with ping-pong loops and other explicit
resampling modes are rejected.
The existing linear resampler remains the player default. Instrument
volume ramp-up settings are accepted only when zero (the default).
Extended instrument global volume overrides are supported; values above 64
are clamped, while values above 65535 are rejected.
Extended MIDI channel, program and bank settings are accepted only when zero;
MIDI output is not supported.
Extended instrument panning supports the full 0–256 range; out-of-range values
are rejected.
Extended instrument filter modes support unchanged, low-pass and high-pass.
Volume-envelope release nodes are supported for OpenMPT 1.28.00.43 and later;
pitch- and pan-envelope release nodes are accepted only when unset.
Integer and fractional pitch-to-tempo lock settings are accepted only when
zero; active pitch-to-tempo locking is not supported.
Forward and ping-pong sustain loops are supported experimentally in instrument-mode
MPTM saved with OpenMPT 1.26 or later. On note-off playback switches to the normal
sample loop if present, or continues through the sample tail. Ping-pong sustain
loops must span at least two samples and use nearest-neighbor or linear resampling;
linear additionally requires IT ping-pong mode. Broader effect interactions are
not yet verified.
Saved IT pitch-pan separation behavior (bit 115) is supported experimentally
for compatible saved versions, including selected panning, portamento, NNA and
note transitions compared with OpenMPT.
Instrument plug-in assignments and serialized plug-in parameter automation are
bypassed with a warning; plug-in effects and instruments are not supported, so
affected songs may sound different or silent.
Plug-in velocity and volume handling settings are parsed but bypassed.
Song and pattern rows-per-beat settings control modern tempo; pattern tempo
swing is supported experimentally. Rows-per-measure and sample cue points are
parsed. Cue offsets `o1`–`o9` are supported experimentally on ordinary notes
without an effect in 1.26+ profiles; `o0` and Oxx on the same channel are
rejected because their shared offset memory is not implemented.
MPTM instrument-only rows are supported experimentally, including selected
saved retrigger-flag and multi-sample portamento interactions. S6x tick delay
and SEx row delay are supported experimentally on otherwise empty rows or rows with Dxx,
Exx, Fxx, Gxx, Hxx, Kxx, Lxx, Nxx, Pxx, Qxx, Uxx, Wxx, Txx, S1x or `^xx`
effects. Ordinary notes, with or without an instrument, are supported when
their cell has no effect or only a delay command; instrument-only cells have
the same restriction. Special notes, volume-column commands, other delay
interactions and S00 memory on a delay channel are rejected.
Sample-mode note-off can differ from OpenMPT; instrument-only transitions after
that note-off have not been verified.
MPTM `^xx` fine tuning has initial OpenMPT comparisons for instrument pitch
wheel depth and note and instrument transitions. Extended-parameter and smooth
fine-tuning commands remain unsupported.
Note-off with an instrument and old effects is rejected.
MPTM playback stops when it revisits the same order, row and pattern-loop
state, rather than repeating an infinite song loop.
S2x finetune has PCM and OPL reference coverage, including instrument note
mapping, but broader interactions remain experimental. When the IT voice pool
is full, new notes reclaim a low-volume background voice; selection is an
approximation of OpenMPT's priority. Qxx retriggers and
volume changes have PCM reference coverage for all parameter values and selected
OPL and envelope comparisons. Qxx, including Q00, supports ordinary notes,
instrument numbers, direct volume and volume slides, and selected note-cut and
instrument-mode note-off cases. Note fade, other volume effects and some
special-note combinations are rejected. Q00 recalls the preceding Qxx; Q00 with
special notes remains rejected. Qxx cannot restart a PCM sample that has already
ended when the short-sample retrigger
compatibility flag is disabled; the loader warns when this may occur. Qxx on a
pattern-delay row is supported on a separate cell without a note, instrument or
volume command.
MPTM `]xy` note delay and cut is supported experimentally, with direct volume
and selected sample and instrument mode timing comparisons. Special notes,
volume effects, instrument-only cells with `]x0` and pattern-delay combinations
with `]xy` are rejected.
MPTM note fade is supported in instrument mode, including with E04, volume
command 76 (fine volume down by one), or Dxx volume slides. Fade with Dxx is
rejected if the module contains any OPL sample, even if it is unused. Other
combined commands and sample-mode fade remain unsupported. Flexible OPL
note-off has initial reference tests.
MPTM volume-column pitch slides are supported in the accepted profile;
combinations with effect-column portamento or note delay are rejected.
Volume-column command 203 (vibrato with remembered depth) is supported
experimentally, including saved volume-column memory behavior. Combining it
with effect commands other than S3x–S5x is rejected.
MPTM Ixx (tremor), Kxx (vibrato and volume slide), Lxx (tone portamento and
volume slide), Nxx (channel-volume slide), Pxx (panning slide), Rxx (tremolo),
Uxx (fine vibrato), and Wxx (global-volume slide) are supported experimentally for files saved with
OpenMPT 1.26 or later. PCM reference tests cover selected old-effects,
pitch-mode and saved-flag combinations, including shared Wxx and portamento
memory and speed-one pitch slides; OPL tests cover voice state, not bit-exact
audio. Ixx and Rxx also have PCM and relative OPL output comparisons across
selected saved profiles; older OpenMPT I11 behavior is preserved. Volume-column
tone portamento with Lxx and these eight effects in
pre-1.26 profiles are rejected. S3x–S5x waveform selection is supported
experimentally for OpenMPT 1.26 and later, with PCM reference comparisons and
OPL voice-state checks. Modern random waveforms use duh's random generator;
legacy-profile random S3x–S5x waveforms are rejected. Panbrello's
sample-and-hold and retained-offset flags are handled independently.
S1x glissando is supported experimentally for OpenMPT 1.26 and later when
the effective pitch mode uses frequencies, with PCM and relative OPL pitch
comparisons. Period-based pitch profiles are rejected for S1x; older nonlinear
profiles may be upgraded to period-based pitch. S70 cuts past notes on the
current channel; S73–S76 select the new-note action, and S77/S78 and S79/S7A
control the current voice's volume and pan envelopes. These controls have
selected OpenMPT comparisons; S71/S72 and S7B–S7F remain unsupported in MPTM.
Saved envelope-position behavior affects envelope disablement. S9F reverse playback is supported experimentally,
including S00 memory. Sinc resampling
with ping-pong loops remains rejected. Txx tempo
slides and per-channel memory are supported experimentally for OpenMPT 1.26
and later, with classic
and modern clock reference tests. The saved tempo-clamp and VBlank timing flags
are honored; VBlank timing makes Txx set speed instead. Pre-1.26 tempo slides
and tempo commands with the VBlank flag are rejected. Other currently
unsupported MPTM effects remain rejected.
Channel-pan restoration for instrument and sample defaults has initial
normal-note reference tests.
The newer duplicate-check flag is projected for sample checks with cut.
Duplicate-note checks with cut are supported for PCM and OPL instruments
without plug-in routing in profiles saved with OpenMPT 1.26 or later.
Duplicate-instrument checks with note-off are supported for PCM instruments
without plug-in routing in profiles saved with OpenMPT 1.26 or later; other
duplicate-note and duplicate-instrument combinations remain unsupported.
Enabled OPL new-note actions have initial OpenMPT comparisons; other
compatibility combinations need broader validation.
OPL envelope-end no-reset and its short-lived effect on note-only portamento
have reference coverage; other interactions with a stopped channel are not yet
fully verified.

The matrix is a snapshot, not an automatic scanner. Update it when loader
gates, runtime behavior or reference coverage change.
