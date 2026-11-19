package mod

import "math"

// System 5: Resamples active voices into their isolated scratch buffers.
func ResamplerSystem(w *Tracker, frames int) {
	for v := range MaxVoices {
		if !w.VoiceLives[v].Active {
			continue
		}

		pb := &w.VoiceSamples[v]
		if pb.oplChannel != 0 {
			continue
		}

		if pb.SampleID < 0 || pb.SampleID >= len(w.Assets.Samples) {
			w.stopVoice(VoiceID(v))
			continue
		}

		sample := &w.Assets.Samples[pb.SampleID]

		dataLen := len(sample.Data)
		if dataLen < 2 {
			w.stopVoice(VoiceID(v))
			continue
		}

		step := w.VoicePitches[v].StepIncrement
		pos := pb.Position
		reverse := pb.Reverse
		buf := pb.ScratchBuf[:frames]
		stereo := len(sample.RightData) != 0

		var right []float32
		if stereo {
			right = pb.RightScratchBuf[:frames]
		}

		// Clamp loop boundaries to actual PCM slice bounds
		loopStart := int(sample.LoopStart)
		loopEnd := int(sample.LoopEnd)
		loopEnabled, pingPong := sample.LoopEnable, sample.LoopPingPong

		if sample.mptSustainLoop && !w.VoiceLives[v].KeyOff && !w.VoiceLives[v].mptSustainSuppressed {
			loopStart, loopEnd = int(sample.mptSustainStart), int(sample.mptSustainEnd)
			loopEnabled, pingPong = true, sample.itSustainPingPong || w.VoiceLives[v].mptSustainInheritPingPong
		}

		if loopEnd > dataLen {
			loopEnd = dataLen
		}

		hasValidLoop := loopEnabled && loopEnd > loopStart && loopStart >= 0
		sinc, nearest, cubic := w.Assets.mptSincResampling, w.Assets.mptNearestResampling, w.Assets.mptCubicResampling

		if instrument := w.VoiceLives[v].ITInstrument; instrument > 0 && instrument <= len(w.Assets.ITInstruments) {
			sinc = w.Assets.ITInstruments[instrument-1].mptSincResampling
			nearest = w.Assets.ITInstruments[instrument-1].mptNearestResampling
			cubic = w.Assets.ITInstruments[instrument-1].mptCubicResampling
		}
		mptNearestPingPong := nearest && pingPong && w.Assets.ITCompatibility.mptMixProfile != nil

		if mptNearestPingPong {
			// OpenMPT advances loop positions with a 32-bit fractional increment.
			step = math.Floor(step*4294967296) / 4294967296
		}
		table := 0

		if step > 1 {
			table = 1
		}

		if step > 1.5 {
			table = 2
		}

		for i := range frames {
			if hasValidLoop {
				if pingPong {
					// IT and old ModPlug use different turnaround points at the loop end.
					if reverse && pos < float64(loopStart) {
						pos = 2*float64(loopStart) - pos
						if math.Floor(pos) >= float64((loopStart+loopEnd)/2) {
							pos = float64(loopStart)
						}
						reverse = false
					} else if !reverse && pos >= float64(loopEnd) {
						pb.mptLooped = true
						overshoot := pos - float64(loopEnd)

						diff := 1
						if w.Assets.ITCompatibility.LegacyModPlug {
							diff = 0
						}

						if mptNearestPingPong {
							diff = 0
							if w.Assets.ITCompatibility.mptPingPongMode {
								diff = 1
							}
						}

						if nearest && w.Assets.ITCompatibility.mptImprecisePingPong {
							pos = mptImprecisePingPongTurn(pos, loopStart, loopEnd, diff)
						} else if overshoot < float64(loopEnd-loopStart-diff) ||
							mptNearestPingPong && math.Floor(overshoot) < float64(loopEnd-loopStart-diff) {
							pos = float64(loopEnd-diff) - overshoot
						} else {
							pos = float64(loopStart)
						}

						reverse = true
					}

					idx := int(pos)
					frac := float32(pos - float64(idx))
					idx = min(idx, loopEnd-1)

					nextIdx := idx + 1
					if nextIdx >= loopEnd {
						nextIdx = loopEnd - 1
						// IT's interpolation lookahead mirrors the penultimate sample.
						if !w.Assets.ITCompatibility.LegacyModPlug && loopEnd-loopStart > 1 {
							nextIdx--
						}
					}

					buf[i] = sample.Data[idx] + frac*(sample.Data[nextIdx]-sample.Data[idx])

					if stereo {
						right[i] = sample.RightData[idx] + frac*(sample.RightData[nextIdx]-sample.RightData[idx])
					}

					if nearest {
						nearestIdx := int(pos + .5)
						if nearestIdx >= loopEnd {
							nearestIdx = loopEnd - 1
							if w.Assets.ITCompatibility.mptPingPongLookahead && loopEnd-loopStart > 1 {
								nearestIdx--
							}
						}
						buf[i] = sample.Data[nearestIdx]
						if stereo {
							right[i] = sample.RightData[nearestIdx]
						}
					}

					if sinc {
						coeff := mptSincCoefficients(pos, table)
						buf[i] = mptSincSample(sample.Data, pos, coeff, loopStart, loopEnd, true, pb.mptLooped)
						if stereo {
							right[i] = mptSincSample(sample.RightData, pos, coeff, loopStart, loopEnd, true, pb.mptLooped)
						}
					}

					if reverse {
						pos -= step
					} else {
						pos += step
					}

					continue
				}

				// Wrap position cleanly within [loopStart, loopEnd)
				if reverse && pos < float64(loopStart) {
					reflected := 2*float64(loopStart) - pos
					if math.Floor(reflected) >= float64((loopStart+loopEnd)/2) {
						reflected = float64(loopStart)
					}
					pos = float64(loopEnd-1) + reflected - math.Floor(reflected)
					pb.mptLooped = true
				}

				if pos >= float64(loopEnd) {
					pb.mptLooped = true
					loopLen := float64(loopEnd - loopStart)
					pos = float64(loopStart) + math.Mod(pos-float64(loopStart), loopLen)
				}

				idx := int(pos)
				if idx >= loopEnd {
					idx = loopStart
				}

				nextIdx := idx + 1
				if nextIdx >= loopEnd {
					nextIdx = loopStart // Wrap interpolation sample back to loop start
				}

				if !cubic {
					frac := float32(pos - float64(idx))
					s0 := sample.Data[idx]
					s1 := sample.Data[nextIdx]
					buf[i] = s0 + frac*(s1-s0)
					if stereo {
						right[i] = sample.RightData[idx] + frac*(sample.RightData[nextIdx]-sample.RightData[idx])
					}
				}

				if nearest {
					nearestIdx := int(pos + .5)
					if nearestIdx >= loopEnd {
						nearestIdx = loopStart
					}
					buf[i] = sample.Data[nearestIdx]
					if stereo {
						right[i] = sample.RightData[nearestIdx]
					}
				}

				if sinc {
					coeff := mptSincCoefficients(pos, table)
					buf[i] = mptSincSample(sample.Data, pos, coeff, loopStart, loopEnd, false, pb.mptLooped)
					if stereo {
						right[i] = mptSincSample(sample.RightData, pos, coeff, loopStart, loopEnd, false, pb.mptLooped)
					}
				}

				if cubic {
					buf[i] = mptCubicSample(sample.Data, pos, loopStart, loopEnd, pb.mptLooped)
					if stereo {
						right[i] = mptCubicSample(sample.RightData, pos, loopStart, loopEnd, pb.mptLooped)
					}
				}

				if reverse {
					pos -= step
				} else {
					pos += step
				}
			} else {
				// One-shot (non-looping) sample
				idx := int(math.Floor(pos))
				if idx < 0 || idx >= dataLen || !reverse && !sinc && !nearest && !cubic && idx >= dataLen-1 {
					if !reverse && !sinc && !nearest && !cubic && idx == dataLen-1 {
						buf[i] = sample.Data[idx]
						if stereo {
							right[i] = sample.RightData[idx]
						}
						i++
					}
					pb.Ended = true
					for j := i; j < frames; j++ {
						buf[j] = 0
						if stereo {
							right[j] = 0
						}
					}
					break
				}

				if !cubic {
					frac := float32(pos - float64(idx))
					s0 := sample.Data[idx]
					nextIdx := min(idx+1, dataLen-1)
					s1 := sample.Data[nextIdx]
					buf[i] = s0 + frac*(s1-s0)
					if stereo {
						right[i] = sample.RightData[idx] + frac*(sample.RightData[nextIdx]-sample.RightData[idx])
					}
				}

				if nearest {
					nearestIdx := min(int(pos+.5), dataLen-1)
					buf[i] = sample.Data[nearestIdx]
					if stereo {
						right[i] = sample.RightData[nearestIdx]
					}
				}

				if sinc {
					coeff := mptSincCoefficients(pos, table)
					buf[i] = mptSincSample(sample.Data, pos, coeff, 0, 0, false, false)
					if stereo {
						right[i] = mptSincSample(sample.RightData, pos, coeff, 0, 0, false, false)
					}
				}

				if cubic {
					buf[i] = mptCubicSample(sample.Data, pos, 0, 0, false)
					if stereo {
						right[i] = mptCubicSample(sample.RightData, pos, 0, 0, false)
					}
				}

				if reverse {
					pos -= step
				} else {
					pos += step
				}
			}
		}

		pb.Position = pos
		pb.Reverse = reverse
		applyITVoiceFilter(&w.VoiceFilters[v], buf)
		if stereo {
			applyITVoiceFilter(&w.VoiceRightFilters[v], right)
		}
	}
}

