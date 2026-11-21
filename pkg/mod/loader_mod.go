// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.

package mod

import (
	"encoding/binary"
	"fmt"
)

const (
	modHeaderSize  = 1084
	modSampleCount = 31
	modChannels    = 4
	modPatternRows = 64
	modPatternSize = modPatternRows * modChannels * 4
)

func isProTrackerMOD(data []byte) bool {
	return len(data) >= modHeaderSize && string(data[1080:1084]) == "M.K."
}

func loadProTrackerMOD(data []byte) (*S3MLoadResult, error) {
	orderCount := int(data[950])
	if orderCount < 1 || orderCount > 128 {
		return nil, fmt.Errorf("ProTracker MOD: invalid order count %d", orderCount)
	}

	assets := SongAssets{proTracker: true, NumTracks: modChannels}
	assets.Panning = [MaxTracks]float32{-1, 1, 1, -1}
	assets.OrderList = make([]int, orderCount)
	patternCount := 0
	for i, entry := range data[952 : 952+orderCount] {
		if entry >= 128 {
			return nil, fmt.Errorf("ProTracker MOD: order %d references invalid pattern %d", i, entry)
		}
		assets.OrderList[i] = int(entry)
		patternCount = max(patternCount, int(entry)+1)
	}

	sampleDataOffset := modHeaderSize + patternCount*modPatternSize
	if sampleDataOffset > len(data) {
		return nil, fmt.Errorf("ProTracker MOD: truncated pattern data")
	}
	var sampleBytes int
	for i := range modSampleCount {
		start := 42 + i*30
		sampleBytes += modWordLength(data[start : start+2])
	}
	if len(data)-sampleDataOffset > sampleBytes {
		// Some editors save patterns referenced past the active order list.
		// Use those entries only if they account for all extra bytes.
		storedCount := patternCount
		for _, entry := range data[952+orderCount : 1080] {
			storedCount = max(storedCount, int(entry)+1)
		}
		if storedCount > 128 || len(data)-modHeaderSize-storedCount*modPatternSize != sampleBytes {
			return nil, fmt.Errorf("ProTracker MOD: file size does not match pattern and sample sizes")
		}
		sampleDataOffset = modHeaderSize + storedCount*modPatternSize
	}
	assets.Samples = make([]SampleAsset, modSampleCount)
	offset := sampleDataOffset
	for i := range assets.Samples {
		header := data[20+i*30 : 20+(i+1)*30]
		length := modWordLength(header[22:24])
		if length > len(data)-offset {
			return nil, fmt.Errorf("ProTracker MOD: truncated sample %d", i+1)
		}

		loopStart := modWordLength(header[26:28])
		loopLength := modWordLength(header[28:30])
		if loopLength > 2 && loopStart+loopLength > length {
			return nil, fmt.Errorf("ProTracker MOD: sample %d loop exceeds sample length", i+1)
		}
		tune := int8(header[24] & 15)
		if tune >= 8 {
			tune -= 16
		}
		sample := &assets.Samples[i]
		*sample = SampleAsset{
			Volume:      min(header[25], 64),
			C5Speed:     8363, // Pitch already comes from the Amiga period.
			modFineTune: tune,
		}
		if loopLength > 2 {
			sample.LoopEnable = true
			sample.LoopStart = uint32(loopStart)
			sample.LoopEnd = uint32(loopStart + loopLength)
		}
		sample.Data = make([]float32, length)
		for j, b := range data[offset : offset+length] {
			sample.Data[j] = float32(int8(b)) / 128
		}
		offset += length
	}
	var warnings []string
	var warned [32]bool
	var maxPanning uint8
	for pos := modHeaderSize; pos < modHeaderSize+patternCount*modPatternSize; pos += 4 {
		effect, arg := data[pos+2]&15, data[pos+3]
		switch {
		case effect == modSetPanning:
			maxPanning = max(maxPanning, arg)
		case effect == modExtended && arg>>4 == modFinePanning:
			maxPanning = max(maxPanning, (arg&15)<<4)
		}
	}
	assets.Patterns = make([]PatternAsset, patternCount)
	for pi := range assets.Patterns {
		rows := make([][]ModCommand, modPatternRows)
		for ri := range rows {
			rows[ri] = make([]ModCommand, modChannels)
			for ch := range rows[ri] {
				pos := modHeaderSize + pi*modPatternSize + (ri*modChannels+ch)*4
				cell := data[pos : pos+4]
				instrument := cell[0]&0xf0 | cell[2]>>4
				if instrument > modSampleCount {
					return nil, fmt.Errorf("ProTracker MOD: pattern %d row %d channel %d: invalid sample %d", pi, ri, ch+1, instrument)
				}
				rows[ri][ch] = ModCommand{
					Instrument: instrument,
					modPeriod:  binary.BigEndian.Uint16(cell[:2]) & 0x0fff,
				}
				effect, arg := cell[2]&15, cell[3]
				switch effect {
				case modArpeggio, modSlideUp, modSlideDown, modTonePorta, modVibrato, modPortaVolume, modVibratoVolume, modTremolo,
					modSetPanning, modSampleOffset, modVolumeSlide, modPositionJump, modSetVolume, modPatternBreak, modSpeedTempo:
					rows[ri][ch].EffectCmd, rows[ri][ch].EffectArg = effect, arg
					continue
				case modExtended:
					if arg>>4 == modFinePanning {
						// Low-valued panning commands are also used as demo sync markers.
						if maxPanning >= 0x30 {
							rows[ri][ch].EffectCmd = modSetPanning
							rows[ri][ch].EffectArg = (arg & 15) * 17
						}
						continue
					}
					switch arg >> 4 {
					case modLEDFilter, modFineSlideUp, modFineSlideDown, modPatternLoop, modRetrigger,
						modFineVolumeUp, modFineVolumeDown, modNoteCut, modNoteDelay, modPatternDelay:
						rows[ri][ch].EffectCmd, rows[ri][ch].EffectArg = effect, arg
						continue
					}
				}
				key, label := effect, fmt.Sprintf("%Xxx", effect)
				if effect == 0x0e {
					key, label = 16+(arg>>4), fmt.Sprintf("E%Xx", arg>>4)
				}
				if !warned[key] {
					warned[key] = true
					warnings = append(warnings, "ProTracker MOD: "+label+" effect is not supported")
				}
			}
		}
		assets.Patterns[pi] = PatternAsset{Rows: rows}
	}

	return &S3MLoadResult{Assets: assets, InitialSpeed: 6, InitialTempo: 125, Warnings: warnings}, nil
}

func modWordLength(data []byte) int {
	return int(binary.BigEndian.Uint16(data)) * 2
}
