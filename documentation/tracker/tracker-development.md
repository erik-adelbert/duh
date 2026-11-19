# Tracker development and reference tests

This guide records implementation details, reference-test coverage and known
verification limits. For supported formats and loader restrictions, see the
[compatibility inventory](tracker-compatibility.md).

Run all tracker tests, including integration tests, with:

```sh
go test ./pkg/mod/...
```

Commands in this guide are run from the repository root.

## Implementation and verification notes

Experimental MPTM extension-boundary location, extension-field parsing and
serialized-sequence parsing accept only limited layouts and fields and do not
establish playback compatibility.

The tracker has an internal, experimental OPL3 voice and mixer path used by
synthetic tests and the MPTM loader. PCM/OPL
transitions with notes and experimental instrument-only sample and patch
switches are implemented, but instrument envelope behavior on such switches needs
further verification. On OPL-to-PCM switches, the old OPL voice is cut rather
than left sounding as observed in the tested OpenMPT fixtures.
An isolated experimental OPL envelope-end timing option is tested against
OpenMPT with the target's saved behavior flags. Clearing the envelope-position
handling flag changes the reference timing, so that option is not a general
MPTM profile. The distinct version-specific retained OPL channel-state quirk
is implemented for the tested note and portamento transitions.
The experimental modern-tempo clock supports song and pattern swing, with
pattern swing overriding song swing. Synthetic OpenMPT comparisons cover
normalization, pattern transitions and breaks, tempo and speed changes, delays,
fractional sequence defaults, later-row timing markers and read chunk sizes.
SCx tick markers also cover simultaneous later-row speed and tempo changes.
A separate experimental parser
reads the target song's instrument and song extension fields and serialized
sequences. Saved playback flags can be decoded for a limited set of OpenMPT
behavior indices, but version- and song-dependent flag upgrades and playback
compatibility are incomplete; an isolated projection handles the target's
pitch-mode and OPL envelope flag upgrades without authorizing playback. A
separate experimental loop profile handles saved loop flags for OpenMPT
1.26 and later, including the position-sensitive loop/jump flag. A separate
panbrello projection handles waveform, sample-and-hold,
hold and pan-override flags. An isolated duplicate-sample
projection covers cross-instrument checks in OpenMPT 1.26–1.28. The loader
selects the applicable loop, panbrello and duplicate-sample projections; it
does not establish general MPTM compatibility.

The loader supports serialized plug-in parameter-control pattern overrides by
bypassing them with a warning. Pattern signatures, tempo swing and colors are
also parsed; other serialized pattern commands and fields remain unsupported.

An optional structural audit of the Triangel Island target
(`MPTECS_MPTM_TARGET` when running tests) parses all its patterns, instruments,
sample streams and serialized sequence without claiming playback compatibility.
It contains 16 OPL patches and two compressed stereo PCM samples. Its modern
tempo, mix level and preamp, OPL transitions and
other enabled playback behaviors still need end-to-end compatibility work.
An isolated experimental extension path preserves instrument fadeout precision
and is checked against OpenMPT. A separate experimental compatible-mix profile
applies PCM sample-preamp and OPL plugin-volume changes independently, with
absolute scaling checked against OpenMPT for synthetic single and simultaneous
PCM/OPL voices. The profile applies global volume after mixing, including OPL;
synthetic initial and changing global volumes are checked against OpenMPT with
volume ramping disabled. OPL saturation has been checked for synthetic
OPL-only mixes of up to 18 voices, including hard panning and reduced global
volume. Full-song mixing and global-volume ramping remain unvalidated. The
loader uses the fadeout and mix projections for its supported subset.
PCM voice gain and panning changes use a one-millisecond, sample-rate-scaled
ramp that continues across render buffers; panbrello, volume slides, surround,
envelope and nonzero instrument fadeout transitions may use a longer
tick-length ramp. Note cuts
and naturally ending samples still stop immediately; OPL and global-volume
changes use separate mixing paths.

Experimental sequence-default resolution preserves fractional tempo and uses
legacy song defaults for missing or zero sequence tempo and speed. Synthetic
OpenMPT tests cover no-repeat playback with legacy and sequence restart orders,
including out-of-range position jumps. The loader selects one serialized
sequence at a time; general subsong selection and repeat playback are not supported.

IT song and channel volume header values initialize per-player playback state;
they remain separate from the loaded asset values. Mxx sets channel volume
from 0 to 64; continued past notes retain their channel volume at the time
they become past notes. Vxx sets song-wide volume from 0 to 128. Values above
the valid ranges are ignored, except that legacy ModPlug Vxx values above 128
are treated as 128.

## OpenMPT reference tests

The tracker tests compare synthetic playback cases with `openmpt123` when
it is available in `PATH`; those comparisons are skipped when it is absent.
Run them with `go test ./pkg/mod -run TestOpenMPT`.

## Evidence and priorities

Reference tests exercise selected synthetic cases against `openmpt123`; they
are not exhaustive across arguments, versions, flags, sample types or effect
combinations. Examples include
[effect mapping](../../pkg/mod/loader_it_test.go),
[row delay](../../pkg/mod/mpt_pattern_delay_test.go),
[fine tuning](../../pkg/mod/mpt_microtuning_test.go),
[envelope controls](../../pkg/mod/mpt_envelope_control_test.go),
[resampling](../../pkg/mod/mpt_song_resampling_test.go), and
[decoder behavior](../../pkg/mod/decoder_test.go). Reference comparisons skip
when `openmpt123` is unavailable.

Prioritize by measuring a local corpus, not by guessing prevalence:

1. Make unsupported S3M features visible instead of silently bypassing them.
2. Inventory all detectable blockers per module, including combinations and
   profile/extension restrictions; report when a scan cannot be completed.
3. Implement and compare complete effect families, starting with general
   MPTM SEx row delay and its interactions, rather than one argument per song.
4. Expand reference and chunking tests across relevant profiles and
   representative files; track accepted but unverified behavior separately.