func applyITVoiceFilter(filter *ITFilterState, buf []float32) {
	if !filter.Enabled {
		return
	}

	for i, sample := range buf {
		y := float64(sample)*filter.A0 + math.Max(-2, math.Min(2, filter.Y1))*filter.B0 + math.Max(-2, math.Min(2, filter.Y2))*filter.B1
		filter.Y2, filter.Y1 = filter.Y1, y
		if filter.HighPass {
			filter.Y1 -= float64(sample)
		}
		buf[i] = float32(y)
	}
}

// System 6: Applies de-clicking volume ramps and mixes all voices into stereo output.
func MixerAccumulationSystem(w *Tracker, outStereo []float32, frames int) {
	// Zero the output slice chunk
	for i := range frames * 2 {
		outStereo[i] = 0
	}

	mixPCMVoices(w, outStereo, frames)

	if w.opl != nil {
		buf := w.oplBuffer[:frames*2]
		w.opl.render(buf)

		gain := float32(1)
		if profile := w.Assets.ITCompatibility.mptMixProfile; profile != nil {
			gain = profile.oplGain
		}

		for i, sample := range buf {
			outStereo[i] += float32(sample) / 32768 * gain
		}
	}

	if w.Assets.IT && w.Assets.ITCompatibility.mptMixProfile != nil {
		gain := float32(w.ITGlobalVolume) / 128
		for i := 0; i < frames*2; i++ {
			outStereo[i] *= gain
		}
	}
}

