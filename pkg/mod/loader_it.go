// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/erik-adelbert/duh/pkg/mmcmp"
)

// LoadIT loads a subset of IT modules with mono or stereo PCM samples.
// Unsupported IT features are reported rather than silently ignored.
func LoadIT(path string) (*S3MLoadResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	data, err = unpackITContainer(data)
	if err != nil {
		return nil, err
	}

	return loadIT(data)
}

func unpackITContainer(data []byte) ([]byte, error) {
	if bytes.HasPrefix(data, []byte("ziRCONia")) {
		rd, sz := bytes.NewReader(data), int64(len(data))

		reader, err := mmcmp.Unpack(rd, sz)
		if err != nil {
			return nil, fmt.Errorf("invalid MMCMP container: %w", err)
		}

		data, err = io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("decompressing MMCMP container: %w", err)
		}
	}

	return data, nil
}

func loadIT(data []byte) (*S3MLoadResult, error) {
	if isMPTM(data) {
		return nil, errors.New("MPTM modules are not yet supported")
	}
	return loadITData(data, false)
}

func loadITData(data []byte, experimentalMPTM bool) (*S3MLoadResult, error) {
	return loadITDataChannels(data, experimentalMPTM, itHeaderChannels)
}

func loadITDataChannels(data []byte, experimentalMPTM bool, patternChannels int) (*S3MLoadResult, error) {
	if len(data) < 192 || string(data[:4]) != "IMPM" {
		return nil, errors.New("not an IT module")
	}

	u16 := func(off int) int { return int(binary.LittleEndian.Uint16(data[off:])) }
	u32 := func(off int) uint32 { return binary.LittleEndian.Uint32(data[off:]) }

	orders, instruments, samples, patterns := u16(32), u16(34), u16(36), u16(38)

	instrumentMode := u16(44)&4 != 0
	if instrumentMode && instruments == 0 {
		return nil, errors.New("IT instrument mode has no instruments")
	}

	if orders == 0 || patterns == 0 || 192+orders+4*(instruments+samples+patterns) > len(data) {
		return nil, errors.New("invalid or truncated IT pointer table")
	}

	if data[50] == 0 || data[51] < 32 {
		return nil, errors.New("invalid IT speed or tempo")
	}

	assets := SongAssets{
		IT: true, NumTracks: itHeaderChannels, ITGlobalVolume: min(data[48], 128), ITMixGain: 1,
		ITCompatibility: ITCompatibility{
			ExtendedFilter:        u16(44)&0x1000 != 0,
			OldEffects:            u16(44)&0x10 != 0,
			LinearSlides:          u16(44)&0x08 != 0,
			CompatibleGxx:         u16(44)&0x20 != 0,
			HighOffsetUnsupported: u16(40) < 0x200,
			LegacyModPlug:         u16(40) == 0x217 && u16(42) == 0x200 && u32(60) == 0,
		},
	}

	assets.ITExtendedFilter = assets.ITCompatibility.ExtendedFilter
	assets.ITLegacyPitchPan = assets.ITCompatibility.LegacyModPlug

	for i := range itHeaderChannels {
		channelPan := data[64+i]
		if channelPan == 255 {
			assets.ITChannelVolume[i] = 64
			continue
		}

		pan := channelPan & 127
		if pan <= 64 {
			assets.Panning[i] = float32(pan)/32 - 1
		}

		assets.ITMuted[i] = channelPan&128 != 0
		assets.ITSurround[i] = pan == 100
		assets.ITChannelVolume[i] = min(data[128+i], 64)
	}

	for _, order := range data[192 : 192+orders] {
		if order == 255 {
			break
		}

		if order == 254 {
			assets.OrderList = append(assets.OrderList, itOrderSkip)
			continue
		}

		if int(order) >= patterns {
			return nil, fmt.Errorf("unsupported IT order %d", order)
		}

		assets.OrderList = append(assets.OrderList, int(order))
	}

	if nextITOrder(assets.OrderList, 0) == len(assets.OrderList) {
		return nil, errors.New("IT module has no playable orders")
	}

	base := 192 + orders

	if instrumentMode {
		if u16(42) < 0x200 {
			return nil, errors.New("old IT instrument headers are not supported")
		}

		assets.ITInstruments = make([]ITInstrument, instruments)
		for i := range instruments {
			ptr := int(u32(base + i*4))
			if ptr == 0 || ptr > len(data)-554 || string(data[ptr:ptr+4]) != "IMPI" {
				return nil, fmt.Errorf("invalid IT instrument header %d", i+1)
			}

			instrument, err := parseITInstrument(data[ptr:ptr+554], samples)
			if err != nil {
				return nil, fmt.Errorf("IT instrument %d: %w", i+1, err)
			}

			if experimentalMPTM && instrument.DuplicateCheck != 0 &&
				(instrument.DuplicateCheck != 1 || instrument.DuplicateAction != 0) &&
				(instrument.DuplicateCheck != 2 || instrument.DuplicateAction != 0) &&
				(instrument.DuplicateCheck != 3 || instrument.DuplicateAction != 1) {
				return nil, fmt.Errorf("IT instrument %d: unsupported MPTM duplicate-note action", i+1)
			}

			assets.ITInstruments[i] = instrument
		}
	}

	base += instruments * 4

	assets.Samples = make([]SampleAsset, samples)
	for i := range samples {
		ptr := int(u32(base + i*4))
		if ptr == 0 {
			continue
		}

		if ptr > len(data)-80 || string(data[ptr:ptr+4]) != "IMPS" {
			return nil, fmt.Errorf("invalid IT sample header %d", i+1)
		}

		h := data[ptr : ptr+80]
		flags, convert := h[18], h[46]

		length := uint64(u32(ptr + 48))
		if length == 0 {
			continue
		}

		if experimentalMPTM && convert == 0x40 {
			offset := uint64(u32(ptr + 72))

			if flags&^byte(0x19) != 0 || flags&1 == 0 || h[47]&128 != 0 && h[47]&127 > 64 ||
				length != 12 || offset > uint64(len(data)) || 12 > uint64(len(data))-offset {
				return nil, fmt.Errorf("invalid MPTM OPL sample %d", i+1)
			}

			patch := new(oplPatch)
			copy(patch[:], data[int(offset):int(offset)+12])

			sample := itSampleMetadata(h)
			sample.opl = patch

			assets.Samples[i] = sample
			continue
		}

		allowedConvert := byte(1)
		compressed := flags&8 != 0

		if compressed {
			allowedConvert |= 4
		}

		sustain := flags&0x20 != 0
		sustainPingPong := sustain && flags&0x80 != 0
		if sustain && !instrumentMode || convert&^allowedConvert != 0 {
			return nil, fmt.Errorf("IT sample %d uses unsupported encoding or loop (flags %#x, conversion %#x)", i+1, flags, convert)
		}

		stereo := flags&4 != 0
		if stereo && u16(40) < 0x214 {
			return nil, fmt.Errorf("IT sample %d uses a stereo flag in a pre-2.14 file", i+1)
		}

		if assets.ITCompatibility.LegacyModPlug && h[77]&127 != 0 && h[78] != 0 && h[79]&7 == 3 {
			return nil, fmt.Errorf("IT sample %d uses unsupported legacy random auto-vibrato", i+1)
		}

		if flags&1 == 0 || h[47]&128 != 0 && h[47]&127 > 64 {
			return nil, fmt.Errorf("invalid IT sample %d flags or panning", i+1)
		}

		width := uint64(1)
		if flags&2 != 0 {
			width = 2
		}

		offset := uint64(u32(ptr + 72))
		if offset == 0 || offset > uint64(len(data)) {
			return nil, fmt.Errorf("truncated IT sample %d", i+1)
		}

		var pcm, right []float32
		if compressed {
			if length > (uint64(len(data))-offset)*8 {
				return nil, fmt.Errorf("truncated IT compressed sample %d", i+1)
			}

			var err error
			var consumed int

			pcm, consumed, err = decodeITCompressed(data[int(offset):], int(length), width == 2, convert&4 != 0)
			if err != nil {
				return nil, fmt.Errorf("IT compressed sample %d: %w", i+1, err)
			}

			if stereo {
				right, _, err = decodeITCompressed(data[int(offset)+consumed:], int(length), width == 2, convert&4 != 0)
				if err != nil {
					return nil, fmt.Errorf("IT compressed sample %d right channel: %w", i+1, err)
				}
			}
		} else {
			channels := uint64(1)
			if stereo {
				channels = 2
			}

			if length > (uint64(len(data))-offset)/(width*channels) {
				return nil, fmt.Errorf("truncated IT sample %d", i+1)
			}

			pcm = decodeS3MSample(data, int(offset), int(length), width == 2, convert&1 != 0)
			if stereo {
				right = decodeS3MSample(data, int(offset+length*width), int(length), width == 2, convert&1 != 0)
			}
		}

		start, end := u32(ptr+52), u32(ptr+56)
		if flags&0x10 != 0 && (start >= end || uint64(end) > length) {
			return nil, fmt.Errorf("invalid IT loop in sample %d", i+1)
		}

		sustainStart, sustainEnd := u32(ptr+64), u32(ptr+68)
		if sustain && (sustainStart >= sustainEnd || uint64(sustainEnd) > length) {
			return nil, fmt.Errorf("invalid IT sustain loop in sample %d", i+1)
		}

		if experimentalMPTM && sustain && (sustainPingPong || flags&0x50 == 0x50) && sustainEnd-sustainStart == 1 {
			return nil, fmt.Errorf("MPTM sample %d: one-sample ping-pong sustain loop is not supported", i+1)
		}

		sample := itSampleMetadata(h)
		sample.Data, sample.RightData = pcm, right
		sample.LoopStart, sample.LoopEnd = start, end
		sample.LoopEnable = flags&0x10 != 0
		sample.LoopPingPong = sample.LoopEnable && flags&0x40 != 0
		sample.mptSustainStart, sample.mptSustainEnd, sample.mptSustainLoop = sustainStart, sustainEnd, sustain
		sample.itSustainPingPong = sustainPingPong
		assets.Samples[i] = sample
	}

	if experimentalMPTM {
		for i, instrument := range assets.ITInstruments {
			if instrument.DuplicateCheck != 3 || instrument.DuplicateAction != 1 {
				continue
			}

			for _, sample := range instrument.Samples {
				if sample != 0 && assets.Samples[sample-1].opl != nil {
					return nil, fmt.Errorf("IT instrument %d: duplicate-instrument note-off with OPL samples is not supported", i+1)
				}
			}
		}
	}

	base += samples * 4
	assets.Patterns = make([]PatternAsset, patterns)

	hasOPLSample := false
	for _, sample := range assets.Samples {
		hasOPLSample = hasOPLSample || sample.opl != nil
	}

	var enableEnvelopes [3]bool
	var macroConfig *mptMacroConfig
	for i := range patterns {
		var rows int
		var packed []byte

		ptr := int(u32(base + i*4))

		if ptr == 0 {
			rows = 64
		} else {
			if ptr > len(data)-8 {
				return nil, fmt.Errorf("truncated IT pattern %d", i)
			}

			length := int(u16(ptr))
			rows = int(u16(ptr + 2))

			if rows < 1 || rows > 1024 || length > len(data)-ptr-8 {
				return nil, fmt.Errorf("invalid IT pattern %d", i)
			}

			packed = data[ptr+8 : ptr+8+length]
		}

		pat, err := unpackITPatternChannels(packed, rows, experimentalMPTM, patternChannels)
		if err != nil {
			return nil, fmt.Errorf("IT pattern %d: %w", i, err)
		}

		for ri, row := range pat.Rows {
			for channel, cmd := range row {
				if !experimentalMPTM && (cmd.EffectCmd == 0x19 || cmd.EffectCmd == 0x26) {
					command := 'Z'
					if cmd.EffectCmd == 0x26 {
						command = '\\'
						if assets.ITCompatibility.LegacyModPlug {
							return nil, fmt.Errorf("IT pattern %d row %d channel %d: smooth MIDI macros with legacy ModPlug filter behavior are not supported", i, ri, channel+1)
						}
					}
					if macroConfig == nil {
						config, err := readMPTMacroConfig(data, len(data))
						if err != nil {
							return nil, err
						}

						macroConfig = &config
					}

					for _, macro := range macroConfig.candidates(cmd.EffectArg) {
						if macro.kind == mptMacroUnsupported {
							return nil, fmt.Errorf("IT pattern %d row %d channel %d: unsupported embedded MIDI macros for %c%02X", i, ri, channel+1, command, cmd.EffectArg)
						}

						if assets.ITCompatibility.LegacyModPlug && macro.kind == mptMacroResonance {
							return nil, fmt.Errorf("IT pattern %d row %d channel %d: unsupported filter macro behavior for Z%02X", i, ri, channel+1, cmd.EffectArg)
						}
					}
				}

				if cmd.EffectCmd == 0x15 {
					switch cmd.EffectArg {
					case 0x78:
						enableEnvelopes[0] = true
					case 0x7a:
						enableEnvelopes[1] = true
					case 0x7c:
						enableEnvelopes[2] = true
					}
				}

				if cmd.Note == 253 && !instrumentMode {
					return nil, fmt.Errorf("IT pattern %d row %d channel %d: note fade in sample mode is not supported", i, ri, channel+1)
				}

				if experimentalMPTM && !instrumentMode && cmd.Note == 255 && cmd.EffectCmd == 0x1b {
					return nil, fmt.Errorf("IT pattern %d row %d channel %d: Qxx with note-off in sample mode is not supported", i, ri, channel+1)
				}

				if cmd.Note == 253 {
					itPitchEffect := !experimentalMPTM && cmd.Instrument == 0 && !cmd.HasVolume && (cmd.EffectCmd == 0x03 || cmd.EffectCmd == 0x04)
					itVolume := !experimentalMPTM && itFadeDirectVolume(cmd)
					itVolumeSlide := !experimentalMPTM && cmd.Instrument == 0 && cmd.HasVolume && cmd.Volume >= 85 && cmd.Volume <= 104 && cmd.EffectCmd == 0
					itFineVolumeTiming := !experimentalMPTM && cmd.Instrument == 0 && cmd.HasVolume && cmd.Volume >= 65 && cmd.Volume <= 84 &&
						(cmd.EffectCmd == 0x0f || cmd.EffectCmd == 0x20 && cmd.EffectArg >= 0x20)
					if !itPitchEffect && !itVolume && !itVolumeSlide && !itFineVolumeTiming && (!mptNoteFadeCombinationSupported(cmd) || (!experimentalMPTM && cmd.EffectCmd == 0x0a)) {
						return nil, fmt.Errorf("IT pattern %d row %d channel %d: note fade with other commands is not supported", i, ri, channel+1)
					}

					if cmd.EffectCmd == 0x0a && hasOPLSample {
						return nil, fmt.Errorf("IT pattern %d row %d channel %d: note fade with Dxx and OPL samples is not supported", i, ri, channel+1)
					}
				}

				if assets.ITCompatibility.LegacyModPlug && cmd.EffectCmd == 0x15 && cmd.EffectArg == 0xc0 {
					cmd.Note, cmd.EffectCmd, cmd.EffectArg = 254, 0, 0
					row[channel] = cmd
				}

				if !experimentalMPTM && assets.ITCompatibility.LegacyModPlug && cmd.EffectCmd == 0x15 &&
					cmd.EffectArg >= 0x30 && cmd.EffectArg <= 0x5f && cmd.EffectArg&3 == 3 {
					names := [...]string{"vibrato", "tremolo", "panbrello"}
					return nil, fmt.Errorf("IT pattern %d row %d channel %d: legacy random %s waveform is not supported", i, ri, channel+1, names[cmd.EffectArg>>4-3])
				}

				limit := samples
				if instrumentMode {
					limit = instruments
				}

				if int(cmd.Instrument) > limit {
					if experimentalMPTM || !instrumentMode || cmd.Note < 1 || cmd.Note > 120 || itHasTonePortamento(cmd) {
						return nil, fmt.Errorf("IT pattern %d row %d channel %d references missing instrument or sample %d", i, ri, channel+1, cmd.Instrument)
					}
					if assets.ITCompatibility.LegacyModPlug {
						// Legacy ModPlug silences the channel for an invalid instrument.
						cmd.HasVolume, cmd.Volume = true, 0
						row[channel] = cmd
					}
				}
			}
		}
		assets.Patterns[i] = pat
	}
	if !experimentalMPTM {
		assets.ITCompatibility.mptMacroConfig = macroConfig
	}
	if err := validateITPatternDelays(assets.Patterns, experimentalMPTM, instrumentMode); err != nil {
		return nil, err
	}
	for i := range assets.ITInstruments {
		instrument := &assets.ITInstruments[i]
		envs := [...]*ITVolumeEnvelope{&instrument.VolumeEnvelope, &instrument.PanEnvelope, &instrument.PitchEnvelope}
		for kind, enabled := range enableEnvelopes {
			if !enabled || envs[kind].Enabled {
				continue
			}
			raw := instrument.itDisabledEnvData[kind]
			if raw[1] == 0 {
				continue
			}
			raw[0] |= 1
			if kind == 2 {
				raw[0] &^= 0x80
			}
			env, err := parseITEnvelope(raw[:], kind != 0)
			if err != nil {
				return nil, fmt.Errorf("IT instrument %d disabled envelope %d needed for S7x: %w", i+1, kind, err)
			}
			env.Enabled = false
			*envs[kind] = env
		}
	}
	// ModPlug Tracker 1.09-1.16 uses the original channel-count preamp.
	if assets.ITCompatibility.LegacyModPlug {
		for i := range assets.ITInstruments {
			if assets.ITInstruments[i].PanEnvelope.Enabled {
				assets.ITInstruments[i].PanSwing = 0
			}
		}
		channels := 4
		for _, pattern := range assets.Patterns {
			for _, row := range pattern.Rows {
				for ch, cmd := range row {
					if cmd != (ModCommand{}) && ch+1 > channels {
						channels = ch + 1
					}
				}
			}
		}
		preamp := [...]int{96, 96, 96, 112, 128, 136, 144, 152, 160, 164, 168, 172, 176, 180, 184, 188}
		master := (128 * int(min(data[49], 128))) / 64
		assets.ITMixGain = float32((master*128)/preamp[min(channels, 31)/2]) / 512
	}
	return &S3MLoadResult{Assets: assets, InitialSpeed: int(data[50]), InitialTempo: float64(data[51])}, nil
}

