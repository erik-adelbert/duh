// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package player

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erik-adelbert/duh/internal/tui"
	"github.com/erik-adelbert/duh/pkg/pcm"
)

const TUIRefreshRate = 30 * FPS

func NewTUI(ctrl *Controller, name string, tap *pcm.Tap, withVu, withSpectrum bool, emoji bool) tea.Model {
	m := &model{
		ctrl:   ctrl,
		name:   name,
		slide:  SlidePlayback,
		volume: 0.5,

		withVu:     withVu,
		withSpecto: withSpectrum,

		tap: tap,
	}

	sampleCBs := []func(float64, float64){}

	if withVu {
		m.vubars.left = tui.NewVuBar()
		m.vubars.right = tui.NewVuBar()

		sampleCBs = append(sampleCBs, func(lf, rf float64) {
			m.vubars.left.Addf64(lf)
			m.vubars.right.Addf64(rf)
		})
	}

	if withSpectrum {
		sr := ctrl.a.SampleRate()
		m.spectro, _ = tui.NewSpectro(sr, 0, 0, 0, emoji)

		sampleCBs = append(sampleCBs, func(lf, rf float64) {
			m.spectro.Addf64((lf + rf) / 2)
		})
	}

	m.startTap(sampleCBs)

	return m
}

func (m *model) Init() tea.Cmd {
	m.playScreen = mkPlaybackScreen(m, "")

	m.stopScreen = mkStopScreen(m)

	m.helpScreen = tui.NewScreen(func(sb *strings.Builder) {
		fmt.Fprintf(sb, "%s", HelpText)
	}, false)

	m.ctrl.Toggle()

	return tick()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.pos, m.duration, m.playing = m.ctrl.Snapshot()

		if m.withVu {
			m.vubars.left.Tick(time.Time(msg))
			m.vubars.right.Tick(time.Time(msg))
		}

		return m, tick()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			m.tap.Stop()
			return m, tea.Quit
		case " ":
			m.ctrl.Toggle()
		case "1":
			m.slide = SlidePlayback
		case "?":
			m.slide = SlideHelp
		case "up", "e":
			if m.volume += 0.05; m.volume > 1.0 {
				m.volume = 1.0
			}

			m.ctrl.SetVolume(m.volume)
		case "down", "d":
			if m.volume -= 0.05; m.volume < 0.0 {
				m.volume = 0.0
			}

			m.ctrl.SetVolume(m.volume)
		case "right", "f":
			m.ctrl.ForwardBy(Position(10 * time.Second))
		case "left", "s":
			m.ctrl.RewindBy(Position(10 * time.Second))
		}
	}

	return m, nil
}

func (m *model) View() string {
	switch m.slide {
	case SlidePlayback:
		if m.ctrl.IsPlaying() {
			return m.playScreen.Render()
		}
		return m.stopScreen.Render()
	case SlideHelp:
		return m.helpScreen.Render()
	}

	return ""
}

func (m *model) startTap(cbs []func(float64, float64)) {
	cb := func(samples []float64, nchan int) {
		if len(cbs) == 0 {
			return
		}

		for i := 0; i < len(samples); i += nchan {
			left := samples[i]
			right := samples[i+1]

			for _, cb := range cbs {
				cb(left, right)
			}
		}
	}

	m.tap.UpdateFunc(cb)
	m.tap.Start()
}

type Slide int

const (
	SlidePlayback Slide = iota
	SlideHelp
)

type model struct {
	ctrl   *Controller
	name   string
	volume float64

	pos      Position
	duration Position
	playing  bool
	slide    Slide
	width    int
	height   int

	withVu, withSpecto bool

	vubars struct {
		left  tui.VuBar
		right tui.VuBar
	}

	spectro tui.Spectro

	tap *pcm.Tap

	playScreen *tui.Screen
	stopScreen *tui.Screen
	helpScreen *tui.Screen
}

type tickMsg time.Time

func tick() tea.Cmd {
	TUITick := time.Second / TUIRefreshRate

	return tea.Tick(TUITick, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

const FPS = 1
