package mod

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestMPTMissingFlagsProfile(t *testing.T) {
	for _, tt := range []struct {
		version uint32
		linear  bool
		want    mptSavedPlayBehaviours
		fail    bool
	}{
		{0x01199999, true, 0, true},
		{0x01200000, true, 0, false},
		{0x01200001, true, 1 << mptSlidesAtSpeed1, false},
		{0x01200103, false, 1 << mptSlidesAtSpeed1, false},
		{0x01200104, true, 0, false},
		{0x01230400, true, 0, false},
		{0x01240000, true, 1 << mptPeriodsAreHertz, false},
		{0x01240000, false, 0, false},
		{0x01259999, true, 1 << mptPeriodsAreHertz, false},
		{0x01260000, true, 0, true},
	} {
		flags, additional, err := resolveMPTPlaybackProfileFlags(nil, tt.version, tt.linear)
		if (err != nil) != tt.fail || flags != tt.want || additional != (mptAdditionalPlayBehaviours{}) {
			t.Errorf("version %#x linear %v: flags %#x additional %+v error %v", tt.version, tt.linear, flags, additional, err)
		}
	}
	flags, _, err := resolveMPTPlaybackProfileFlags([]byte{}, 0x01240000, true)
	if err != nil || flags != 0 {
		t.Fatalf("explicit empty field was upgraded: flags %#x error %v", flags, err)
	}
	if _, _, err := resolveMPTPlaybackProfileFlags([]byte{0, 0, 0, 0, 0, 0, 8}, 0x01230400, true); err == nil {
		t.Fatal("explicit unknown flag accepted")
	}
}

func mptWithoutPlaybackFlags(t *testing.T, data []byte) []byte {
	t.Helper()
	bounds, err := locateMPTExtensions(data)
	if err != nil {
		t.Fatal(err)
	}
	pos := bytes.Index(data[bounds.propertiesStart:bounds.serializedStart], []byte(".FSM"))
	if pos < 0 {
		t.Fatal("missing test flag field")
	}
	pos += bounds.propertiesStart
	end := pos + 6 + int(binary.LittleEndian.Uint16(data[pos+4:]))
	result := append([]byte(nil), data[:pos]...)
	result = append(result, data[end:len(data)-4]...)
	tail := bounds.serializedStart - (end - pos)
	return binary.LittleEndian.AppendUint32(result, uint32(tail))
}

func TestLoadMPTMMissingFlags(t *testing.T) {
	flags := mptSavedPlayBehaviours(0)
	b := mptSharedEffectFixture(t, []byte{0x81, 3, 60, 1, 0}, flags)
	binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01230400)
	missing := mptWithoutPlaybackFlags(t, b)
	ref, actual := mptSharedEffectRenderForDuration(t, missing, 100)
	assertMPTPCMCorrelation(t, ref, actual, 10, 50)
	explicitRef, explicitActual := mptSharedEffectRenderForDuration(t, b, 100)
	if !reflect.DeepEqual(actual, explicitActual) || !reflect.DeepEqual(ref, explicitRef) {
		t.Fatal("absent and explicitly zero flags differ for OpenMPT 1.23")
	}
}

func mptMissingFlagsInstrumentFixture(t *testing.T) []byte {
	t.Helper()
	b := oplEnvelopeEndFixture(t, 0x01230400, []byte{0, 0, 0, 0, 0, 0, 0}, []byte{0})
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	properties := append([]byte(nil), b[bounds.propertiesStart:bounds.serializedStart]...)
	for _, field := range []struct {
		code string
		data []byte
	}{
		{"...C", []byte{1, 0}}, {"..MT", []byte{2}},
		{".MMP", []byte{4, 0, 0, 0}}, {".APS", []byte{64, 0, 0, 0}}, {"VTSV", []byte{64, 0, 0, 0}},
	} {
		properties = append(properties, mptExtensionField(field.code, uint16(len(field.data)), field.data)...)
	}
	b[30], b[44] = 8, b[44]|8
	data := append([]byte(nil), b[:bounds.propertiesStart]...)
	data = append(data, properties...)
	tail := len(data)
	data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
	data = binary.LittleEndian.AppendUint32(data, uint32(tail))
	return mptWithoutPlaybackFlags(t, data)
}

func TestOpenMPTMissingFlagsSwing(t *testing.T) {
	for _, pan := range []bool{false, true} {
		t.Run(map[bool]string{false: "volume", true: "pan"}[pan], func(t *testing.T) {
			b := mptMissingFlagsInstrumentFixture(t)
			instrument := int(binary.LittleEndian.Uint32(b[198:]))
			pattern := int(binary.LittleEndian.Uint32(b[210:]))
			b[pattern+11] = 2
			b[instrument+25], b[instrument+304], b[256+19] = 32, 0, 32
			random := rand.New(rand.NewSource(1)).Intn(256) - 128
			swing, delta := 0, 0
			for value := 1; value <= 64; value++ {
				d := ((random*value/64 + 1) * 64) / 199
				if pan {
					d = random * value * 4 / 128
				}

				if d != 0 && d%4 == 0 && d > -64 && d < 64 {
					swing, delta = value, d
					break
				}
			}
			if swing == 0 {
				t.Fatal("could not construct fixed-swing reference")
			}
			offset := 26
			if pan {
				offset = 27
			}
			b[instrument+offset] = byte(swing)
			_, actual := mptSharedEffectRenderForDuration(t, b, 100)
			fixed := append([]byte(nil), b...)
			fixed[instrument+offset] = 0
			if pan {
				fixed[instrument+25] = byte(32 + delta/4)
			} else {
				fixed[256+19] = byte(32 + delta/4)
			}
			ref := referenceMPTMForDuration(t, fixed, 100)
			baseline := append([]byte(nil), b...)
			baseline[instrument+offset] = 0
			refBase, actualBase := mptSharedEffectRenderForDuration(t, baseline, 100)
			for channel := range 2 {
				gotBase, wantBase := channelRMS(actualBase, 10, 30, channel), channelRMS(refBase, 10, 30, channel)
				if gotBase < .00001 || wantBase < .00001 {
					t.Fatal("silent comparison baseline")
				}
				got := channelRMS(actual, 10, 30, channel) / gotBase
				want := channelRMS(ref, 10, 30, channel) / wantBase
				if math.Abs(got-want) > .04 {
					t.Fatalf("swing %d delta %d channel %d: tracker %g, OpenMPT %g", swing, delta, channel, got, want)
				}
			}
		})
	}
}
