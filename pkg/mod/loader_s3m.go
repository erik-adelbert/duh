// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

type S3MLoadResult struct {
	Assets       SongAssets
	InitialSpeed int
	InitialTempo float64
	Warnings     []string // Playback limitations that do not prevent loading.
}

func LoadS3M(path string) (*S3MLoadResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return loadS3M(data)
}

func loadS3M(data []byte) (*S3MLoadResult, error) {
	if len(data) < 0x60 || string(data[0x2C:0x30]) != "SCRM" {
		return nil, errors.New("not a valid S3M file (missing SCRM signature)")
	}

	ordNum := int(binary.LittleEndian.Uint16(data[0x20:0x22]))
	insNum := int(binary.LittleEndian.Uint16(data[0x22:0x24]))
	patNum := int(binary.LittleEndian.Uint16(data[0x24:0x26]))
	trackerVersion := binary.LittleEndian.Uint16(data[0x28:0x2a])
	tracker := trackerVersion >> 12
	schismDays := int(trackerVersion) - 0x4050
	if trackerVersion == 0x4fff {
		schismDays = int(binary.LittleEndian.Uint16(data[0x36:0x38]))
	}
	frequencyPitch := tracker == 3 || tracker == 4 && trackerVersion != 0x4100 && schismDays >= 4201
	signedSamples := binary.LittleEndian.Uint16(data[0x2A:0x2C]) == 1

	initSpeed := int(data[0x31])
	if initSpeed == 0 {
		initSpeed = 6
	}

	initTempo := float64(data[0x32])
	if initTempo < 32 {
		initTempo = 125.0
	}

	// Determine active track count from channel settings (0x40..0x5F)
	numTracks := 0
	for ch := range 32 {
		setting := data[0x40+ch]
		if setting < 16 { // 0..7 = Left PCM, 8..15 = Right PCM
			if ch+1 > numTracks {
				numTracks = ch + 1
			}
		}
	}
	if numTracks == 0 {
		numTracks = 16
	}

	cursor := 0x60
	if cursor+ordNum+2*(insNum+patNum) > len(data) {
		return nil, errors.New("truncated S3M order or parapointer table")
	}

	var orderList []int
	for i := range ordNum {
		ord := data[cursor+i]

		if ord == 255 { // End of song marker
			break
		}

		if ord == 254 {
			orderList = append(orderList, itOrderSkip)
			continue
		}
		if int(ord) >= patNum {
			return nil, errors.New("S3M order references a missing pattern")
		}
		orderList = append(orderList, int(ord))
	}

	if nextITOrder(orderList, 0) == len(orderList) {
		return nil, errors.New("S3M module has no playable orders")
	}
	cursor += ordNum

	insPtrs := make([]int, insNum)
	for i := range insNum {
		insPtrs[i] = int(binary.LittleEndian.Uint16(data[cursor:cursor+2])) * 16
		cursor += 2
	}

	patPtrs := make([]int, patNum)
	for i := range patNum {
		patPtrs[i] = int(binary.LittleEndian.Uint16(data[cursor:cursor+2])) * 16
		cursor += 2
	}
	sharedEffectMemory := tracker != 2 && tracker != 3 && tracker != 4
	if trackerVersion == 0x1320 && string(data[0x36:0x3e]) != "SCLUB2.0" &&
		binary.LittleEndian.Uint16(data[0x3e:0x40]) == 0 && ordNum%2 == 0 &&
		data[0x34] == 0 && binary.LittleEndian.Uint16(data[0x26:0x28])&^uint16(0x50) == 0 &&
		data[0x35] == 0xfc && len(patPtrs) > 0 && len(insPtrs) > 0 && patPtrs[0] > insPtrs[0] {
		sharedEffectMemory = false
	}

	legacyModPlugStereo := trackerVersion == 0x1320 && !sharedEffectMemory && ordNum%16 == 0
	stereo := data[0x33]&0x80 != 0 || legacyModPlugStereo
	var panning [MaxTracks]float32
	var channelVolume [MaxTracks]uint8
	for i := range numTracks {
		channelVolume[i] = 64
	}
	if stereo {
		for i := range numTracks {
			panning[i] = -0.6

			if data[0x40+i]&8 != 0 {
				panning[i] = 0.6
			}
		}
	}

	if data[0x35] == 0xfc {
		if cursor+32 > len(data) {
			return nil, errors.New("truncated S3M panning table")
		}

		for i := range numTracks {
			if pan := data[cursor+i]; pan&0x20 != 0 {
				panning[i] = float32(pan&15)/7.5 - 1
			}
		}
	}

	samples := make([]SampleAsset, insNum)
	var warnings []string
	var gusAddresses uint16
	hasSamples := false
	for i, ptr := range insPtrs {
		sample, warning, err := loadS3MSample(data, ptr, i+1, signedSamples)
		if err != nil {
			return nil, err
		}
		samples[i] = sample
		if ptr > 0 && ptr+0x30 <= len(data) && data[ptr] < 2 {
			gusAddresses |= binary.LittleEndian.Uint16(data[ptr+0x28 : ptr+0x2a])
			hasSamples = hasSamples || binary.LittleEndian.Uint32(data[ptr+0x10:ptr+0x14]) != 0
		}
		if warning != "" {
			warnings = append(warnings, warning)
		}
	}

	patterns := make([]PatternAsset, patNum)
	for i, ptr := range patPtrs {
		var err error
		patterns[i], err = unpackS3MPattern(data, ptr, numTracks)
		if err != nil {
			return nil, fmt.Errorf("S3M pattern %d: %w", i, err)
		}
		if trackerVersion < 0x1320 {
			for _, row := range patterns[i].Rows {
				for ch := range row {
					cmd := &row[ch]
					if cmd.EffectCmd != 0x15 || cmd.EffectArg&0xf0 != 0xa0 {
						continue
					}
					ctype := data[0x40+ch] & 0x7f
					switch arg := cmd.EffectArg & 15; {
					case gusAddresses > 1 || ctype >= 16 || arg > 7:
						cmd.EffectCmd, cmd.s3mDummy = 0, true
					case arg <= 3:
						right := ctype&8 != 0
						if arg&1 != 0 {
							right = !right
						}
						cmd.EffectArg = 0x83
						if right {
							cmd.EffectArg = 0x8c
						}
					default:
						cmd.EffectArg = 0x88
					}
				}
			}
		}
	}

	master := int(data[0x33])
	preamp := max(master&0x7f, 16)
	switch {
	case signedSamples && master < 8:
		preamp = min((master+1)*16, 127)
	case master == 2 || master == 18:
		preamp = 32
	case master&0x7f == 0:
		preamp = 48
	}
	if !stereo {
		preamp = (preamp*8 + 5) / 11
	}
	flags := binary.LittleEndian.Uint16(data[0x26:0x28])
	isST3 := tracker == 1 && string(data[0x36:0x3e]) != "SCLUB2.0"
	if trackerVersion == 0x1320 && (!sharedEffectMemory ||
		data[0x34] == 0 && binary.LittleEndian.Uint16(data[0x3e:0x40]) == 0 &&
			data[0x35] != 0xfc && (flags == 0 || flags == 8)) {
		isST3 = false
	}
	if isST3 && hasSamples && gusAddresses == 0 && trackerVersion != 0x1300 {
		isST3 = false
	}
	if isST3 && gusAddresses > 1 {
		preamp = 48
	}
	return &S3MLoadResult{
		Assets: SongAssets{
			s3m:                   true,
			s3mSharedEffectMemory: sharedEffectMemory,
			s3mMixGain:            float32(preamp) / 128,
			s3mFrequencyPitch:     frequencyPitch,
			OrderList:             orderList,
			Patterns:              patterns,
			Samples:               samples,
			NumTracks:             numTracks,
			Panning:               panning,
			ITGlobalVolume:        min(data[0x30], 64) * 2,
			ITChannelVolume:       channelVolume,
		},
		InitialSpeed: initSpeed,
		InitialTempo: initTempo,
		Warnings:     warnings,
	}, nil
}

