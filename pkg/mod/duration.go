// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import (
	"errors"
	"os"
	"time"
)

const (
	durationMaxTicks = 2000000
	durationMaxRows  = 100000
)

// DurationEstimator simulates tracker timing without rendering audio. Its
// result is the time to the end or the first repeated playback state, even
// when the decoder is configured to repeat. Create one with
// Decoder.NewDurationEstimator; it is not safe for concurrent use.
type DurationEstimator struct {
	tracker *Tracker
	frames  int64
	ticks   int
	done    bool
	err     error
}

// NewDurationEstimator creates an independent estimate from the beginning of
// the decoder's selected sequence. It does not change the decoder's position.
func (d *Decoder) NewDurationEstimator() (*DurationEstimator, error) {
	if d.tracker == nil {
		return nil, os.ErrClosed
	}
	w := NewTracker(d.tracker.Assets, decoderSampleRate, 0)
	w.mptVisitedRows = make(map[mptVisitedRow]struct{})
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = d.initialSpeed, d.initialTempo, -1
	return &DurationEstimator{tracker: w}, nil
}

// Advance processes at most maxTicks ticks. Duration is valid only when done
// is true and err is nil. A complexity limit returns an error rather than an
// incomplete estimate.
func (e *DurationEstimator) Advance(maxTicks int) (duration time.Duration, done bool, err error) {
	if maxTicks <= 0 {
		return 0, false, errors.New("module duration: tick budget must be positive")
	}
	if e.err != nil {
		return 0, false, e.err
	}
	if e.done {
		return e.duration(), true, nil
	}
	w := e.tracker
	if w == nil {
		return 0, false, errors.New("module duration: estimator is not initialized")
	}
	for range maxTicks {
		if !TickSchedulerSystem(w) {
			if w.Clock.SongFinished {
				e.done = true
				e.tracker = nil
				return e.duration(), true, nil
			}
			e.err = errors.New("module duration: scheduler did not advance")
			e.tracker = nil
			return 0, false, e.err
		}
		if e.ticks >= durationMaxTicks || len(w.mptVisitedRows) > durationMaxRows {
			e.err = errors.New("module duration: complexity limit exceeded")
			e.tracker = nil
			return 0, false, e.err
		}
		PatternFetchSystem(w)
		GlobalFlowSystem(w)
		if w.Assets.IT && w.Clock.Tick == 0 {
			for t := 0; t < w.Assets.NumTracks; t++ {
				cmd := w.PendingCmds[t]
				if cmd.EffectCmd == 0x15 {
					w.rememberITSpecial(TrackID(t), cmd)
				}
			}
		}
		e.frames += int64(w.Clock.SamplesRemaining)
		w.Clock.SamplesRemaining = 0
		e.ticks++
	}
	return 0, false, nil
}

func (e *DurationEstimator) duration() time.Duration {
	return time.Duration(e.frames/decoderSampleRate)*time.Second +
		time.Duration(e.frames%decoderSampleRate)*time.Second/decoderSampleRate
}
