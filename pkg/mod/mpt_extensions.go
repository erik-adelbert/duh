// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// mptExtensions preserves parsed properties; it does not authorize playback.
// The caller must independently locate the bounded extension region and validate
// the serialized MPTM sequence, compatibility flags, and playback semantics.
type mptExtensions struct {
	instrumentFadeout            []uint32
	instrumentMIDIPWD            []int8
	instrumentMIDIChannel        []uint8
	instrumentMIDIProgram        []uint8
	instrumentMIDIBank           []uint16
	instrumentPlugins            []uint8
	instrumentCutSwing           []uint8
	instrumentResSwing           []uint8
	instrumentResampling         []uint32
	instrumentVolumeRamp         []uint16
	instrumentGlobalVolume       []uint32
	instrumentPanning            []uint32
	instrumentFilterMode         []uint8
	instrumentPitchReleaseNode   []uint8
	instrumentPanReleaseNode     []uint8
	instrumentVolumeReleaseNode  []uint8
	instrumentPitchTempoLock     []uint16
	instrumentPitchTempoFraction []uint16
	instrumentPluginVelocity     []uint8
	instrumentPluginVolume       []uint8
	rowsPerBeat                  *uint32
	rowsPerMeasure               *uint32
	songResampling               *uint32
	legacyRestart                *uint16
	extraChannelSettings         []byte
	sampleCues                   map[uint16][]uint32
	channels                     *uint16
	tempoMode                    *uint8
	defaultTempo                 *uint32
	defaultTempoFraction         *uint32
	mixLevels                    *uint32
	createdVersion               *uint32
	lastSavedVersion             *uint32
	samplePreamp                 *uint32
	pluginVolume                 *uint32
	tempoSwing                   []uint32
	playbackFlags                []byte
	artist                       *string
}

func parseMPTExtensions(data []byte, instrumentCount int) (mptExtensions, error) {
	var result mptExtensions

	if instrumentCount < 0 || instrumentCount > 255 {
		return result, errors.New("invalid MPTM instrument count")
	}

	type fieldKey struct {
		song bool
		code string
	}

	seen := make(map[fieldKey]bool)

	instruments, song := false, false
	for len(data) > 0 {
		if len(data) >= 4 && string(data[:4]) == "XTPM" {
			if instruments || song {
				return mptExtensions{}, errors.New("unexpected MPTM instrument extension header")
			}

			instruments = true
			data = data[4:]
			continue
		}

		if len(data) >= 4 && string(data[:4]) == "STPM" {
			if song {
				return mptExtensions{}, errors.New("duplicate MPTM song extension header")
			}
			song = true
			data = data[4:]
			continue
		}

		if !instruments && !song {
			return mptExtensions{}, errors.New("missing MPTM extension header")
		}

		if len(data) < 6 {
			return mptExtensions{}, errors.New("truncated MPTM extension field header")
		}

		code := string(data[:4])
		size := int(u16(data[4:6]))
		data = data[6:]

		if !song {
			if instrumentCount == 0 || size == 0 {
				return mptExtensions{}, fmt.Errorf("invalid MPTM instrument field %q size or count", code)
			}
			size *= instrumentCount
		}

		if size > len(data) {
			return mptExtensions{}, fmt.Errorf("truncated MPTM field %q", code)
		}

		field := data[:size]
		data = data[size:]
		key := fieldKey{song, code}

		if seen[key] && (!song || code != "CUES") {
			return mptExtensions{}, fmt.Errorf("duplicate MPTM field %q", code)
		}

		seen[key] = true

		if err := result.readField(song, code, field, instrumentCount); err != nil {
			return mptExtensions{}, err
		}
	}

	if !instruments && !song {
		return mptExtensions{}, errors.New("missing MPTM extension header")
	}

	return result, nil
}

