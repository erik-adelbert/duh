package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTDefaultCutoffMacro(t *testing.T) {
	for _, opl := range []bool{false, true} {
		for _, cutoff := range []byte{0, 32, 64, 96, 127} {
			t.Run(fmt.Sprintf("opl=%t/cutoff=%d", opl, cutoff), func(t *testing.T) {
				fixture := func(value byte) ([]byte, SongAssets) {
					b := mptmPlaybackFixture()
					b[50] = 2
					b[bytesIndex(t, b, "..MT")+6] = 0
					instrument := byte(1)
					if opl {
						instrument = 2
					}
					pattern := int(binary.LittleEndian.Uint32(b[202:]))
					rows := []byte{0x81, 11, 60, instrument, 26, value, 0}
					rows = append(rows, make([]byte, 63)...)
					binary.LittleEndian.PutUint16(b[pattern:], uint16(len(rows)))
					copy(b[pattern+8:], rows)
					loaded, err := loadMPTM(b)
					if err != nil {
						t.Fatal(err)
					}
					return b, loaded.Assets
				}
				b, assets := fixture(cutoff)
				base, baseAssets := fixture(127)
				ref, refBase := referenceTransition(t, b), referenceTransition(t, base)
				actual, actualBase := renderMPTMixFixture(t, assets, 5760), renderMPTMixFixture(t, baseAssets, 5760)
				wantBase, gotBase := rmsWindow(refBase, 40, 100), rmsWindow(actualBase, 40, 100)
				if wantBase < .001 || gotBase < .001 {
					t.Fatal("silent comparison baseline")
				}
				want, got := rmsWindow(ref, 40, 100)/wantBase, rmsWindow(actual, 40, 100)/gotBase
				if math.Abs(got-want) > .05 {
					t.Errorf("relative RMS: OpenMPT %.5f tracker %.5f", want, got)
				}
			})
		}
	}
}

func TestMPTCutoffMacroValidation(t *testing.T) {
	b := mptmPlaybackFixture()
	pattern := int(binary.LittleEndian.Uint32(b[202:]))
	copy(b[pattern+8:], []byte{0x81, 8, 26, 32, 0})
	b[46] |= 8
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "embedded MIDI macros") {
		t.Fatalf("embedded macro configuration: %v", err)
	}
}

func TestOpenMPTCutoffMacroTransitions(t *testing.T) {
	tests := []struct {
		name        string
		rows, plain []byte
		windows     [][2]int
	}{
		{
			"before note",
			[]byte{0x81, 8, 26, 32, 0, 0x81, 3, 60, 1, 0},
			[]byte{0, 0x81, 3, 60, 1, 0},
			[][2]int{{50, 70}, {90, 110}},
		},
		{
			"change playing note",
			[]byte{0x81, 3, 60, 1, 0, 0x81, 8, 26, 32, 0, 0x81, 8, 26, 96, 0},
			[]byte{0x81, 3, 60, 1, 0, 0, 0},
			[][2]int{{50, 70}, {90, 110}},
		},
		{
			"full cutoff without note",
			[]byte{0x81, 11, 60, 1, 26, 32, 0, 0x81, 8, 26, 127, 0},
			[]byte{0x81, 3, 60, 1, 0, 0},
			[][2]int{{50, 70}, {90, 110}},
		},
		{
			"full cutoff with new note",
			[]byte{0x81, 11, 60, 1, 26, 32, 0, 0x81, 11, 60, 1, 26, 127, 0},
			[]byte{0x81, 3, 60, 1, 0, 0x81, 3, 60, 1, 0},
			[][2]int{{50, 70}, {90, 110}},
		},
	}
	for _, tt := range tests {
		for _, opl := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/opl=%t", tt.name, opl), func(t *testing.T) {
				fixture := func(rows []byte) ([]byte, []float32) {
					b := mptmPlaybackFixture()
					b[50] = 2
					b[bytesIndex(t, b, "..MT")+6] = 0
					pattern := int(binary.LittleEndian.Uint32(b[202:]))
					packed := append(append([]byte(nil), rows...), make([]byte, 64)...)
					if opl {
						for i := 0; i+3 < len(packed); i++ {
							if packed[i] == 0x81 && (packed[i+1] == 3 || packed[i+1] == 11) {
								packed[i+3] = 2
							}
						}
					}
					binary.LittleEndian.PutUint16(b[pattern:], uint16(len(packed)))
					copy(b[pattern+8:], packed)
					loaded, err := loadMPTM(b)
					if err != nil {
						t.Fatal(err)
					}
					w := NewTracker(loaded.Assets, referenceRate, 5760)
					w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 2, 125, -1
					actual := make([]float32, 5760*2)
					if n := w.Read(actual, 5760); n != 5760 {
						t.Fatalf("short render: %d", n)
					}
					return b, actual
				}
				b, actual := fixture(tt.rows)
				base, actualBase := fixture(tt.plain)
				ref, refBase := referenceTransition(t, b), referenceTransition(t, base)
				for _, win := range tt.windows {
					wantBase, gotBase := rmsWindow(refBase, win[0], win[1]), rmsWindow(actualBase, win[0], win[1])
					if wantBase < .001 || gotBase < .001 {
						t.Fatal("silent comparison baseline")
					}
					want, got := rmsWindow(ref, win[0], win[1])/wantBase, rmsWindow(actual, win[0], win[1])/gotBase
					if math.Abs(got-want) > .06 {
						t.Errorf("%d-%d ms relative RMS: OpenMPT %.5f tracker %.5f", win[0], win[1], want, got)
					}
				}
			})
		}
	}
}

