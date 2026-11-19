package mod

type itPatternLoopProfile struct {
	resetTarget     bool
	allowNested     bool
	preserveOnBreak bool
	jumpPriority    itLoopJumpPriority
}

type itLoopJumpPriority uint8

const (
	itLoopWins itLoopJumpPriority = iota
	itJumpAlwaysWins
	itJumpRightWins
)

func (w *Tracker) loopProfile() itPatternLoopProfile {
	if profile := w.Assets.ITCompatibility.itPatternLoopProfile; profile != nil {
		return *profile
	}
	if w.Assets.ITCompatibility.LegacyModPlug {
		return itPatternLoopProfile{}
	}
	return itPatternLoopProfile{true, true, true, itJumpRightWins}
}

func (w *Tracker) itPatternLoop(track int, count uint8) {
	mem := &w.TrackMemories[track]
	if count == 0 {
		mem.itLoopStart = w.Clock.Row
		return
	}
	if mem.itLoopCount == 0 {
		if !w.loopProfile().allowNested {
			for i := 0; i < w.Assets.NumTracks; i++ {
				if i != track && w.TrackMemories[i].itLoopCount != 0 {
					return
				}
			}
		}
		mem.itLoopCount = count
	} else {
		mem.itLoopCount--
		if mem.itLoopCount == 0 {
			if w.loopProfile().resetTarget {
				mem.itLoopStart = w.Clock.Row + 1
			}
			return
		}
	}
	w.itLoopRow, w.itLoopChannel = mem.itLoopStart, track
}

func (w *Tracker) scheduleITFlow() {
	jumpOrder, breakRow := -1, -1
	priority := w.loopProfile().jumpPriority
	for t := 0; t < w.Assets.NumTracks; t++ {
		cmd := w.PendingCmds[t]
		switch cmd.EffectCmd {
		case 0x0b:
			if w.itLoopChannel < 0 || priority == itJumpAlwaysWins || priority == itJumpRightWins && t > w.itLoopChannel {
				jumpOrder = int(cmd.EffectArg)
			}
		case 0x0d:
			breakRow = int(cmd.EffectArg)
		}
	}

	if w.itLoopChannel >= 0 && jumpOrder == -1 {
		order, row := w.Clock.Order, w.itLoopRow
		if row >= len(w.Assets.Patterns[w.Assets.OrderList[order]].Rows) {
			order, row = nextITOrder(w.Assets.OrderList, order+1), 0
		}
		w.itFlowPending, w.itFlowBreak, w.itNextOrder, w.itNextRow = true, false, order, row
		return
	}
	if jumpOrder == -1 && breakRow == -1 {
		return
	}
	if jumpOrder == -1 {
		jumpOrder = w.Clock.Order + 1
	}
	if w.Assets.mptSequenceLength != 0 && jumpOrder >= w.Assets.mptSequenceLength {
		jumpOrder = w.Assets.mptRestartOrder
	}
	jumpOrder = nextITOrder(w.Assets.OrderList, jumpOrder)
	if jumpOrder >= len(w.Assets.OrderList) || breakRow < 0 || breakRow >= len(w.Assets.Patterns[w.Assets.OrderList[jumpOrder]].Rows) {
		breakRow = 0
	}
	w.itFlowPending, w.itFlowBreak, w.itNextOrder, w.itNextRow = true, true, jumpOrder, breakRow
}