func mptNoteFadeCombinationSupported(cmd ModCommand) bool {
	if cmd.Instrument != 0 {
		return false
	}
	if cmd.HasVolume {
		return cmd.Volume == 76 && cmd.EffectCmd == 0 && cmd.EffectArg == 0
	}
	return cmd.EffectCmd == 0 && cmd.EffectArg == 0 ||
		cmd.EffectCmd == 0x02 && cmd.EffectArg == 4 ||
		cmd.EffectCmd == 0x0a
}

func itFadeDirectVolume(cmd ModCommand) bool {
	return cmd.Note == 253 && cmd.Instrument == 0 && cmd.HasVolume && cmd.Volume <= 64 &&
		(cmd.EffectCmd == 0 || cmd.EffectCmd == 0x02)
}

func itSampleMetadata(h []byte) SampleAsset {
	c5Speed := binary.LittleEndian.Uint32(h[60:])
	if c5Speed == 0 {
		c5Speed = 8363
	}
	return SampleAsset{
		C5Speed:        max(c5Speed, 256),
		Volume:         min(h[19], 64),
		ITGlobalVolume: min(h[17], 64),
		ITPanning:      h[47] & 127,
		ITHasPanning:   h[47]&128 != 0,
		ITVibratoRate:  h[76],
		ITVibratoDepth: h[77] & 127,
		ITVibratoSweep: h[78],
		ITVibratoType:  h[79] & 7,
	}
}