func loadS3MSample(data []byte, ptr, number int, signed bool) (SampleAsset, string, error) {
	if ptr == 0 {
		return SampleAsset{}, "", nil
	}
	if ptr+0x50 > len(data) {
		return SampleAsset{}, "", fmt.Errorf("truncated S3M instrument %d", number)
	}
	if data[ptr] != 1 {
		if data[ptr] != 0 {
			return SampleAsset{}, fmt.Sprintf("S3M instrument %d: non-PCM instrument is not supported", number), nil
		}
		return SampleAsset{}, "", nil
	}

	// The 24-bit sample parapointer stores its high byte before its low word.
	memSeg := (int(data[ptr+0x0D]) << 16) | int(binary.LittleEndian.Uint16(data[ptr+0x0E:ptr+0x10]))
	offset := memSeg * 16
	length := binary.LittleEndian.Uint32(data[ptr+0x10 : ptr+0x14])
	flags := data[ptr+0x1F]
	stereo, is16Bit := flags&0x02 != 0, flags&0x04 != 0

	bytesPerSample := uint64(1)
	if is16Bit {
		bytesPerSample = 2
	}
	channels := uint64(1)
	if stereo {
		channels = 2
	}
	if data[ptr+0x1e] != 0 {
		return SampleAsset{}, "", fmt.Errorf("S3M sample %d: packed samples are not supported", number)
	}
	if length != 0 && (offset <= 0 || offset >= len(data) ||
		uint64(length)*bytesPerSample*channels > uint64(len(data)-offset)) {
		return SampleAsset{}, "", fmt.Errorf("truncated S3M sample %d", number)
	}

	sample := SampleAsset{
		Data:       decodeS3MSample(data, offset, int(length), is16Bit, signed),
		LoopStart:  binary.LittleEndian.Uint32(data[ptr+0x14 : ptr+0x18]),
		LoopEnd:    binary.LittleEndian.Uint32(data[ptr+0x18 : ptr+0x1C]),
		LoopEnable: flags&0x01 != 0,
		C5Speed:    binary.LittleEndian.Uint32(data[ptr+0x20 : ptr+0x24]),
		Volume:     min(data[ptr+0x1C], 64),
	}
	if stereo && length != 0 {
		sample.RightData = decodeS3MSample(data, offset+int(uint64(length)*bytesPerSample), int(length), is16Bit, signed)
	}
	return sample, "", nil
}

