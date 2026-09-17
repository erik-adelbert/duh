package s3m

import (
	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/help"
	mod "github.com/erik-adelbert/duh/internal/tracker/components/module"
	"github.com/erik-adelbert/duh/internal/tracker/entities"
)

func (s3m *S3M) RegisterTo(w *ecs.World) {
	var (
		m entities.Module
		// ords []entities.Order
		// pats []entities.Pattern
		// smps []entities.Sample
		// inss []entities.Instrument
		// vocs []entities.Voice
		// plgs []entities.MixPlugin
	)

	m.Format = mod.FormatS3M

	m.Name = asString(s3m.Name[:])
	// TODO: check all the spagetttis fell into the right hole here
	m.Metadata.Speed = uint32(help.Clamp(s3m.DefaultSpeed, MinSpeed, MaxSpeed))
	m.Metadata.Tempo = uint32(help.Clamp(s3m.DefaultTempo, MinTempo, MaxTempo))

	// volume := help.Clamp(uint16(s3m.GlobalVolume<<2), MinVolume, MaxVolume)
	// if volume == 0 {
	// 	volume = MaxVolume
	// }

	// preAmp := uint(s3m.SongPreAmp & VolumeMask)

	if s3m.Creator < st320 && has(s3m.Flags, st3VolSlides) { // old Scream Tracker 3 version
		m.Flags |= mod.FastSlides
	}

	if has(s3m.Flags, st3AmigaLimits) { // stereo
		m.Flags |= mod.AmigaLimits
	}

	voiceMap := make(map[int]uint8)

	for i, s := range s3m.ChanSettings {
		if s != unusedCh {
			voiceMap[i] = s
		}
	}

	numVoices := max(4, len(voiceMap)) // at least 4 channels
	// Voices := make([]AudioChan, numVoices)
	VoiceSettings := make([]AudioSetting, numVoices)

	for i := range VoiceSettings {
		var (
			pan      = uint32(0x80) // default center pan
			muteFlag = AudioMute    // muted by default
		)

		// default pan and unmute if this channel is used
		if setting, ok := voiceMap[i]; ok {
			pan = 0x40 // default left

			if (setting&0xF)&8 != 0 { // right
				pan = 0xC0
			}

			if setting&0x80 == 0 {
				muteFlag = 0 // unmute
			}
		}

		if s3m.Panning == customPanning && s3m.Pannings[i]&0x20 != 0 {
			pan = ((uint32(s3m.Pannings[i] & 0xF)) << 4) + 8
			muteFlag = 0 // unmute
		}

		VoiceSettings[i] = AudioSetting{
			Flags: muteFlag,
			Pan:   pan,
			Vol:   64, // default volume
		}
	}

	// orders
	// numOrders := clamp(len(s3m.Orders), 1, MaxNumOrders)

	// Orders := clone(s3m.Orders, numOrders)

	// samples
	numSamples := help.Clamp(len(s3m.Instruments), 1, MaxNumInstrs)
	Samples := make([]*Sample, numSamples)

	for i := range Samples {
		Samples[i] = s3m.Instruments[i].AsSampleFrom(s3m.Format)
	}

	// patterns
	numPatterns := help.Clamp(len(s3m.Patterns), 1, MaxNumPatterns)
	Patterns := make([]Pattern, numPatterns)

	for i, pat := range s3m.Patterns {
		Patterns[i] = make(Pattern, len(pat))

		for j, row := range pat {
			Patterns[i][j] = make([]Event, numVoices)

			for _, e := range row { // events

				k := int(e.ChanID)

				if k >= numVoices {
					continue // invalid channel, ignore
				}

				note := e.Note

				switch {
				case note == 0xFF:
					note = 0
				case note < 0xF0:
					note = (note & 0xF) + 12*(note>>4) + 13 // convert to internal note range
				}

				volCmd := VCmdVolume
				vol := e.Vol

				if vol >= 0x80 && vol <= 0xC0 {
					vol -= 0x80
					volCmd = VCmdPanning
				}

				vol = help.Clamp(vol, 0, 0x40)
				cmd, param := CmdFromS3M(e.Cmd, false)
				Patterns[i][j][k] = Event{
					Note:   note,
					Instr:  e.InstrID,
					Vol:    vol,
					VolCmd: volCmd,
					Cmd:    cmd,
					Param:  param,
				}
			}
		}
	}
}
