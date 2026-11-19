// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

func (w *Tracker) updateS3MPanbrello() {
	for t := 0; t < w.Assets.NumTracks; t++ {
		mem := &w.TrackMemories[t]
		cmd := w.PendingCmds[t]
		if cmd.EffectCmd == 0x18 && cmd.EffectArg&0xf0 != 0 {
			mem.itPanbrello = mem.itPanbrello&15 | cmd.EffectArg&0xf0
		}
		if cmd.EffectCmd == 0x18 && cmd.EffectArg&15 != 0 {
			mem.itPanbrello = mem.itPanbrello&0xf0 | cmd.EffectArg&15
		}
		voice := w.TrackCursors[t].ActiveVoice
		if voice == -1 || !w.VoiceLives[voice].Active {
			continue
		}
		amp := &w.VoiceAmps[voice]
		amp.itPanbrelloOffset = 0
		if cmd.EffectCmd != 0x18 {
			continue
		}
		pos := uint8(((int(mem.itPanbrelloPos) + 16) >> 2) & 63)
		delta := itEffectWaveform(w, mem.itPanbrelloWaveform, pos, false, true)
		amp.itPanbrelloOffset = float32((delta*int(mem.itPanbrello&15)+2)/8) / 128
		mem.itPanbrelloPos += mem.itPanbrello >> 4
	}
}
