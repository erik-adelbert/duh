// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import (
	"encoding/binary"
	"errors"
	"fmt"
)

func parseITInstrument(h []byte, sampleCount int) (ITInstrument, error) {
	var inst ITInstrument

	if h[17] > 3 || h[18] > 3 || h[18] != 0 && h[19] > 2 {
		return inst, errors.New("unsupported new-note or duplicate-note action")
	}

	inst.FilterEnvelope = h[468]&0x80 != 0
	inst.NewNoteAction = h[17]
	inst.DuplicateCheck = h[18]
	inst.DuplicateAction = h[19]
	inst.Fadeout = binary.LittleEndian.Uint16(h[20:])
	inst.GlobalVolume = min(h[24], 128)
	inst.HasPanning = h[25]&0x80 == 0
	inst.Panning = min(h[25]&0x7f, 64)
	inst.PitchPanSeparation = int8(h[22])
	inst.PitchPanCenter = h[23]
	inst.VolumeSwing = min(h[26], 100)
	inst.PanSwing = min(h[27], 64)
	inst.HasFilterCutoff, inst.FilterCutoff = h[58]&128 != 0, h[58]&127
	inst.HasFilterResonance, inst.FilterResonance = h[59]&128 != 0, h[59]&127

	for i, offset := range [...]int{304, 386, 468} {
		if h[offset]&1 == 0 {
			copy(inst.itDisabledEnvData[i][:], h[offset:offset+82])
		}
	}

	for i := range 120 {
		note, sample := h[64+i*2], h[65+i*2]
		if int(sample) > sampleCount || sample != 0 && note >= 120 {
			return inst, fmt.Errorf("invalid note mapping %d", i)
		}
		inst.Samples[i], inst.Notes[i] = sample, note
	}

	envelopes := []struct {
		name   string
		offset int
		signed bool
		dest   *ITVolumeEnvelope
	}{
		{"volume", 304, false, &inst.VolumeEnvelope},
		{"panning", 386, true, &inst.PanEnvelope},
		{"pitch", 468, true, &inst.PitchEnvelope},
	}

	for _, spec := range envelopes {
		data := h[spec.offset : spec.offset+82]
		if spec.offset == 468 && inst.FilterEnvelope {
			data = append([]byte(nil), data...)
			data[0] &^= 0x80
		}

		env, err := parseITEnvelope(data, spec.signed)
		if err != nil {
			return inst, fmt.Errorf("%s envelope: %w", spec.name, err)
		}

		*spec.dest = env
	}
	return inst, nil
}

func parseITEnvelope(b []byte, signed bool) (ITVolumeEnvelope, error) {
	var env ITVolumeEnvelope

	if b[0]&1 == 0 {
		return env, nil
	}

	if b[0]&^byte(15) != 0 || b[1] == 0 || b[1] > 25 {
		return env, errors.New("unsupported flags or invalid point count")
	}

	env.Enabled, env.Loop, env.Sustain = true, b[0]&2 != 0, b[0]&4 != 0
	env.Carry = b[0]&8 != 0
	env.LoopStart, env.LoopEnd, env.SustainStart, env.SustainEnd = b[2], b[3], b[4], b[5]
	count := int(b[1])

	if env.Loop && (int(env.LoopEnd) >= count || env.LoopStart > env.LoopEnd) ||
		env.Sustain && (int(env.SustainEnd) >= count || env.SustainStart > env.SustainEnd) {
		return env, errors.New("invalid loop")
	}

	for i := range count {
		value := int16(b[6+i*3])
		if signed {
			value = int16(int8(b[6+i*3]))
		}

		tick := binary.LittleEndian.Uint16(b[7+i*3:])

		if value > 64 || signed && (value < -32 || value > 32) || i > 0 && tick < env.Points[i-1].Tick {
			return env, errors.New("invalid point")
		}

		env.Points = append(env.Points, ITEnvelopePoint{Tick: tick, Value: value})
	}
	return env, nil
}
