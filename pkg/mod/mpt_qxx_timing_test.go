package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMQxxTimingAndMemory(t *testing.T) {
	for _, speed := range []byte{1, 2, 3, 6} {
		for _, compatible := range []bool{false, true} {
			t.Run(fmt.Sprintf("speed=%d/compatible=%t", speed, compatible), func(t *testing.T) {
				rows := []byte{0x81, 11, 60, 2, 17, 0x02, 0,
					0x81, 8, 17, 0, 0, 0,
					0x81, 8, 17, 0x03, 0, 0x81, 8, 17, 0, 0,
					0x81, 8, 17, 0x92, 0, 0x81, 8, 17, 0, 0}
				b := mptS70Fixture(t, rows)
				b[50] = speed
				flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITRetrigger)/8
				mask := byte(1 << (mptITRetrigger % 8))
				b[flag] &^= mask
				if compatible {
					b[flag] |= mask
				}
				sample := binary.LittleEndian.Uint32(b[202:])
				binary.LittleEndian.PutUint32(b[sample+60:], 48000)
				data := binary.LittleEndian.Uint32(b[sample+72:])
				for i := range 4096 {
					var value uint16
					if i < 700 {
						value = 12000
					}
					binary.LittleEndian.PutUint16(b[data+uint32(i)*2:], value)
				}
				duration := int(speed)*7*20 + 20
				ref, actual := mptSharedEffectRenderForDuration(t, b, duration)
				baseRef, baseActual := rmsWindow(ref, 8, 10), rmsWindow(actual, 8, 10)
				if baseRef < .001 || baseActual < .001 {
					t.Fatal("silent comparison baseline")
				}
				for tick := 0; tick < int(speed)*7; tick++ {
					want := rmsWindow(ref, tick*20+8, tick*20+10) / baseRef
					got := rmsWindow(actual, tick*20+8, tick*20+10) / baseActual
					if math.IsNaN(want) || math.IsNaN(got) || math.Abs(got-want) > .10 {
						t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
					}
				}
				assertMPTPatternDelayChunking(t, b, referenceRate/5)
			})
		}
	}
}

func TestOpenMPTMQxxPatternDelay(t *testing.T) {
	for _, delay := range [][]byte{{0x63}, {0xe2}, {0xe2, 0x63}} {
		for _, speed := range []byte{1, 2, 3, 6} {
			for _, compatible := range []bool{false, true} {
				for _, arg := range []byte{0, 0x02, 0x63, 0x92} {
					for _, firstTick := range []bool{false, true} {
						t.Run(fmt.Sprintf("delay=%X/speed=%d/compatible=%t/Q%02X/first=%t", delay, speed, compatible, arg, firstTick), func(t *testing.T) {
							rows := []byte{0x83, 11, 60, 2, 17, 0x02, 0}
							for channel, value := range delay {
								rows = append(rows, byte(0x81+channel), 8, 19, value)
							}
							rows = append(rows, 0x83, 8, 17, arg, 0, 0)
							b := mptS70Fixture(t, rows)
							b[50] = speed
							b[bytesIndex(t, b, "...C")+6] = 3
							flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITRetrigger)/8
							mask := byte(1 << (mptITRetrigger % 8))
							b[flag] &^= mask
							if compatible {
								b[flag] |= mask
							}
							flag = bytesIndex(t, b, ".FSM") + 6 + int(mptITFirstTickHandling)/8
							mask = byte(1 << (mptITFirstTickHandling % 8))
							b[flag] &^= mask
							if firstTick {
								b[flag] |= mask
							}
							sample := binary.LittleEndian.Uint32(b[202:])
							binary.LittleEndian.PutUint32(b[sample+60:], 48000)
							data := binary.LittleEndian.Uint32(b[sample+72:])
							for i := range 4096 {
								var value uint16
								if i < 700 {
									value = 12000
								}
								binary.LittleEndian.PutUint16(b[data+uint32(i)*2:], value)
							}
							ref, actual := mptSharedEffectRenderForDuration(t, b, 800)
							baseRef, baseActual := rmsWindow(ref, 8, 10), rmsWindow(actual, 8, 10)
							if baseRef < .001 || baseActual < .001 {
								t.Fatal("silent comparison baseline")
							}
							for tick := range 39 {
								want := rmsWindow(ref, tick*20+8, tick*20+10) / baseRef
								got := rmsWindow(actual, tick*20+8, tick*20+10) / baseActual
								if math.IsNaN(want) || math.IsNaN(got) || math.Abs(got-want) > .10 {
									t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
								}
							}
							assertMPTPatternDelayChunking(t, b, referenceRate/5)
						})
					}
				}
			}
		}
	}
}

func TestMPTRetriggerAfterVolume(t *testing.T) {
	tests := []struct {
		name    string
		profile bool
		cmd     ModCommand
		want    bool
	}{
		{"non-MPTM", false, ModCommand{EffectCmd: 0x1b, HasVolume: true, Volume: 65}, false},
		{"Q00 slide", true, ModCommand{EffectCmd: 0x1a, HasVolume: true, Volume: 65}, true},
		{"no volume", true, ModCommand{EffectCmd: 0x1b, Volume: 65}, false},
		{"direct volume", true, ModCommand{EffectCmd: 0x1b, HasVolume: true, Volume: 64}, false},
		{"first slide", true, ModCommand{EffectCmd: 0x1b, HasVolume: true, Volume: 65}, true},
		{"last slide", true, ModCommand{EffectCmd: 0x1b, HasVolume: true, Volume: 104}, true},
		{"pitch slide", true, ModCommand{EffectCmd: 0x1b, HasVolume: true, Volume: 105}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Tracker{}
			if tt.profile {
				w.Assets.ITCompatibility.mptMixProfile = &mptMixProfile{}
			}
			if got := w.mptRetriggerAfterVolume(tt.cmd); got != tt.want {
				t.Fatalf("retrigger after volume = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestOpenMPTMQxxVolumeSlides(t *testing.T) {
	for _, vol := range []byte{65, 66, 74, 75, 76, 84, 85, 86, 94, 95, 96, 104} {
		for _, arg := range []byte{0x02, 0x12, 0x63, 0x92, 0xc2} {
			for _, compatible := range []bool{false, true} {
				t.Run(fmt.Sprintf("volume=%d/Q%02X/compatible=%t", vol, arg, compatible), func(t *testing.T) {
					rows := []byte{0x81, 7, 60, 2, 32, 0, 0x81, 12, vol, 17, arg, 0, 0x81, 8, 17, 0, 0}
					b := mptS70Fixture(t, rows)
					b[50] = 4
					flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITRetrigger)/8
					mask := byte(1 << (mptITRetrigger % 8))
					b[flag] &^= mask
					if compatible {
						b[flag] |= mask
					}
					ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
					baseRef, baseActual := rmsWindow(ref, 8, 10), rmsWindow(actual, 8, 10)
					if baseRef < .001 || baseActual < .001 {
						t.Fatal("silent comparison baseline")
					}
					for tick := 4; tick < 12; tick++ {
						want := rmsWindow(ref, tick*20+15, tick*20+18) / baseRef
						got := rmsWindow(actual, tick*20+15, tick*20+18) / baseActual
						if math.IsNaN(want) || math.IsNaN(got) || math.Abs(got-want) > .06 {
							t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
						}
					}
				})
			}
		}
	}
}
