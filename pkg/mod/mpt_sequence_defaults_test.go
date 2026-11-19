package mod

import (
	"strings"
	"testing"
)

func TestResolveMPTSequenceDefaults(t *testing.T) {
	legacy := mptSequenceDefaults{restart: 1, tempo: 1250000, speed: 6}
	zero, tempo, speed, restart := uint32(0), uint32(1005000), uint32(3), uint16(0)
	tests := []struct {
		name string
		seq  mptSequence
		want mptSequenceDefaults
	}{
		{"missing", mptSequence{orders: []uint16{0, 1}}, legacy},
		{"zero overrides", mptSequence{orders: []uint16{0, 1}, defaultTempo: &zero, defaultSpeed: &zero}, legacy},
		{"nonzero overrides", mptSequence{orders: []uint16{0, 1}, restart: &restart, defaultTempo: &tempo, defaultSpeed: &speed}, mptSequenceDefaults{0, 1005000, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveMPTSequenceDefaults(tt.seq, legacy)
			if err != nil || got != tt.want {
				t.Fatalf("defaults = %+v, error %v, want %+v", got, err, tt.want)
			}
		})
	}
}

func TestMPTLegacySequenceDefaults(t *testing.T) {
	for _, tt := range []struct {
		name      string
		extended  *uint32
		wantTempo uint32
		wantErr   bool
	}{
		{"header", nil, 2550000, false},
		{"extended", new(uint32(980)), 9800000, false},
		{"zero", new(uint32(0)), 0, true},
		{"overflow", new(uint32(4294967295)), 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mptLegacySequenceDefaults(255, 10, tt.extended, nil)
			if (err != nil) != tt.wantErr || !tt.wantErr && (got.tempo != tt.wantTempo || got.speed != 10) {
				t.Fatalf("defaults = %+v, error %v", got, err)
			}
		})
	}
}

func TestResolveMPTSequenceDefaultsRejectsUnsupported(t *testing.T) {
	for _, tt := range []struct {
		name   string
		orders []uint16
		legacy mptSequenceDefaults
		want   string
	}{
		{"empty", nil, mptSequenceDefaults{tempo: 1250000, speed: 6}, "restart"},
		{"restart out of bounds", []uint16{0}, mptSequenceDefaults{restart: 1, tempo: 1250000, speed: 6}, "restart"},
		{"tempo too low", []uint16{0}, mptSequenceDefaults{tempo: 319999, speed: 6}, "tempo"},
		{"tempo too high", []uint16{0}, mptSequenceDefaults{tempo: 10000001, speed: 6}, "tempo"},
		{"zero speed", []uint16{0}, mptSequenceDefaults{tempo: 1250000}, "speed"},
		{"speed too high", []uint16{0}, mptSequenceDefaults{tempo: 1250000, speed: 256}, "speed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveMPTSequenceDefaults(mptSequence{orders: tt.orders}, tt.legacy)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
	for _, tempo := range []uint32{320000, 10000000} {
		for _, speed := range []uint32{1, 255} {
			if _, err := resolveMPTSequenceDefaults(mptSequence{orders: []uint16{0}}, mptSequenceDefaults{tempo: tempo, speed: speed}); err != nil {
				t.Fatalf("tempo %d speed %d: %v", tempo, speed, err)
			}
		}
	}
}
