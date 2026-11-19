package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestPCMGainRamp(t *testing.T) {
	for _, rate := range []float64{44100, 48000, 96000} {
		t.Run(fmt.Sprintf("%.0f", rate), func(t *testing.T) {
			frames := int(math.Round(rate / 1000))
			w := NewTracker(SongAssets{Samples: []SampleAsset{{Data: []float32{1, 1}}}}, rate, frames+1)
			w.VoiceLives[0].Active = true
			amp := &w.VoiceAmps[0]
			amp.TargetLeft, amp.TargetRight = 1, -0.5
			for i := range w.VoiceSamples[0].ScratchBuf {
				w.VoiceSamples[0].ScratchBuf[i] = 1
			}
			out := make([]float32, (frames+1)*2)
			MixerAccumulationSystem(w, out, frames+1)
			for i := 0; i <= frames; i++ {
				want := float32(i) / float32(frames)
				if math.Abs(float64(out[2*i]-want)) > 1e-5 || math.Abs(float64(out[2*i+1]+want/2)) > 1e-5 {
					t.Fatalf("frame %d: got %g, %g, want %g, %g", i, out[2*i], out[2*i+1], want, -want/2)
				}
			}
			if amp.CurrentLeft != 1 || amp.CurrentRight != -0.5 || amp.rampRemaining != 0 {
				t.Fatalf("ramp did not finish: %+v", *amp)
			}
		})
	}
}

func TestPCMGainRampRetarget(t *testing.T) {
	w := NewTracker(SongAssets{Samples: []SampleAsset{{Data: []float32{1, 1}}}}, 48000, 64)
	w.VoiceLives[0].Active = true
	for i := range w.VoiceSamples[0].ScratchBuf {
		w.VoiceSamples[0].ScratchBuf[i] = 1
	}
	amp := &w.VoiceAmps[0]
	amp.TargetLeft, amp.TargetRight = 1, 0.5
	out := make([]float32, 128)
	MixerAccumulationSystem(w, out, 24)
	left, right := amp.CurrentLeft, amp.CurrentRight
	amp.TargetLeft, amp.TargetRight = -1, 1
	MixerAccumulationSystem(w, out, 48)
	if out[0] != left || out[1] != right {
		t.Fatalf("retarget discontinuity: got %g, %g, want %g, %g", out[0], out[1], left, right)
	}
	if amp.CurrentLeft != -1 || amp.CurrentRight != 1 || amp.rampRemaining != 0 {
		t.Fatalf("retarget did not finish: %+v", *amp)
	}
	if got := testing.AllocsPerRun(100, func() {
		amp.TargetLeft = -amp.TargetLeft
		MixerAccumulationSystem(w, out, 48)
	}); got != 0 {
		t.Fatalf("gain ramp allocated %g times per block", got)
	}
}

func TestPCMGainRampDuration(t *testing.T) {
	amp := AmplitudeState{CurrentLeft: 1, TargetRight: 1, itPanbrelloOffset: 0.5}
	amp.updatePCMRampDuration(48000, 960, true)
	if amp.rampDuration != 960 || amp.rampPanbrello != 0.5 {
		t.Fatalf("active panbrello: %+v", amp)
	}
	amp.itPanbrelloOffset = 0
	amp.updatePCMRampDuration(48000, 960, true)
	if amp.rampDuration != 960 || amp.rampPanbrello != 0 {
		t.Fatalf("ending panbrello: %+v", amp)
	}
	amp.updatePCMRampDuration(48000, 960, true)
	if amp.rampDuration != 48 {
		t.Fatalf("ordinary transition: %+v", amp)
	}
	amp.itSmoothRamp = true
	amp.updatePCMRampDuration(48000, 960, true)
	if amp.rampDuration != 960 {
		t.Fatalf("effect transition: %+v", amp)
	}
	amp.itSmoothRamp = false
	amp.itPanbrelloOffset = 0.5
	amp.CurrentLeft = 0
	amp.updatePCMRampDuration(48000, 960, true)
	if amp.rampDuration != 48 {
		t.Fatalf("silent voice attack: %+v", amp)
	}
	amp.CurrentLeft = 1
	amp.updatePCMRampDuration(48000, 960, false)
	if amp.rampDuration != 48 {
		t.Fatalf("non-IT transition: %+v", amp)
	}
}

func TestPCMGainRampDoesNotRestartForUnchangedTarget(t *testing.T) {
	w := NewTracker(SongAssets{Samples: []SampleAsset{{Data: []float32{1, 1}}}}, 48000, 64)
	w.VoiceLives[0].Active = true
	amp := &w.VoiceAmps[0]
	amp.TargetLeft, amp.rampDuration = 1, 48
	out := make([]float32, 96)
	MixerAccumulationSystem(w, out, 24)
	amp.rampDuration = 960
	MixerAccumulationSystem(w, out, 24)
	if amp.CurrentLeft != 1 || amp.rampRemaining != 0 {
		t.Fatalf("unchanged target restarted ramp: %+v", amp)
	}
}