func (e *mptExtensions) readField(song bool, code string, field []byte, instrumentCount int) error {
	invalidSize := func() error { return fmt.Errorf("invalid size %d for MPTM field %q", len(field), code) }
	if !song {
		switch code {
		case "..RV", "..BM", "LTTP", "PTTF":
			if len(field) != instrumentCount*2 {
				return invalidSize()
			}
			values := make([]uint16, instrumentCount)
			for i := range values {
				values[i] = u16(field[i*2:])
			}
			switch code {
			case "..RV":
				e.instrumentVolumeRamp = values
			case "LTTP":
				e.instrumentPitchTempoLock = values
			case "PTTF":
				e.instrumentPitchTempoFraction = values
			case "..BM":
				e.instrumentMIDIBank = values
			}
		case "...R", "..VG":
			width := len(field) / instrumentCount
			if width != 1 && width != 2 && width != 4 {
				return invalidSize()
			}

			values := make([]uint32, instrumentCount)
			for i := range values {
				for j := range width {
					values[i] |= uint32(field[i*width+j]) << (8 * j)
				}
			}

			if code == "...R" {
				e.instrumentResampling = values
			} else {
				e.instrumentGlobalVolume = values
			}
		case "..OF", "...P":
			if len(field) != instrumentCount*4 {
				return invalidSize()
			}

			values := make([]uint32, instrumentCount)
			for i := range values {
				values[i] = u32(field[i*4:])
			}

			if code == "..OF" {
				e.instrumentFadeout = values
			} else {
				e.instrumentPanning = values
			}
		case "DWPM":
			if len(field) != instrumentCount {
				return invalidSize()
			}

			e.instrumentMIDIPWD = make([]int8, instrumentCount)
			for i := range e.instrumentMIDIPWD {
				e.instrumentMIDIPWD[i] = int8(field[i])
			}
		case "..SC", "..SR", ".PiM", "..CM", "..PM", "..MF", "NREP", "NREA", "NREV", "HEVP", "HOVP":
			if len(field) != instrumentCount {
				return invalidSize()
			}

			values := append([]uint8(nil), field...)
			switch code {
			case "..SC":
				e.instrumentCutSwing = values
			case "..SR":
				e.instrumentResSwing = values
			case "..CM":
				e.instrumentMIDIChannel = values
			case "..PM":
				e.instrumentMIDIProgram = values
			case "..MF":
				e.instrumentFilterMode = values
			case "NREP":
				e.instrumentPitchReleaseNode = values
			case "NREA":
				e.instrumentPanReleaseNode = values
			case "NREV":
				e.instrumentVolumeReleaseNode = values
			case "HEVP":
				e.instrumentPluginVelocity = values
			case "HOVP":
				e.instrumentPluginVolume = values
			case ".PiM":
				e.instrumentPlugins = values
			}
		default:
			return fmt.Errorf("unsupported MPTM instrument field %q", code)
		}
		return nil
	}
	switch code {
	case "CUES":
		return e.readSampleCues(field)
	case "SnhC":
		if len(field)%2 != 0 {
			return invalidSize()
		}

		if len(field) > (MaxTracks-itHeaderChannels)*2 {
			return fmt.Errorf("MPTM channel settings exceed the supported limit of %d channels", MaxTracks)
		}
		e.extraChannelSettings = make([]byte, len(field))
		copy(e.extraChannelSettings, field)
	case "CCOL":
		if len(field)%4 != 0 {
			return invalidSize()
		}
	case "...C":
		if len(field) != 2 {
			return invalidSize()
		}

		value := u16(field)
		e.channels = &value
	case "..PR":
		if len(field) != 2 {
			return invalidSize()
		}
		value := u16(field)
		e.legacyRestart = &value
	case "..MT":
		if len(field) != 1 {
			return invalidSize()
		}
		value := field[0]
		e.tempoMode = &value
	case "..TD", "DTFR", ".VWC", ".MMP", "VWSL", ".APS", "VTSV", ".BPR", ".MPR", "RSMP":
		if len(field) != 4 {
			return invalidSize()
		}
		value := u32(field)
		switch code {
		case "RSMP":
			e.songResampling = &value
		case ".BPR":
			e.rowsPerBeat = &value
		case ".MPR":
			e.rowsPerMeasure = &value
		case "..TD":
			e.defaultTempo = &value
		case "DTFR":
			e.defaultTempoFraction = &value
		case ".VWC":
			e.createdVersion = &value
		case ".MMP":
			e.mixLevels = &value
		case "VWSL":
			e.lastSavedVersion = &value
		case ".APS":
			e.samplePreamp = &value
		case "VTSV":
			e.pluginVolume = &value
		}
	case "SWNG":
		if len(field) < 2 || len(field) != 2+int(u16(field))*4 {
			return invalidSize()
		}
		e.tempoSwing = make([]uint32, (len(field)-2)/4)
		for i := range e.tempoSwing {
			e.tempoSwing[i] = u32(field[2+i*4:])
		}
	case ".FSM":
		e.playbackFlags = make([]byte, len(field))
		copy(e.playbackFlags, field)
	case "AUTH":
		value := string(field)
		e.artist = &value
	default:
		return fmt.Errorf("unsupported MPTM song field %q", code)
	}
	return nil
}

var (
	u16 = binary.LittleEndian.Uint16
	u32 = binary.LittleEndian.Uint32
)
