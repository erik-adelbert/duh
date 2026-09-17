// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package player

import (
	"fmt"
	"strings"

	"github.com/erik-adelbert/duh/internal/tui"
)

var NO_COLOR = tui.NoColor()

const WidgetWidth = 40

func mkPlaybackScreen(m *model, pads string) *tui.Screen {
	return tui.NewScreen(func(sb *strings.Builder) {
		fmt.Fprintf(sb, "duh μplayer\n\n%s  %v/%v\n\n",
			m.name, m.pos, m.duration,
		)

		fmt.Fprintf(sb, "V %s\n", tui.Slider(m.volume, WidgetWidth))
		if m.withVu {
			sb.WriteString("\nL ")
			m.vubars.left.Frender(sb, WidgetWidth)
			sb.WriteString("\nR ")
			m.vubars.right.Frender(sb, WidgetWidth)
		}

		if m.withSpecto {
			sb.WriteByte('\n')
			_, _ = m.spectro.FRender(sb, pads)
		}

		fmt.Fprintf(sb,
			"\n[Space] Play/Pause [←→] Fwd/Rev [↑↓] Volume [?] Help [q] Quit",
		)

	}, true)
}

func mkStopScreen(m *model) *tui.Screen {
	return tui.NewScreen(func(sb *strings.Builder) {
		fmt.Fprintf(sb, "μplayer\n\n%s  %v/%v\n\n",
			m.name, m.pos, m.duration,
		)
		fmt.Fprintf(sb, "[Space] Play/Pause [?] Help [q] Quit")

	}, true)
}

const HelpText = `
Keys:
  Space   Play / Pause
  ↑↓      Volume Up / Down
  ←→      Seek Backward / Forward
  1       Playback
  ?       Help
  q       Quit
`

const (
	Red    = "\033[31m"
	Green  = "\033[32m"
	Orange = "\033[33m"
	Reset  = "\033[0m"
)

const (
	Left  bool = true
	Right      = false
)
