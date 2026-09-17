package mod

import (
	"encoding/binary"
)

func init() {
	importers["M.K."] = protracker{}
	importers["M!K!"] = protracker{}
}

type protracker struct{}

func (p protracker) String() string {
	return "ProTracker 4-channel"
}

func (p protracker) channelCount() int {
	return 4
}

func (p protracker) readPattern(r Reader) (pat Pattern, err error) {
	pat = NewPattern(4)

	size := binary.Size(pat[0][0])

	for _, row := range pat {
		for i := range row {
			n, err := r.Read(row[i][:])

			switch {
			case n != size:
				return pat, mkError(errRead, "incomplete event data")
			case err != nil:
				return pat, mkError(errRead, err)
			}
		}
	}

	return
}

func (p protracker) reorder(order []byte) {
	// ProTracker uses a simple order list, so no reordering is needed.
}
