package mixer

import "github.com/erik-adelbert/duh/internal/ecs"

type System struct {
	name string
}

func (s *System) Name() string {
	return s.name
}

func (s *System) Priority() int {
	return ecs.SystemPriorityLow
}

func (s *System) Once() bool {
	return false
}

func (s *System) Update(w *ecs.World) {
	StereoMix(w)
}