func decodeS3MSample(fileData []byte, offset, length int, is16Bit, signed bool) []float32 {
	if length == 0 || offset <= 0 || offset >= len(fileData) {
		return nil
	}

	pcm := make([]float32, length)

	if is16Bit {
		for i := 0; i < length && offset+i*2+1 < len(fileData); i++ {
			raw := binary.LittleEndian.Uint16(fileData[offset+i*2 : offset+i*2+2])
			if signed {
				pcm[i] = float32(int16(raw)) / 32768.0
			} else {
				pcm[i] = (float32(raw) - 32768.0) / 32768.0
			}
		}
	} else {
		for i := 0; i < length && offset+i < len(fileData); i++ {
			raw := fileData[offset+i]
			if signed {
				pcm[i] = float32(int8(raw)) / 128.0
			} else {
				pcm[i] = (float32(raw) - 128.0) / 128.0
			}
		}
	}

	return pcm
}

func unpackS3MPattern(fileData []byte, ptr, numTracks int) (PatternAsset, error) {
	rows := make([][]ModCommand, 64)
	for i := range 64 {
		rows[i] = make([]ModCommand, numTracks)
	}

	if ptr == 0 {
		return PatternAsset{Rows: rows}, nil
	}
	if ptr < 0 || ptr+2 > len(fileData) {
		return PatternAsset{}, errors.New("truncated pattern header")
	}

	packedLen := int(binary.LittleEndian.Uint16(fileData[ptr : ptr+2]))
	end := ptr + 2 + packedLen
	if end > len(fileData) {
		return PatternAsset{}, errors.New("truncated packed pattern")
	}
	pos := ptr + 2
	row := 0

	for row < 64 {
		if pos >= end {
			return PatternAsset{}, errors.New("truncated pattern rows")
		}
		flag := fileData[pos]
		pos++

		if flag == 0 {
			row++ // End of row marker
			continue
		}

		var cmd ModCommand

		ch := int(flag & 0x1F)
		if ch < numTracks {
			cmd = rows[row][ch]
		}
		fields := 0
		if flag&0x20 != 0 {
			fields += 2
		}
		if flag&0x40 != 0 {
			fields++
		}
		if flag&0x80 != 0 {
			fields += 2
		}
		if fields > end-pos {
			return PatternAsset{}, errors.New("truncated pattern cell")
		}

		if flag&0x20 != 0 { // Note and Instrument follow
			cmd.Note = decodeS3MNote(fileData[pos])
			cmd.Instrument = fileData[pos+1]
			pos += 2
		}

		if flag&0x40 != 0 { // Volume byte follows
			cmd.Volume = fileData[pos]
			pos++
			cmd.HasVolume = true
		}

		if flag&0x80 != 0 { // Effect command and argument follow
			effCmd, effArg := fileData[pos], fileData[pos+1]
			pos += 2
			cmd.EffectCmd, cmd.EffectArg = mapS3MEffect(effCmd, effArg)
		}

		if ch < numTracks {
			rows[row][ch] = cmd
		}
	}

	return PatternAsset{Rows: rows}, nil
}

