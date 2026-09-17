package s3m

type AudioChan struct {
	CurrentSample                          *int8
	Pos, PosLo                             uint32
	Inc                                    int32
	LVol, RVol                             int32
	LRamp, RRamp                           int32
	Flags                                  AudioFlags
	LoopStart, LoopEnd                     uint32
	LRampVol, RRampVol                     int32
	Filter                                 filter
	LOfs, ROfs                             int32
	RampLen                                int32
	Sample                                 *Sample
	LNewVol, RNewVol                       int32
	ActualVol, ActualPan                   int32
	Vol, Pan, FadeOutVol                   int32
	Period                                 int32
	C4Speed                                int32
	PortaDest                              int32
	Instr                                  *Instrument
	SampleSrc                              *Sample
	VolEnvPos, PanEnvPos, PitchEnvPos      uint32
	MasterCh, VUMeter                      uint32
	GobalVol, InsVol                       int32
	Finetune, Transpose                    int32
	PortaSlide                             int32
	VibAutoDepth                           int32
	VibAutoPos, VibPos, TreloPos, PanloPos uint32
	VolSwing, PanSwing                     int16
	Note, NNA                              byte
	NewNote, NewInstr, Command, Arp        byte
	OldVolSlide, OldFineVolUpDown          byte
	OldPortaUpDown, OldFinePortaUpDown     byte
	OldPanSlide, OldChVolSlide             byte
	VibType, VibSpeed, VibDepth            byte
	TreloType, TreloSpeed, TreloDepth      byte
	PanloType, PanloSpeed, PanloDepth      byte
	OldCmdEx, OldVolParam, OldTempo        byte
	OldOffset, OldHiOffset                 byte
	CutOff, Resonnance                     byte
	RetrigCount, RetrigParam               byte
	TremorCount, TremorParam               byte
	PatternLoop, PatternLoopCount          byte
	RowNote, RowInstr, RowVol              byte
	RowVolCmd                              byte
	RowCmd                                 byte
	RowParam                               byte
	LeftVu, RightVu                        byte
	ActiveMacro                            byte
}

type AudioSetting struct {
	Name  string
	Pan   uint32
	Vol   uint32
	Flags AudioFlags
}

type AudioFlags uint32

const (
	Audio16Bit AudioFlags = 1 << iota
	AudioLoop
	AudioLoopBounce
	AudioSustain
	AudioSustainBounce
	AudioPanning AudioFlags = 1 << iota
	AudioStereo
	AudioBounce
	AudioMute AudioFlags = 1 << iota
	AudioKeyOff
	AudioFadeOut
	AudioSurround
	AudioRaw
	AudioHQResampling AudioFlags = 1 << iota
	AudioFiltering
	AudioVolumeRamp
	AudioVibrato AudioFlags = 1 << iota
	AudioTremolo
	AudioPanbrello
	AudioPortamento
	AudioGlissando
	AudioVolumeEnv AudioFlags = 1 << iota
	AudioPanEnv
	AudioPitchEnv
	AudioFastVolumeRamp
	AudioExtraLoudness AudioFlags = 1 << iota
	AudioReverb
	AudioExtraReverb
)

type filter struct {
	Y  [4]int32
	A0 int32
	B  [2]int32
}
