package s3m

type Instrument struct {
	Env                      envelope
	Keys                     [128]byte
	Notes                    [128]byte
	Name                     string
	Filename                 string
	FadeOut                  uint
	Flags                    uint
	Vol                      uint16
	Pan                      uint16
	VolParams                params
	PanParams                params
	PitchParams              params
	VolEnv, PanEnv, PitchEnv byte
	VolSwing, PanSwing       byte
	NNA, DCT, DCA            byte
	IFC, IFR                 byte
	PPS, PPC                 byte
	Midi                     midi
}

type Sample struct {
	Data           []int8      // 24 bytes
	Name           string      // 16 bytes
	Sustain        span        // 16 bytes
	Loop           span        // 16 bytes
	Length         int         // 8 bytes
	C4Speed        uint        // 8 bytes
	Vib            vibrato     // 4 bytes
	Flags          SampleFlags // 2 bytes
	Pan            uint16      // 2 bytes
	Vol, GlobalVol uint16      // 2 bytes each
	RelativeTone   int8        // 1 byte
	FineTune       int8        // 1 byte
}

type SampleFlags uint16

const (
	Sample16Bit SampleFlags = 1 << iota
	SampleStereo
	SampleLooped
	SamplePingPongLoop
	SampleSustainLoop
	SamplePingPongSustainLoop
)

func (f SampleFlags) Has(flag SampleFlags) bool {
	return f&flag != 0
}

type envelope struct {
	VolPoints    []uint16
	PanPoints    []uint16
	PitchPoints  []uint16
	VolTimings   []byte
	PanTimings   []byte
	PitchTimings []byte
}

type midi struct {
	Bank    uint16
	Prog    uint8
	Channel uint8
	DrumKey uint8
}

type params struct {
	Start, End               byte
	SustainStart, SustainEnd byte
}

type span struct {
	Start int
	End   int
}

type vibrato struct {
	Type  byte
	Pos   byte
	Speed byte
	Depth byte
}