func pcmGainRampFrames(sampleRate float64) int {
	return max(1, int(math.Round(sampleRate/1000)))
}

func (amp *AmplitudeState) updatePCMRampDuration(sampleRate float64, tickFrames int, it bool) {
	amp.rampDuration = pcmGainRampFrames(sampleRate)
	if it && (amp.itPanbrelloOffset != 0 || amp.rampPanbrello != 0 || amp.itEnvelopeRamping || amp.itSmoothRamp) &&
		(amp.CurrentLeft != 0 || amp.CurrentRight != 0) &&
		(amp.TargetLeft != 0 || amp.TargetRight != 0) {
		amp.rampDuration = max(amp.rampDuration, tickFrames)
	}
	amp.rampPanbrello = amp.itPanbrelloOffset
}

func mixPCMVoices(w *Tracker, outStereo []float32, frames int) {
	for v := range MaxVoices {
		if !w.VoiceLives[v].Active || w.VoiceSamples[v].oplChannel != 0 {
			continue
		}

		amp := &w.VoiceAmps[v]
		buf := w.VoiceSamples[v].ScratchBuf[:frames]
		var right []float32
		if len(w.Assets.Samples[w.VoiceSamples[v].SampleID].RightData) != 0 {
			right = w.VoiceSamples[v].RightScratchBuf[:frames]
		}

		if amp.TargetLeft != amp.rampTargetLeft || amp.TargetRight != amp.rampTargetRight {
			amp.rampTargetLeft, amp.rampTargetRight = amp.TargetLeft, amp.TargetRight
			amp.rampRemaining = amp.rampDuration
			if amp.rampRemaining == 0 {
				amp.rampRemaining = pcmGainRampFrames(w.Clock.SampleRate)
			}
			amp.rampStepLeft = (amp.TargetLeft - amp.CurrentLeft) / float32(amp.rampRemaining)
			amp.rampStepRight = (amp.TargetRight - amp.CurrentRight) / float32(amp.rampRemaining)
		}

		gainL := amp.CurrentLeft
		gainR := amp.CurrentRight
		remaining := amp.rampRemaining
		for i := range frames {
			left := buf[i]
			rightSample := left
			if right != nil {
				rightSample = right[i]
			}

			outStereo[i*2] += left * gainL
			outStereo[i*2+1] += rightSample * gainR

			if remaining > 0 {
				gainL += amp.rampStepLeft
				gainR += amp.rampStepRight

				remaining--
				if remaining == 0 {
					gainL, gainR = amp.TargetLeft, amp.TargetRight
				}
			}
		}

		amp.CurrentLeft, amp.CurrentRight = gainL, gainR
		amp.rampRemaining = remaining
		if w.VoiceSamples[v].Ended {
			w.stopVoice(VoiceID(v))
		}
	}
}

// Read is the public entry point equivalent to openmpt_module_read_interleaved_float_stereo.
func (w *Tracker) Read(outStereo []float32, requestedFrames int) int {
	framesRendered := 0

	for framesRendered < requestedFrames && !w.Clock.SongFinished {
		// 1. Run control-rate systems if a new tick begins
		if TickSchedulerSystem(w) {
			if w.Clock.SongFinished {
				break
			}
			PatternFetchSystem(w)
			GlobalFlowSystem(w)
			VoiceAllocationSystem(w)
			PitchEffectSystem(w)
			VolumeEffectSystem(w)
			MPTCutoffMacroSystem(w)
			VoiceLifecycleSystem(w)
			VoiceKinematicsSystem(w)
		}

		// 2. Determine sample chunk size up to next tick boundary
		chunk := min(w.Clock.SamplesRemaining, requestedFrames-framesRendered)

		// 3. Run audio-rate DSP systems
		ResamplerSystem(w, chunk)
		sliceStart := framesRendered * 2
		sliceEnd := (framesRendered + chunk) * 2
		MixerAccumulationSystem(w, outStereo[sliceStart:sliceEnd], chunk)

		w.Clock.SamplesRemaining -= chunk
		framesRendered += chunk
	}

	return framesRendered
}
