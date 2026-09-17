package mod

import (
	"encoding/binary"
	"fmt"
)

// init registers the FastTracker signatures for 2, 4, 6, 8, and 10-32 channels.
func init() {
	type param struct {
		start, size int
		nchan       func(int) int
		suffix      string
	}

	params := []param{
		{
			// Register the CHN2, 4, 6, and 8 signatures.
			start:  0,
			size:   4,
			nchan:  func(i int) int { return 2 * (i + 1) },
			suffix: "CHN",
		},
		{
			// Register the CH10-32 signatures.
			start:  10,
			size:   23,
			nchan:  func(i int) int { return 10 + i },
			suffix: "CH",
		},
	}

	for _, p := range params {
		for i := range p.size {
			nchan := p.nchan(i)

			sig := fmt.Sprintf("%d%s", nchan, p.suffix)

			importers[sig] = fasttracker{chanCount: nchan}
		}
	}
}

type fasttracker struct {
	chanCount int
}

func (p fasttracker) String() string {
	return fmt.Sprintf("FastTracker %d-channel", p.chanCount)
}

func (p fasttracker) channelCount() int {
	return p.chanCount
}

func (p fasttracker) readPattern(r Reader) (pat Pattern, err error) {
	pat = NewPattern(p.chanCount)
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

func (p fasttracker) reorder(order []byte) {
	// FastTracker uses a simple order list, so no reordering is needed.
}
