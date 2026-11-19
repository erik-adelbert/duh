package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func oplEnvelopePortamentoAssets() SongAssets {
	_, assets := transitionFixture(true, false, false)
	instruments := make([]ITInstrument, 2)
	for i := range instruments {
		instruments[i].GlobalVolume = 128
		for note := range 120 {
			instruments[i].Notes[note] = uint8(note)
			instruments[i].Samples[note] = uint8(2 - i)
		}
	}
	instruments[0].VolumeEnvelope = ITVolumeEnvelope{Enabled: true, Points: []ITEnvelopePoint{{Tick: 0, Value: 64}, {Tick: 2, Value: 0}}}
	assets.ITInstruments = instruments
	assets.ITCompatibility.oplEnvelopeEndCut = true
	return assets
}

func renderOPLEnvelopePortamento(t *testing.T, assets SongAssets, cmd ModCommand) []float32 {
	t.Helper()
	return renderOPLEnvelopeRows(t, assets, map[int]ModCommand{4: cmd})
}

func renderOPLEnvelopeRows(t *testing.T, assets SongAssets, commands map[int]ModCommand) []float32 {
	t.Helper()
	rows := make([][]ModCommand, 64)
	for i := range rows {
		rows[i] = make([]ModCommand, 1)
	}
	rows[0][0] = ModCommand{Note: 61, Instrument: 1}
	for row, command := range commands {
		rows[row][0] = command
	}
	assets.OrderList, assets.Patterns = []int{0}, []PatternAsset{{Rows: rows}}
	w := NewTracker(assets, referenceRate, 5760)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 1, 125, -1
	actual := make([]float32, 5760*2)
	if got := w.Read(actual, 5760); got != 5760 {
		t.Fatalf("short render: %d", got)
	}
	return actual
}

func TestOpenMPTOPLEnvelopeNoResetPortamento(t *testing.T) {
	for _, tt := range []struct {
		version uint32
		saved   bool
		wantHz  float64
	}{
		{0x01280019, false, 523.25},
		{0x01280020, false, 261.63},
		{0x01290055, false, 261.63},
		{0x01290056, false, 523.25},
		{0x01300600, false, 523.25},
		{0x01300600, true, 261.63},
	} {
		t.Run(fmt.Sprintf("%08x/saved=%t", tt.version, tt.saved), func(t *testing.T) {
			flags := make([]byte, 14)
			copy(flags, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7})
			if tt.saved {
				flags[13] = 0x20
			}
			_, additional, err := parseMPTPlaybackProfileFlags(flags)
			if err != nil {
				t.Fatal(err)
			}
			effective := projectMPTOPLNoResetAtEnvelopeEnd(additional.oplNoResetAtEnvelopeEnd, tt.version, true, true)
			if effective != (tt.wantHz < 400) {
				t.Fatalf("effective no-reset = %v, want frequency %.2f", effective, tt.wantHz)
			}
			pcm := referenceTransition(t, oplEnvelopeEndFixture(t, tt.version, flags, []byte{0x81, 9, 72, 7, 16, 0}))
			assets := oplEnvelopePortamentoAssets()
			assets.ITCompatibility.oplNoResetAtEnvelopeEnd = effective
			actual := renderOPLEnvelopePortamento(t, assets, ModCommand{Note: 73, EffectCmd: 3, EffectArg: 16})
			for name, output := range map[string][]float32{"OpenMPT": pcm, "tracker": actual} {
				if rmsWindow(output, 90, 110) < .001 {
					t.Fatalf("%s: silent portamento follow-up", name)
				}
				hz := referenceTickFrequency(t, output, 4)
				if math.Abs(hz/tt.wantHz-1) > .01 {
					t.Fatalf("%s: frequency %.2f, want %.2f", name, hz, tt.wantHz)
				}
			}
		})
	}
}