func isMPTM(data []byte) bool {
	if len(data) < 192 {
		return false
	}
	if string(data[:4]) == "tpm." {
		return true
	}
	if string(data[:4]) != "IMPM" {
		return false
	}
	version := binary.LittleEndian.Uint16(data[40:])
	if version <= 0x888 || version > 0xfff {
		return false
	}
	offset := uint64(binary.LittleEndian.Uint32(data[len(data)-4:]))
	return offset >= 0x100 && offset+3 <= uint64(len(data)-4) && string(data[offset:offset+3]) == "228"
}

func unpackITPatternChannels(data []byte, count int, mptm bool, channels int) (PatternAsset, error) {
	if channels < 1 || channels > MaxTracks {
		return PatternAsset{}, fmt.Errorf("invalid IT pattern channel count %d", channels)
	}
	p := PatternAsset{Rows: make([][]ModCommand, count)}
	for i := range p.Rows {
		p.Rows[i] = make([]ModCommand, channels)
	}
	if data == nil {
		return p, nil
	}
	var masks [MaxTracks]byte
	var last [MaxTracks]ModCommand
	row, at := 0, 0
	read := func() (byte, error) {
		if at >= len(data) {
			return 0, errors.New("truncated pattern")
		}
		v := data[at]
		at++
		return v, nil
	}
	for row < count {
		ch, err := read()
		if err != nil {
			return p, err
		}
		if ch == 0 {
			row++
			continue
		}
		channel := int(ch&0x7f) - 1
		if channel < 0 || channel >= channels {
			return p, fmt.Errorf("unsupported channel %d", channel+1)
		}
		if ch&0x80 != 0 {
			masks[channel], err = read()
			if err != nil {
				return p, err
			}
		}
		mask := masks[channel]
		cmd, prev := &p.Rows[row][channel], &last[channel]
		if mask&0x10 != 0 {
			cmd.Note = prev.Note
		}
		if mask&0x20 != 0 {
			cmd.Instrument = prev.Instrument
		}
		if mask&0x40 != 0 {
			cmd.Volume, cmd.HasVolume = prev.Volume, prev.HasVolume
		}
		if mask&0x80 != 0 {
			cmd.EffectCmd, cmd.EffectArg = prev.EffectCmd, prev.EffectArg
		}
		if mask&1 != 0 {
			n, err := read()
			if err != nil {
				return p, err
			}
			switch {
			case n < 120:
				cmd.Note = n + 1
			case n == 246 && !mptm:
				cmd.Note = 253
			case n == 253 && mptm:
				cmd.Note = 253
			case n == 254:
				cmd.Note = 254
			case n == 255:
				cmd.Note = 255
			default:
				return p, fmt.Errorf("unsupported IT note %d", n)
			}
			prev.Note = cmd.Note
		}
		if mask&2 != 0 {
			cmd.Instrument, err = read()
			if err != nil {
				return p, err
			}
			prev.Instrument = cmd.Instrument
		}
		if mask&4 != 0 {
			cmd.Volume, err = read()
			if err != nil {
				return p, err
			}
			if cmd.Volume >= 125 && cmd.Volume <= 127 ||
				cmd.Volume > 202 && (mptm && cmd.Volume != 203 && (cmd.Volume < 224 || cmd.Volume > 232) || !mptm && cmd.Volume > 212) {
				return p, fmt.Errorf("unsupported IT volume command %d", cmd.Volume)
			}
			cmd.HasVolume = true
			prev.Volume, prev.HasVolume = cmd.Volume, true
		}
		if mask&8 != 0 {
			effect, err := read()
			if err != nil {
				return p, err
			}
			arg, err := read()
			if err != nil {
				return p, err
			}
			cmd.EffectCmd, cmd.EffectArg, err = mapITPatternEffect(effect, arg, mptm)
			if err != nil {
				return p, err
			}
			prev.EffectCmd, prev.EffectArg = cmd.EffectCmd, cmd.EffectArg
		}

		if cmd.HasVolume && cmd.Volume >= 105 && cmd.Volume <= 124 &&
			(cmd.EffectCmd == 1 || cmd.EffectCmd == 2 || cmd.EffectCmd == 3 || cmd.EffectCmd == 0x1f ||
				cmd.EffectCmd == 0x15 && cmd.EffectArg&0xf0 == 0xd0) {
			return p, fmt.Errorf("IT volume pitch slide with portamento or note delay is not supported")
		}
		if cmd.HasVolume && cmd.Volume >= 203 && cmd.Volume <= 212 && cmd.EffectCmd != 0 &&
			!itVolumeVibratoEffectSupported(*cmd, mptm) {
			return p, fmt.Errorf("IT volume vibrato with an effect command is not supported")
		}
		if itHasVolumeTonePortamento(*cmd) && cmd.EffectCmd == 0x1f {
			return p, fmt.Errorf("IT volume tone portamento with Lxx is not supported")
		}
		if mptm && (cmd.EffectCmd == 0x1a && cmd.Note >= 253 ||
			cmd.EffectCmd == 0x1b && (cmd.Note == 253 || cmd.Note >= 254 && (cmd.Instrument != 0 || cmd.HasVolume)) ||
			(cmd.EffectCmd == 0x1a || cmd.EffectCmd == 0x1b) &&
				cmd.HasVolume && cmd.Volume > 104) {
			return p, fmt.Errorf("MPTM Q%02X combined with a special note or volume effect is not supported", cmd.EffectArg)
		}
		if mptm && cmd.EffectCmd == 0x25 {
			switch {
			case cmd.Note >= 253:
				return p, fmt.Errorf("MPTM delay-cut combined with a special note is not supported")
			case cmd.HasVolume && cmd.Volume > 64:
				return p, fmt.Errorf("MPTM delay-cut combined with a volume effect is not supported")
			case cmd.Note == 0 && cmd.Instrument != 0 && cmd.EffectArg&15 == 0:
				return p, fmt.Errorf("MPTM delay-cut ]x0 combined with an instrument-only cell is not supported")
			}
		}
	}
	return p, nil
}