func renderMPTFilterInstrumentFixture(t *testing.T, b, swing []byte) ([]byte, []float32) {
	t.Helper()
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	properties := append([]byte("XTPM"), mptExtensionField("..SC", 1, swing)...)
	properties = append(properties, b[bounds.propertiesStart:bounds.serializedStart]...)
	for _, field := range []struct {
		code string
		data []byte
	}{
		{"...C", []byte{1, 0}}, {"..MT", []byte{0}}, {".MMP", []byte{4, 0, 0, 0}},
		{".APS", []byte{64, 0, 0, 0}}, {"VTSV", []byte{64, 0, 0, 0}},
	} {
		properties = append(properties, mptExtensionField(field.code, uint16(len(field.data)), field.data)...)
	}
	data := append([]byte(nil), b[:bounds.propertiesStart]...)
	data = append(data, properties...)
	tail := len(data)
	data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
	data = binary.LittleEndian.AppendUint32(data, uint32(tail))
	loaded, err := loadMPTM(data)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, referenceRate, 5760)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 2, 125, -1
	actual := make([]float32, 5760*2)
	if w.Read(actual, 5760) != 5760 {
		t.Fatal("short render")
	}
	return data, actual
}

func TestOpenMPTCutoffMacroInstrument(t *testing.T) {
	for _, opl := range []bool{false, true} {
		for _, swing := range []byte{0, 26} {
			for _, envelope := range []bool{false, true} {
				t.Run(fmt.Sprintf("opl=%t/swing=%d/envelope=%t", opl, swing, envelope), func(t *testing.T) {
					fixture := func(macro bool, swing byte) ([]byte, []float32) {
						b := oplEnvelopeEndFixture(t, 0x01280100, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}, []byte{0})
						b[30], b[44], b[50] = 8, b[44]|8, 2
						instrument := 1
						if !opl {
							instrument = 2
						}
						ptr := int(binary.LittleEndian.Uint32(b[194+4*(instrument-1):]))
						b[ptr+304] = 0
						if envelope {
							setTestEnvelope(b[ptr:ptr+554], 468, 0x81, []int8{-16, 16}, []uint16{0, 4})
						}
						if !macro {
							b[ptr+58] = 0x80 | 48
						}
						pattern := int(binary.LittleEndian.Uint32(b[210:]))
						rows := []byte{0x81, 3, 60, byte(instrument), 0}
						if macro {
							rows = []byte{0x81, 11, 60, byte(instrument), 26, 48, 0}
						}
						length := int(binary.LittleEndian.Uint16(b[pattern:]))
						binary.LittleEndian.PutUint16(b[pattern+2:], 32)
						clear(b[pattern+8 : pattern+8+length])
						copy(b[pattern+8:pattern+8+length], rows)
						values := []byte{0, 0}
						values[instrument-1] = swing
						return renderMPTFilterInstrumentFixture(t, b, values)
					}
					b, actual := fixture(true, swing)
					base, baseline := fixture(false, 0)
					ref := referenceTransition(t, b)
					refBase := referenceTransition(t, base)
					wantBase, gotBase := rmsWindow(refBase, 40, 100), rmsWindow(baseline, 40, 100)
					if wantBase < .00001 || gotBase < .00001 {
						t.Fatal("silent comparison baseline")
					}
					want, got := rmsWindow(ref, 40, 100)/wantBase, rmsWindow(actual, 40, 100)/gotBase
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .05 {
						t.Errorf("relative RMS OpenMPT %g tracker %g", want, got)
					}
				})
			}
		}
	}
}

func TestMPTCutoffMacroChannelState(t *testing.T) {
	profile, err := projectMPTMixProfile(4, 64, 64)
	if err != nil {
		t.Fatal(err)
	}
	assets := SongAssets{
		IT: true, NumTracks: 2,
		ITCompatibility: ITCompatibility{mptMixProfile: &profile},
		Samples:         []SampleAsset{{Data: make([]float32, 32), C5Speed: 8363, Volume: 64}},
	}
	w := NewTracker(assets, referenceRate, 64)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x19, EffectArg: 40}
	MPTCutoffMacroSystem(w)
	if w.TrackCursors[0].ITFilterCutoff != 40 || w.TrackCursors[1].ITFilterCutoff != 127 {
		t.Fatal("cutoff did not persist on the selected track")
	}
	for track := range 2 {
		if !w.startSampleVoice(TrackID(track), ModCommand{Note: 61}, 0, 61, 0) {
			t.Fatal("voice allocation failed")
		}
	}
	first, second := w.TrackCursors[0].ActiveVoice, w.TrackCursors[1].ActiveVoice
	if !w.VoiceFilters[first].Enabled || w.VoiceFilters[second].Enabled {
		t.Fatal("new voices did not inherit their track's cutoff")
	}
	w.VoiceFilters[first].Y1, w.VoiceRightFilters[first].Y1 = .25, -.25
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x19, EffectArg: 80}
	MPTCutoffMacroSystem(w)
	if w.VoiceFilters[first].Y1 != .25 || w.VoiceRightFilters[first].Y1 != -.25 ||
		w.VoiceFilters[second].Enabled {
		t.Fatal("macro lost filter history or affected another track")
	}
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x19, EffectArg: 127}
	MPTCutoffMacroSystem(w)
	if !w.VoiceFilters[first].Enabled {
		t.Fatal("full cutoff without a new note unexpectedly disabled the filter")
	}
	w.PendingCmds[0].Note = 61
	MPTCutoffMacroSystem(w)
	if w.VoiceFilters[first].Enabled {
		t.Fatal("full cutoff on a new note did not disable the filter")
	}
}
