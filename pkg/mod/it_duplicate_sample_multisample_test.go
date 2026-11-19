package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTITDuplicateCutDelayedNote(t *testing.T) {
	for _, withVolume := range []bool{false, true} {
		fixture := func(dct, nna byte) []byte {
			b := testITDuplicateSampleFixture(dct, nna, nna, false, 0)
			rows := []byte{0x81, 3, 60, 1, 0}
			if withVolume {
				rows = append(rows, 0x81, 15, 60, 0, 32, 19, 0xd1, 0)
			} else {
				rows = append(rows, 0x81, 11, 60, 0, 19, 0xd1, 0)
			}
			rows = append(rows, 0x81, 3, 60, 1, 0)
			packed := append(rows, make([]byte, 61)...)
			binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
			copy(b[344:], packed)
			return b
		}
		ref, actual := referencePCMWithInterpolation(t, fixture(2, 1), "2")
		refBase, actualBase := referencePCMWithInterpolation(t, fixture(0, 0), "2")
		for _, ms := range []int{48, 68, 108} {
			want := channelRMS(ref, ms, ms+5, 0) / channelRMS(refBase, ms, ms+5, 0)
			got := channelRMS(actual, ms, ms+5, 0) / channelRMS(actualBase, ms, ms+5, 0)
			if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .05 {
				t.Errorf("volume=%v, %dms: OpenMPT ratio %.3f, tracker %.3f", withVolume, ms, want, got)
			}
		}
	}
}

func TestOpenMPTITDuplicateCutEnvelopeCarry(t *testing.T) {
	for _, instrument := range []byte{0, 1} {
		fixture := func(carry bool) []byte {
			b := testITDuplicateSampleFixture(2, 1, 1, false, instrument)
			first := len(b) - 2*554
			b[first+304] = 1
			if carry {
				b[first+304] |= 8
			}
			return b
		}
		ref, actual := referencePCMWithInterpolation(t, fixture(true), "2")
		refBase, actualBase := referencePCMWithInterpolation(t, fixture(false), "2")
		for _, ms := range []int{68, 108} {
			want := channelRMS(ref, ms, ms+5, 0)
			got := channelRMS(actual, ms, ms+5, 0)
			wantBase, gotBase := channelRMS(refBase, ms, ms+5, 0), channelRMS(actualBase, ms, ms+5, 0)
			if wantBase < 5e-5 && gotBase < 5e-5 && want < 5e-5 && got < 5e-5 {
				continue
			}
			if wantBase < 1e-8 || gotBase < 1e-8 || math.Abs(got/gotBase-want/wantBase) > .05 {
				t.Errorf("instrument %d, %dms: OpenMPT %g/%g, tracker %g/%g", instrument, ms, want, wantBase, got, gotBase)
			}
		}
	}
}

func TestITDuplicateCutRetainsZeroVolumeAfterVoiceEnds(t *testing.T) {
	inst := ITInstrument{NewNoteAction: 1, DuplicateCheck: 2, GlobalVolume: 128}
	for i := range inst.Samples {
		inst.Samples[i], inst.Notes[i] = 1, uint8(i)
	}
	inst.Samples[61] = 2
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 1, ITInstruments: []ITInstrument{inst},
		Samples: []SampleAsset{{Data: []float32{1, 1}, Volume: 64}, {Data: []float32{1, 1}, Volume: 64}},
	}, 48000, 4)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	w.PendingCmds[0] = ModCommand{Note: 61}
	VoiceAllocationSystem(w)
	voice := w.TrackCursors[0].ActiveVoice
	if w.VoiceAmps[voice].Volume != 0 {
		t.Fatal("duplicate cut did not silence the instrument-less note")
	}
	w.stopVoice(voice)
	w.PendingCmds[0] = ModCommand{Note: 62}
	VoiceAllocationSystem(w)
	voice = w.TrackCursors[0].ActiveVoice
	if w.VoiceSamples[voice].SampleID != 1 || w.VoiceAmps[voice].Volume != 0 {
		t.Fatal("instrument-less note did not retain volume after voice end")
	}
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	voice = w.TrackCursors[0].ActiveVoice
	if w.VoiceAmps[voice].Volume != 1 {
		t.Fatal("explicit instrument did not restore sample volume")
	}
}

func TestOpenMPTITDuplicateCutThenDifferentSample(t *testing.T) {
	fixture := func(dct, nna byte) []byte {
		b := testITDuplicateSampleFixture(dct, nna, nna, false, 0)
		first := len(b) - 2*554
		sample := len(b)
		b = append(b, b[256:336]...)
		binary.LittleEndian.PutUint16(b[36:], 2)
		binary.LittleEndian.PutUint32(b[206:], uint32(sample))
		binary.LittleEndian.PutUint32(b[210:], 336)
		b[first+65+61*2] = 2
		b[344+12] = 61
		b[344+13] = 0
		return b
	}
	ref, actual := referencePCMWithInterpolation(t, fixture(2, 1), "2")
	refBase, actualBase := referencePCMWithInterpolation(t, fixture(0, 0), "2")
	for _, ms := range []int{68, 108} {
		want := channelRMS(ref, ms, ms+5, 0) / channelRMS(refBase, ms, ms+5, 0)
		got := channelRMS(actual, ms, ms+5, 0) / channelRMS(actualBase, ms, ms+5, 0)
		if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .05 {
			t.Errorf("%dms: OpenMPT ratio %.3f, tracker %.3f", ms, want, got)
		}
	}
}