func decodeS3MNote(b byte) uint8 {
	switch b {
	case 255:
		return 0 // Empty note
	case 254:
		return 254 // Note Cut (^^)
	default:
		// High nibble = octave (0..9), Low nibble = semitone (0..11)
		octave := (b >> 4) & 0x0F
		semi := b & 0x0F
		return octave*12 + semi + 12 // C-4 (0x40) -> 60 (matches C5Speed)
	}
}

// Maps S3M letter commands ('A'=1, 'B'=2, ...) to the ProTracker-style hex codes used in our systems
func mapS3MEffect(s3mCmd, arg byte) (uint8, uint8) {
	switch s3mCmd {
	case 1: // 'A' - Set Speed
		return 0x0F, arg
	case 2: // 'B' - Order Jump
		return 0x0B, arg
	case 3: // 'C' - Pattern Break
		return 0x0D, arg
	case 4: // 'D' - Volume Slide
		return 0x0A, arg
	case 5: // 'E' - Portamento Down
		return 0x02, arg
	case 6: // 'F' - Portamento Up
		return 0x01, arg
	case 7: // 'G' - Tone Portamento
		return 0x03, arg
	case 8: // 'H' - Vibrato
		return 0x04, arg
	case 9: // 'I' - Tremor
		return 0x22, arg
	case 10: // 'J' - Arpeggio
		return 0x10, arg
	case 11: // 'K' - Vibrato and volume slide
		return 0x11, arg
	case 12: // 'L' - Tone portamento and volume slide
		return 0x1f, arg
	case 13: // 'M' - Set channel volume
		return 0x16, arg
	case 14: // 'N' - Channel volume slide
		return 0x1e, arg
	case 15: // 'O' - Sample offset
		return 0x12, arg
	case 16: // 'P' - Panning slide
		return 0x1d, arg
	case 17: // 'Q' - Retrigger and volume slide
		return 0x13, arg
	case 18: // 'R' - Tremolo
		return 0x23, arg
	case 19: // 'S' - Special
		return 0x15, arg
	case 20: // 'T' - Set Tempo or slide
		return 0x20, arg
	case 21: // 'U' - Fine vibrato
		return 0x21, arg
	case 22: // 'V' - Set global volume
		return 0x17, arg
	case 23: // 'W' - Global volume slide
		return 0x1c, arg
	case 24: // 'X' - Set Panning
		return 0x08, arg
	case 25: // 'Y' - Panbrello
		return 0x18, arg
	default:
		return 0, 0
	}
}