func itVolumeVibratoEffectSupported(cmd ModCommand, mptm bool) bool {
	if cmd.EffectCmd == 0x03 || cmd.EffectCmd == 0x10 {
		return !mptm
	}
	if cmd.EffectCmd != 0x15 {
		return false
	}
	return cmd.EffectArg >= 0x30 && cmd.EffectArg <= 0x5f || !mptm && cmd.EffectArg == 0x77
}

func mapITPatternEffect(effect, arg byte, mptm bool) (uint8, uint8, error) {
	switch effect {
	case 9:
		return 0x22, arg, nil
	case 11:
		return 0x11, arg, nil
	case 12:
		return 0x1f, arg, nil
	case 14:
		return 0x1e, arg, nil
	case 16:
		return 0x1d, arg, nil
	case 18:
		return 0x23, arg, nil
	case 20:
		return 0x20, arg, nil
	case 21:
		return 0x21, arg, nil
	case 23:
		return 0x1c, arg, nil
	case 26:
		return 0x19, arg, nil
	case 28:
		return 0x26, arg, nil
	}
	if mptm {
		switch {
		case effect == 29:
			return 0x25, arg, nil
		case effect == 30:
			return 0x24, arg, nil
		case effect == 17 && arg == 0:
			return 0x1a, arg, nil
		case effect == 17:
			return 0x1b, arg, nil
		case effect == 19 && (arg >= 0x10 && arg <= 0x6f || arg == 0x70 || arg >= 0x73 && arg <= 0x7c || arg == 0x9f || arg >= 0xe0):
			return 0x15, arg, nil
		}
	} else {
		switch effect {
		case 17:
			return 0x1b, arg, nil
		case 19:
			if arg >= 0x70 && arg <= 0x7c || arg > 0 && arg < 0x10 ||
				arg&0xf0 == 0x10 || arg&0xf0 == 0x20 || arg >= 0x30 && arg <= 0x5f || arg >= 0xf0 {
				return 0x15, arg, nil
			}
		}
	}
	return mapITEffect(effect, arg)
}

