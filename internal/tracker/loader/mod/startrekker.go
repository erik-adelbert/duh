package mod

import (
	"encoding/binary"
	"fmt"
)

func init() {
	importers["FLT4"] = startrekker{chanCount: 4}
	importers["FLT8"] = startrekker{chanCount: 8}
}

type startrekker struct {
	chanCount int
}

func (p startrekker) String() string {
	return fmt.Sprintf("StarTrekker %d-channel", p.chanCount)
}

func (p startrekker) channelCount() int {
	return p.chanCount
}

func (p startrekker) readPattern(r Reader) (pat Pattern, err error) {
	pat = NewPattern(p.chanCount)
	size := binary.Size(pat[0][0])

	readChans := func(start, end int) error {
		for _, row := range pat {
			for i := start; i < end; i++ {
				n, err := r.Read(row[i][:])

				switch {
				case n != size:
					return mkError(errRead, "incomplete event data")
				case err != nil:
					return mkError(errRead, err)
				}
			}
		}
		return nil
	}

	// Read the first 4 channels
	err = readChans(0, 4)

	if err == nil && p.chanCount == 8 {
		// Read the remaining 4 channels
		err = readChans(4, 8)
	}

	return
}

func (p startrekker) reorder(order []byte) {
	if p.chanCount == 8 {
		for i := range order {
			order[i] /= 2
		}
	}
}
