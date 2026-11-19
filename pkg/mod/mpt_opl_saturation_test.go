package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func mptOPLMultivoiceFixture(t *testing.T, voices int, global byte, varied bool) ([]byte, SongAssets) {
	t.Helper()
	b, assets := mptMixFixture(t, true, false, 64, 64, testOPLPatch)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	patternPos := int(binary.LittleEndian.Uint32(b[202:]))
	packed := make([]byte, 0, voices*4+64)
	assets.NumTracks = voices
	assets.ITGlobalVolume = global
	assets.Patterns[0].Rows[0] = make([]ModCommand, voices)
	for channel := range voices {
		note := byte(60)
		if varied {
			note += byte(channel % 7)
			if channel%2 == 0 {
				b[64+channel], assets.Panning[channel] = 0, -1
			} else {
				b[64+channel], assets.Panning[channel] = 64, 1
			}
		}
		packed = append(packed, byte(0x81+channel), 3, note, 2)
		assets.ITChannelVolume[channel] = 64
		assets.Patterns[0].Rows[0][channel] = ModCommand{Note: note + 1, Instrument: 2}
	}
	for row := 1; row < 64; row++ {
		assets.Patterns[0].Rows[row] = make([]ModCommand, voices)
	}
	packed = append(packed, make([]byte, 64)...)
	data := append([]byte(nil), b[:patternPos]...)
	data[48] = global
	data = binary.LittleEndian.AppendUint16(data, uint16(len(packed)))
	data = binary.LittleEndian.AppendUint16(data, 64)
	data = append(data, 0, 0, 0, 0)
	data = append(data, packed...)
	data = append(data, b[bounds.propertiesStart:bounds.serializedStart]...)
	tail := len(data)
	data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
	data = binary.LittleEndian.AppendUint32(data, uint32(tail))
	if _, err := locateMPTExtensions(data); err != nil {
		t.Fatal(err)
	}
	return data, assets
}

func TestOpenMPTOPLSaturation(t *testing.T) {
	for _, voices := range []int{1, 4, 9, 18} {
		for _, varied := range []bool{false, true} {
			for _, global := range []byte{128, 32} {
				t.Run(fmt.Sprintf("voices=%d/varied=%v/global=%d", voices, varied, global), func(t *testing.T) {
					b, assets := mptOPLMultivoiceFixture(t, voices, global, varied)
					ref, actual := referenceTransition(t, b), renderMPTMixFixture(t, assets, 5760)
					want, got := rmsWindow(ref, 40, 100), rmsWindow(actual, 40, 100)
					var refPeak, actualPeak float64
					refClipped, actualClipped := 0, 0
					clipLevel := float64(assets.ITCompatibility.mptMixProfile.oplGain) * float64(global) / 128
					for i := 40 * 48 * 2; i < 100*48*2; i++ {
						refPeak = math.Max(refPeak, math.Abs(float64(ref[i])))
						actualPeak = math.Max(actualPeak, math.Abs(float64(actual[i])))
						if math.Abs(float64(ref[i])) >= clipLevel*.999 {
							refClipped++
						}
						if math.Abs(float64(actual[i])) >= clipLevel*.999 {
							actualClipped++
						}
					}
					if want < .001 || math.Abs(got/want-1) > .035 || math.Abs(actualPeak/refPeak-1) > .035 {
						t.Errorf("reference rms=%g peak=%g tracker rms=%g peak=%g", want, refPeak, got, actualPeak)
					}
					for channel := range 2 {
						wantChannel := channelRMS(ref, 40, 100, channel)
						gotChannel := channelRMS(actual, 40, 100, channel)
						if wantChannel < .00001 {
							if gotChannel > .00001 {
								t.Errorf("channel %d: expected silence, got %g", channel, gotChannel)
							}
						} else if math.Abs(gotChannel/wantChannel-1) > .035 {
							t.Errorf("channel %d RMS: reference=%g tracker=%g", channel, wantChannel, gotChannel)
						}
					}
					if !varied && voices >= 9 && (refClipped < 100 || actualClipped < 100) {
						t.Errorf("fixture did not reach saturation: reference=%d tracker=%d", refClipped, actualClipped)
					}
				})
			}
		}
	}
}