func mapITEffect(effect, arg byte) (uint8, uint8, error) {
	switch effect {
	case 0:
		return 0, 0, nil
	case 1:
		return 0x0f, arg, nil
	case 2:
		return 0x0b, arg, nil
	case 3:
		return 0x0d, arg, nil
	case 4:
		return 0x0a, arg, nil
	case 5:
		return 0x02, arg, nil
	case 6:
		return 0x01, arg, nil
	case 7:
		return 0x03, arg, nil
	case 8:
		return 0x04, arg, nil
	case 10:
		return 0x10, arg, nil
	case 13:
		return 0x16, arg, nil
	case 15:
		return 0x12, arg, nil
	case 19:
		if arg == 0 || arg&0xf0 == 0x80 || arg == 0x90 || arg == 0x91 || arg >= 0xa0 && arg < 0xf0 {
			return 0x15, arg, nil
		}
		return 0, 0, fmt.Errorf("unsupported IT effect S%02X", arg)
	case 20:
		if arg < 32 {
			return 0, 0, fmt.Errorf("unsupported IT tempo slide T%02X", arg)
		}
		return 0x14, arg, nil
	case 22:
		return 0x17, arg, nil
	case 24:
		return 0x08, arg, nil
	case 25:
		return 0x18, arg, nil
	default:
		return 0, 0, fmt.Errorf("unsupported IT effect %c%02X", 'A'+effect-1, arg)
	}
}
