// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tui

import (
	"fmt"
	"time"
)

func TimerString(d time.Duration) string {
	hours := int(d.Hours())

	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	hundredths := int(d.Milliseconds()/10) % 100

	if hours > 0 {
		return fmt.Sprintf(
			"%02d:%02d:%02d.%02d", hours, minutes, seconds, hundredths,
		)
	}

	return fmt.Sprintf("%02d:%02d.%02d", minutes, seconds, hundredths)
}
