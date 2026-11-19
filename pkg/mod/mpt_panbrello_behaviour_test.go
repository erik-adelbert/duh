package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestMPTPanbrelloProfileProjection(t *testing.T) {
	for _, tt := range []struct {
		name    string
		flags   mptSavedPlayBehaviours
		version uint32
		want    itPanbrelloProfile
		fail    bool
	}{
		{"target", 1<<mptITVibratoTremoloPanbrello | 1<<mptITPanbrelloHold | 1<<mptPanOverride, 0x01280100, itPanbrelloProfile{preciseWaveform: true, hold: true, panOverride: true}, false},
		{"none", 0, 0x01260000, itPanbrelloProfile{}, false},
		{"waveform only", 1 << mptITVibratoTremoloPanbrello, 0x01321200, itPanbrelloProfile{preciseWaveform: true}, false},
		{"hold only", 1 << mptITPanbrelloHold, 0x01280100, itPanbrelloProfile{hold: true}, false},
		{"sample and hold only", 1 << mptITSampleAndHoldPanbrello, 0x01280100, itPanbrelloProfile{sampleAndHold: true}, false},
		{"old version", 0, 0x01259999, itPanbrelloProfile{}, false},
		{"new version", 0, 0x01321201, itPanbrelloProfile{}, false},
		{"unknown flags", 1 << mptKnownPlayBehaviourCount, 0x01280100, itPanbrelloProfile{}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := projectMPTPanbrelloProfile(tt.flags, tt.version)
			if (err != nil) != tt.fail || err == nil && got != tt.want {
				t.Fatalf("profile %+v, error %v, want %+v, fail %v", got, err, tt.want, tt.fail)
			}
		})
	}
}

func TestOpenMPTPanbrelloProfileProjection(t *testing.T) {
	rows := []byte{
		0x81, 11, 60, 1, 25, 0x4f, 0,
		0,
		0x81, 8, 24, 64, 0,
		0x81, 8, 25, 0, 0,
		0x81, 3, 60, 1, 0,
		0x81, 8, 25, 0, 0,
		0,
	}
	const version = uint32(0x01280100)
	all := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	var balances [8][]float64
	for _, precise := range []bool{false, true} {
		for _, hold := range []bool{false, true} {
			for _, override := range []bool{false, true} {
				t.Run(fmt.Sprintf("precise=%v/hold=%v/override=%v", precise, hold, override), func(t *testing.T) {
					flags := all &^ (1<<mptITVibratoTremoloPanbrello | 1<<mptITPanbrelloHold | 1<<mptPanOverride)
					if precise {
						flags |= 1 << mptITVibratoTremoloPanbrello
					}
					if hold {
						flags |= 1 << mptITPanbrelloHold
					}
					if override {
						flags |= 1 << mptPanOverride
					}
					profile, err := projectMPTPanbrelloProfile(flags, version)
					if err != nil {
						t.Fatal(err)
					}
					b := testITPitchFixture(rows, 3, true, false)
					loaded, err := loadIT(b)
					if err != nil {
						t.Fatal(err)
					}
					binary.LittleEndian.PutUint16(b[40:], 0x891)
					b = append(b, []byte("STPM")...)
					b = append(b, mptExtensionField("VWSL", 4, binary.LittleEndian.AppendUint32(nil, version))...)
					flagBytes := make([]byte, 7)
					for i := range flagBytes {
						flagBytes[i] = byte(uint64(flags) >> (8 * i))
					}
					b = append(b, mptExtensionField(".FSM", 7, flagBytes)...)
					tail := len(b)
					b = append(b, mptTestSequences([]mptTestEntry{
						{"l", []byte{1, 0}}, {"a", []byte{0, 0}},
					})...)
					b = binary.LittleEndian.AppendUint32(b, uint32(tail))
					ref := renderMPTSequenceReference(t, b, "--repeat", "-1", "--end-time", "0.480")
					if len(ref) < 480*referenceRate/1000*2 {
						t.Fatalf("short OpenMPT render: %d samples", len(ref))
					}
					loaded.Assets.ITCompatibility.itPanbrelloProfile = &profile
					w := NewTracker(loaded.Assets, referenceRate, 960)
					w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
					actual := make([]float32, 480*referenceRate/1000*2)
					for i := 0; i < len(actual)/2; i += 960 {
						if got := w.Read(actual[i*2:(i+960)*2], 960); got != 960 {
							t.Fatalf("short render at frame %d: %d", i, got)
						}
					}
					balance := func(pcm []float32, tick int) float64 {
						left := channelRMS(pcm, tick*20+15, tick*20+19, 0)
						right := channelRMS(pcm, tick*20+15, tick*20+19, 1)
						if left+right < 1e-6 {
							t.Fatalf("tick %d: silent comparison", tick)
						}
						return (right - left) / (right + left)
					}
					index := 0
					if precise {
						index += 4
					}
					if hold {
						index += 2
					}
					if override {
						index++
					}
					for tick := range 24 {
						want, got := balance(ref, tick), balance(actual, tick)
						balances[index] = append(balances[index], want)
						if tick == 6 {
							continue
						}
						if math.Abs(got-want) > .035 {
							t.Errorf("tick %d: OpenMPT %.3f, tracker %.3f", tick, want, got)
						}
					}
				})
			}
		}
	}
	for _, pair := range [][2]int{{1, 5}, {5, 7}, {6, 7}} {
		if len(balances[pair[0]]) != 24 || len(balances[pair[1]]) != 24 {
			continue
		}
		distinct := false
		for tick := range balances[pair[0]] {
			if math.Abs(balances[pair[0]][tick]-balances[pair[1]][tick]) > .05 {
				distinct = true
			}
		}
		if !distinct {
			t.Errorf("reference profiles %d and %d were not distinguishable", pair[0], pair[1])
		}
	}
}
