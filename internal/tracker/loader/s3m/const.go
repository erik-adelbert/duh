package s3m

type Kind uint8

const (
	KindNone Kind = iota
	KindMod
	KindS3M
	KindXM Kind = 4
	KindIT Kind = 32
)

type flags uint

const (
	FlagWithMIDISetup flags = 0x001
	FlagFastSlides    flags = 0x002
	FlagLinearSlides  flags = 0x010
	FlagPatternLoop   flags = 0x020
	FlagStep          flags = 0x040
	FlagPaused        flags = 0x080
	FlagFading        flags = 0x100
	FlagEnded         flags = 0x200
	FlagGlobalFading  flags = 0x400
	FlagFirstTick     flags = 0x1000
	FlagAmigaLimits   flags = 0x10000
)

const (
	MaxNumBaseChans  = 64
	MaxNumChans      = 128
	MaxNumEnvPoints  = 32
	MaxNumEQBands    = 6
	MaxNumMixEffects = 8
	MaxNumOrders     = 256
	MaxNumPatterns   = 240
	MaxNumInstrs     = 240
	MaxNumSamples    = MaxNumInstrs
	// MaxSampleLength  = 16 * _MiB
	// MaxSampleRate = 192000
	// MinPeriod = 64
	// MaxPeriod = 65535
)

const (
	ModAmigaC2      = 0x1AB
	MaxSampleLength = 16000000
	MaxSampleRate   = 192000
	MaxOrders       = 256
	MaxPatterns     = 240
	MaxSamples      = 240
	MaxInstruments  = MaxSamples
	MaxChannels     = 128
	MaxBaseChannels = 64
	MaxEnvPoints    = 32
	MinPeriod       = 0x0020
	MaxPeriod       = 0x7FFF
	MaxPatternName  = 32
	MaxChannelName  = 20
	MaxInfoName     = 80
	MaxEQBands      = 6
	MaxMixPlugins   = 8
)
