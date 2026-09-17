// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package voice

type Flags uint32

const (
	Voice16Bit Flags = 1 << iota
	VoiceLoop
	VoiceLoopBounce
	VoiceSustain
	VoiceSustainBounce
	VoicePanning Flags = 1 << iota
	VoiceStereo
	VoiceBounce
	VoiceMute Flags = 1 << iota
	VoiceKeyOff
	VoiceFadeOut
	VoiceSurround
	VoiceNearest
	VoiceHQSource Flags = 1 << iota
	VoiceFiltering
	VoiceVolumeRamp
	VoiceVibrato Flags = 1 << iota
	VoiceTremolo
	VoicePanbrello
	VoicePortamento
	VoiceGlissando
	VoiceVolumeEnv Flags = 1 << iota
	VoicePanEnv
	VoicePitchEnv
	VoiceFastVolumeRamp
	VoiceExtraLoudness Flags = 1 << iota
	VoiceReverb
	VoiceExtraReverb
	VoiceHQResampling Flags = 1 << iota
)

func (f Flags) Has(x Flags) bool {
	return f&x != 0
}

func (f *Flags) Set(x Flags, on bool) {
	*f &^= x

	if on {
		*f |= x
	}
}
