package mod

func isMPTCueOffset(cmd ModCommand) bool {
	return cmd.HasVolume && cmd.Volume >= 224 && cmd.Volume <= 232
}
