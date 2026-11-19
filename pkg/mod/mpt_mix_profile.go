// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import "fmt"

// mptMixProfile covers sample/OPL mixer scaling and PCM panning.
type mptMixProfile struct {
	pcmGain     float32
	oplGain     float32
	softPanning bool
}

func projectMPTMixProfile(mixLevels, samplePreamp, pluginVolume uint32) (mptMixProfile, error) {
	if mixLevels != 3 && mixLevels != 4 {
		return mptMixProfile{}, fmt.Errorf("unsupported MPTM mix level %d", mixLevels)
	}

	if samplePreamp > 128 || pluginVolume > 128 {
		return mptMixProfile{}, fmt.Errorf("unsupported MPTM preamp values %d and %d", samplePreamp, pluginVolume)
	}
	// OpenMPT scales OPL output by floor(6169 * pluginVolume / 48)
	// into a 2^27 mixer; this renderer converts the same int16 output by 2^15.
	oplFactor := 6169 * pluginVolume / 48
	return mptMixProfile{pcmGain: float32(samplePreamp) / 128, oplGain: float32(oplFactor) / 4096, softPanning: mixLevels == 3}, nil
}
