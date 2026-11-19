package mod

import "fmt"

func applyMPTVolumeReleaseNodes(instruments []ITInstrument, nodes []uint8, version uint32) error {
	for i, node := range nodes {
		if node == 0xff || i >= len(instruments) {
			continue
		}

		instrument := &instruments[i]
		env := &instrument.VolumeEnvelope
		if len(env.Points) == 0 && instrument.itDisabledEnvData[0][1] != 0 {
			raw := instrument.itDisabledEnvData[0]
			raw[0] |= 1
			parsed, err := parseITEnvelope(raw[:], false)
			if err != nil {
				return fmt.Errorf("MPTM instrument %d volume release envelope: %w", i+1, err)
			}

			parsed.Enabled = false
			*env = parsed
		}

		if len(env.Points) == 0 || int(node) >= len(env.Points) {
			return fmt.Errorf("MPTM instrument %d has invalid volume-envelope release node %d", i+1, node)
		}

		if version < 0x01280043 {
			return fmt.Errorf("MPTM instrument %d uses a volume-envelope release node with unsupported pre-1.28.00.43 playback", i+1)
		}

		instrument.mptHasVolumeRelease = true
		instrument.mptVolumeReleaseNode = node
	}

	return nil
}

func mptReleasedVolume(env *ITVolumeEnvelope, node uint8, tick, valueAtJump int) float32 {
	point := env.Points[node]

	releaseValue := int(point.Value) * 4
	if releaseValue == 0 {
		return 0
	}

	current := mptVolumeEnvelopeValue(env, tick)
	if tick == int(point.Tick) {
		current = releaseValue
	}

	return float32(min(512, valueAtJump*current/releaseValue)) / 256
}

func mptVolumeEnvelopeValue(env *ITVolumeEnvelope, tick int) int {
	points := env.Points
	target := len(points) - 1
	for i := 0; i < len(points)-1; i++ {
		if tick <= int(points[i].Tick) {
			target = i
			break
		}
	}

	point := points[target]
	if tick >= int(point.Tick) {
		return int(point.Value) * 4
	}

	start, value := 0, 0
	if target > 0 {
		previous := points[target-1]
		start, value = int(previous.Tick), int(previous.Value)*1024
	}
	if tick > start && int(point.Tick) > start {
		value += (tick - start) * (int(point.Value)*1024 - value) / (int(point.Tick) - start)
	}

	return (value*256 + 32768) / 65536
}