func TestOpenMPTOPLEnvelopeEndFollowup(t *testing.T) {
	tests := []struct {
		name string
		row  []byte
		cmd  ModCommand
	}{
		{"normal note", []byte{0x81, 1, 72, 0}, ModCommand{Note: 73}},
		{"portamento", []byte{0x81, 9, 72, 7, 16, 0}, ModCommand{Note: 73, EffectCmd: 3, EffectArg: 16}},
		{"portamento same instrument", []byte{0x81, 11, 72, 1, 7, 16, 0}, ModCommand{Note: 73, Instrument: 1, EffectCmd: 3, EffectArg: 16}},
		{"portamento PCM instrument", []byte{0x81, 11, 72, 2, 7, 16, 0}, ModCommand{Note: 73, Instrument: 2, EffectCmd: 3, EffectArg: 16}},
		{"instrument only", []byte{0x81, 2, 1, 0}, ModCommand{Instrument: 1}},
	}
	for _, tt := range tests {
		for _, noReset := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/no-reset=%t", tt.name, noReset), func(t *testing.T) {
				flags := make([]byte, 14)
				copy(flags, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7})
				if noReset {
					flags[13] = 0x20
				}
				ref := referenceTransition(t, oplEnvelopeEndFixture(t, 0x01300600, flags, tt.row))
				assets := oplEnvelopePortamentoAssets()
				assets.ITCompatibility.oplNoResetAtEnvelopeEnd = noReset
				actual := renderOPLEnvelopePortamento(t, assets, tt.cmd)
				if rmsWindow(ref, 85, 95) < .001 || rmsWindow(actual, 85, 95) < .001 {
					t.Fatal("silent follow-up")
				}
				want, got := referenceTickFrequency(t, ref, 4), referenceTickFrequency(t, actual, 4)
				if math.Abs(got/want-1) > .01 {
					t.Fatalf("frequency OpenMPT %.2f tracker %.2f", want, got)
				}
			})
		}
	}
}

func TestOpenMPTOPLEnvelopeEndInterveningCommand(t *testing.T) {
	tests := []struct {
		name string
		row  []byte
		cmd  ModCommand
	}{
		{"empty", []byte{0}, ModCommand{}},
		{"volume", []byte{0x81, 4, 64, 0}, ModCommand{HasVolume: true, Volume: 64}},
		{"note cut", []byte{0x81, 1, 254, 0}, ModCommand{Note: 254}},
		{"note off", []byte{0x81, 1, 255, 0}, ModCommand{Note: 255}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7, 0, 0, 0, 0, 0, 0, 0x20}
			row := append(append([]byte(nil), tt.row...), 0x81, 9, 72, 7, 16, 0)
			ref := referenceTransition(t, oplEnvelopeEndFixture(t, 0x01300600, flags, row))
			assets := oplEnvelopePortamentoAssets()
			assets.ITCompatibility.oplNoResetAtEnvelopeEnd = true
			actual := renderOPLEnvelopeRows(t, assets, map[int]ModCommand{4: tt.cmd, 5: {Note: 73, EffectCmd: 3, EffectArg: 16}})
			if rmsWindow(ref, 105, 115) < .001 || rmsWindow(actual, 105, 115) < .001 {
				t.Fatal("silent follow-up")
			}
			want, got := referenceTickFrequency(t, ref, 5), referenceTickFrequency(t, actual, 5)
			if math.Abs(got/want-1) > .01 {
				t.Fatalf("frequency OpenMPT %.2f tracker %.2f", want, got)
			}
		})
	}
}

func TestOpenMPTOPLEndPortamentoTiming(t *testing.T) {
	flags := []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7, 0, 0, 0, 0, 0, 0, 0x20}
	for _, end := range []uint16{2, 3} {
		for row := int(end) + 1; row <= 5; row++ {
			t.Run(fmt.Sprintf("end=%d/row=%d", end, row), func(t *testing.T) {
				b := oplEnvelopeEndFixtureRows(t, 0x01300600, flags, row-1, []byte{0x81, 9, 72, 7, 16, 0})
				ptr := binary.LittleEndian.Uint32(b[194:])
				binary.LittleEndian.PutUint16(b[ptr+314:], end)
				ref := referenceTransition(t, b)
				assets := oplEnvelopePortamentoAssets()
				assets.ITCompatibility.oplNoResetAtEnvelopeEnd = true
				assets.ITInstruments[0].VolumeEnvelope.Points[1].Tick = end
				actual := renderOPLEnvelopeRows(t, assets, map[int]ModCommand{row: {Note: 73, EffectCmd: 3, EffectArg: 16}})
				want, got := referenceTickFrequency(t, ref, row), referenceTickFrequency(t, actual, row)
				if math.Abs(got/want-1) > .01 {
					t.Fatalf("frequency OpenMPT %.2f tracker %.2f", want, got)
				}
			})
		}
	}
}

