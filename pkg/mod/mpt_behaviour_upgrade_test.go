package mod

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestMPTBehaviourProjectionPitchMode(t *testing.T) {
	tests := []struct {
		name        string
		version     uint32
		linear      bool
		savedHertz  bool
		expectHertz bool
	}{
		{"old linear keeps enabled", 0x01260000, true, true, true},
		{"old linear keeps disabled", 0x01280100, true, false, false},
		{"old nonlinear clears enabled", 0x01280100, false, true, false},
		{"compatibility export keeps enabled", 0x01300000, false, true, true},
		{"compatibility export keeps disabled", 0x01300000, false, false, false},
		{"before nonlinear fix clears", 0x01300035, false, true, false},
		{"nonlinear fix keeps enabled", 0x01300036, false, true, true},
		{"nonlinear fix keeps disabled", 0x01300036, false, false, false},
		{"recent linear keeps enabled", 0x01321200, true, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saved := mptSavedPlayBehaviours(1 << mptITArpeggio)
			if tt.savedHertz {
				saved |= 1 << mptPeriodsAreHertz
			}
			got, err := projectMPTBehaviours(saved, tt.version, tt.linear, false, false)
			if err != nil {
				t.Fatal(err)
			}
			if got.known.enabled(mptPeriodsAreHertz) != tt.expectHertz || !got.known.enabled(mptITArpeggio) || got.oplNoResetAtEnvelopeEnd {
				t.Fatalf("projection = %+v, want hertz %v and unchanged arpeggio", got, tt.expectHertz)
			}
		})
	}
}

func TestMPTBehaviourProjectionOPL(t *testing.T) {
	tests := []struct {
		name     string
		version  uint32
		opl, ins bool
		want     bool
	}{
		{"before window", 0x01280019, true, true, false},
		{"start of window", 0x01280020, true, true, true},
		{"target version", 0x01280100, true, true, true},
		{"end of window", 0x01290055, true, true, true},
		{"after window", 0x01290056, true, true, false},
		{"no OPL", 0x01280100, false, true, false},
		{"no instruments", 0x01280100, true, false, false},
		{"neither", 0x01280100, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := projectMPTBehaviours(0, tt.version, true, tt.opl, tt.ins)
			if err != nil || got.oplNoResetAtEnvelopeEnd != tt.want || got.known != 0 {
				t.Fatalf("projection = %+v, error = %v, want OPL flag %v", got, err, tt.want)
			}
		})
	}
}

func TestMPTBehaviourProjectionRejectsUnsupported(t *testing.T) {
	for _, tt := range []struct {
		name    string
		saved   mptSavedPlayBehaviours
		version uint32
		want    string
	}{
		{"unknown bit", 1 << mptKnownPlayBehaviourCount, 0x01280100, "bits"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := projectMPTBehaviours(tt.saved, tt.version, true, false, false)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestOpenMPTBehaviourProjectionPitchMode(t *testing.T) {
	tests := []struct {
		name        string
		version     uint32
		linear      bool
		expectEqual bool
	}{
		{"old nonlinear clears hertz", 0x01280100, false, true},
		{"new nonlinear retains hertz", 0x01300036, false, false},
		{"compatibility export retains hertz", 0x01300000, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frequencies := [2]float64{}
			for i := range frequencies {
				b := testITPitchFixture([]byte{0x81, 11, 60, 1, 6, 8, 0}, 6, tt.linear, false)
				b = append(b, []byte("STPM")...)
				version := binary.LittleEndian.AppendUint32(nil, tt.version)
				b = append(b, mptExtensionField("VWSL", 4, version)...)
				b = append(b, mptExtensionField(".FSM", 1, []byte{byte(i) << 7})...)
				ref, _ := referencePCMWithInterpolation(t, b, "2")
				frequencies[i] = referenceTickFrequency(t, ref, 5)
			}
			difference := math.Abs(frequencies[0]-frequencies[1]) / frequencies[0]
			if tt.expectEqual && difference > 0.00001 || !tt.expectEqual && difference < 0.001 {
				t.Fatalf("OpenMPT frequencies = %v, relative difference %.6f", frequencies, difference)
			}
			cleared, err := projectMPTBehaviours(0, tt.version, tt.linear, false, false)
			if err != nil {
				t.Fatal(err)
			}
			set, err := projectMPTBehaviours(1<<mptPeriodsAreHertz, tt.version, tt.linear, false, false)
			if err != nil {
				t.Fatal(err)
			}
			if (cleared.known.enabled(mptPeriodsAreHertz) == set.known.enabled(mptPeriodsAreHertz)) != tt.expectEqual {
				t.Fatal("projection differs from OpenMPT pitch behavior")
			}
		})
	}
}
