// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

func itHasVolumeTonePortamento(cmd ModCommand) bool {
	return cmd.HasVolume && cmd.Volume >= 193 && cmd.Volume <= 202
}

func itHasTonePortamento(cmd ModCommand) bool {
	return cmd.EffectCmd == 0x03 || cmd.EffectCmd == 0x1f || itHasVolumeTonePortamento(cmd)
}

func itVolumePortamentoSpeed(volume uint8) uint8 {
	speeds := [...]uint8{0, 1, 4, 8, 16, 32, 64, 96, 128, 255}
	return speeds[volume-193]
}

func (compat ITCompatibility) itSharesPortamentoMemory() bool {
	return !compat.LegacyModPlug && !compat.CompatibleGxx && !compat.mptSeparatePortamentoMemory
}

func initITTonePortamentoMemory(w *Tracker, mem *EffectMemory, speed uint8) {
	switch {
	case speed != 0:
		mem.LastPortaSpeed = speed
		if w.Assets.ITCompatibility.itSharesPortamentoMemory() {
			mem.LastPortaUp, mem.LastPortaDown = speed, speed
		}
	case w.Assets.ITCompatibility.itSharesPortamentoMemory():
		mem.LastPortaSpeed = mem.LastPortaUp
	}
}