func TestOpenMPTOPLEndPortamentoMappedSample(t *testing.T) {
	flags := []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7, 0, 0, 0, 0, 0, 0, 0x20}
	for _, speed := range []uint32{4182, 8363, 16726} {
		t.Run(fmt.Sprintf("sample-speed=%d", speed), func(t *testing.T) {
			b := oplEnvelopeEndFixture(t, 0x01300600, flags, []byte{0x81, 9, 72, 7, 16, 0})
			ptr := binary.LittleEndian.Uint32(b[194:])
			b[ptr+65+72*2] = 1
			binary.LittleEndian.PutUint32(b[256+60:], speed)
			ref := referenceTransition(t, b)
			assets := oplEnvelopePortamentoAssets()
			assets.ITCompatibility.oplNoResetAtEnvelopeEnd = true
			assets.ITInstruments[0].Samples[72] = 1
			assets.Samples[0].C5Speed = speed
			actual := renderOPLEnvelopePortamento(t, assets, ModCommand{Note: 73, EffectCmd: 3, EffectArg: 16})
			want, got := referenceTickFrequency(t, ref, 4), referenceTickFrequency(t, actual, 4)
			if math.Abs(got/want-1) > .01 {
				t.Fatalf("frequency OpenMPT %.2f tracker %.2f", want, got)
			}
		})
	}
}

func TestOpenMPTOPLEndPortamentoSilentStart(t *testing.T) {
	flags := []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7, 0, 0, 0, 0, 0, 0, 0x20}
	for row := 2; row <= 4; row++ {
		t.Run(fmt.Sprintf("row=%d", row), func(t *testing.T) {
			b := oplEnvelopeEndFixtureRows(t, 0x01300600, flags, row-1, []byte{0x81, 13, 72, 64, 7, 16, 0})
			sample := binary.LittleEndian.Uint32(b[206:])
			b[sample+19] = 0
			ref := referenceTransition(t, b)
			assets := oplEnvelopePortamentoAssets()
			assets.ITCompatibility.oplNoResetAtEnvelopeEnd = true
			assets.Samples[1].Volume = 0
			actual := renderOPLEnvelopeRows(t, assets, map[int]ModCommand{row: {Note: 73, HasVolume: true, Volume: 64, EffectCmd: 3, EffectArg: 16}})
			want, got := referenceTickFrequency(t, ref, row), referenceTickFrequency(t, actual, row)
			if math.Abs(got/want-1) > .01 {
				t.Fatalf("frequency OpenMPT %.2f tracker %.2f", want, got)
			}
		})
	}
}

func TestOpenMPTOPLEndPortamentoAfterPitchSlide(t *testing.T) {
	flags := []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7, 0, 0, 0, 0, 0, 0, 0x20}
	b := oplEnvelopeEndFixtureRows(t, 0x01300600, flags, 2, []byte{0x81, 8, 6, 0xff, 0, 0x81, 9, 72, 7, 16, 0})
	ref := referenceTransition(t, b)
	assets := oplEnvelopePortamentoAssets()
	assets.ITCompatibility.oplNoResetAtEnvelopeEnd = true
	actual := renderOPLEnvelopeRows(t, assets, map[int]ModCommand{3: {EffectCmd: 1, EffectArg: 0xff}, 4: {Note: 73, EffectCmd: 3, EffectArg: 16}})
	want, got := referenceTickFrequency(t, ref, 4), referenceTickFrequency(t, actual, 4)
	if math.Abs(got/want-1) > .01 {
		t.Fatalf("frequency OpenMPT %.2f tracker %.2f", want, got)
	}
}

func TestMPTOPLEnvelopeNoResetProjection(t *testing.T) {
	for _, tt := range []struct {
		saved, opl, instrument, want bool
	}{
		{false, true, true, true},
		{false, false, true, false},
		{false, true, false, false},
		{true, false, false, true},
	} {
		if got := projectMPTOPLNoResetAtEnvelopeEnd(tt.saved, 0x01280100, tt.opl, tt.instrument); got != tt.want {
			t.Errorf("%+v: got %v", tt, got)
		}
	}
}
