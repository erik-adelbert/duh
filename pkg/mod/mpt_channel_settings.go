// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

func applyMPTExtraChannelSettings(assets *SongAssets, channels int, settings []byte) {
	for channel := itHeaderChannels; channel < channels; channel++ {
		assets.ITChannelVolume[channel] = 64
	}

	for i := 0; i < len(settings)/2; i++ {
		pan, volume := settings[i*2], settings[i*2+1]
		if pan == 0xff {
			continue
		}

		channel := itHeaderChannels + i
		assets.ITMuted[channel] = pan&0x80 != 0

		pan &= 0x7f
		if pan <= 64 {
			assets.Panning[channel] = float32(pan)/32 - 1
		}

		assets.ITSurround[channel] = pan == 100
		assets.ITChannelVolume[channel] = volume
	}
}
