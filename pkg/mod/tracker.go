// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import "math/rand"

const (
	MaxTracks        = 127
	MaxVoices        = 256
	itHeaderChannels = 64
	itOrderSkip      = -1
)

type TrackID int
type VoiceID int

// --- Read-Only Loaded Assets ---

type ModCommand struct {
	Note       uint8 // 0 = none, 1..120 = C-0..B-9, 253 = fade, 254 = cut, 255 = key-off
	Instrument uint8
	Volume     uint8
	HasVolume  bool
	EffectCmd  uint8
	EffectArg  uint8
	s3mDummy   bool
	modPeriod  uint16
}

type PatternAsset struct {
	Rows              [][]ModCommand // [row][track]
	mptRowsPerBeat    uint32
	mptRowsPerMeasure uint32
	mptTempoSwing     []uint32
}

type ITEnvelopePoint struct {
	Tick  uint16
	Value int16
}

type ITVolumeEnvelope struct {
	Enabled      bool
	Loop         bool
	Sustain      bool
	Carry        bool
	LoopStart    uint8
	LoopEnd      uint8
	SustainStart uint8
	SustainEnd   uint8
	Points       []ITEnvelopePoint
}

type ITInstrument struct {
	Samples                 [120]uint8
	Notes                   [120]uint8
	NewNoteAction           uint8
	DuplicateCheck          uint8
	DuplicateAction         uint8
	Fadeout                 uint16
	mptFadeout              uint16
	hasMPTFadeout           bool
	GlobalVolume            uint8
	mptGlobalVolumeOverride bool
	Panning                 uint8
	mptPanning              uint16
	hasMPTPanning           bool
	HasPanning              bool
	PitchPanSeparation      int8
	PitchPanCenter          uint8
	VolumeSwing             uint8
	PanSwing                uint8
	mptCutoffSwing          uint8
	mptResonanceSwing       uint8
	mptVolumeReleaseNode    uint8
	mptHasVolumeRelease     bool
	mptFilterMode           uint8
	mptHasFilterMode        bool
	mptSincResampling       bool
	mptCubicResampling      bool
	mptNearestResampling    bool
	mptPitchWheelDepth      int8
	FilterCutoff            uint8
	FilterResonance         uint8
	HasFilterCutoff         bool
	HasFilterResonance      bool
	FilterEnvelope          bool
	VolumeEnvelope          ITVolumeEnvelope
	PanEnvelope             ITVolumeEnvelope
	PitchEnvelope           ITVolumeEnvelope
	itDisabledEnvData       [3][82]byte
}

type SampleAsset struct {
	Data              []float32
	RightData         []float32 // nil for mono; otherwise the same length as Data
	opl               *oplPatch
	LoopStart         uint32
	LoopEnd           uint32
	LoopEnable        bool
	LoopPingPong      bool
	mptSustainStart   uint32
	mptSustainEnd     uint32
	mptSustainLoop    bool
	itSustainPingPong bool
	mptCues           [9]uint32
	C5Speed           uint32
	Volume            uint8
	ITGlobalVolume    uint8
	ITPanning         uint8
	ITHasPanning      bool
	ITVibratoRate     uint8
	ITVibratoDepth    uint8
	ITVibratoSweep    uint8
	ITVibratoType     uint8
	modFineTune       int8
}

type ITCompatibility struct {
	// LegacyModPlug selects the shared behavior profile for ModPlug 1.09-1.16 files.
	LegacyModPlug                bool
	mptCueOffsetITBehaviour      bool
	ExtendedFilter               bool
	OldEffects                   bool
	LinearSlides                 bool
	CompatibleGxx                bool
	HighOffsetUnsupported        bool
	oplPortaSampleRetrigger      bool // Experimental MPTM behavior.
	oplInstrumentSwitch          bool // Experimental MPTM behavior.
	oplEnvelopeEndCut            bool // Experimental OpenMPT OPL envelope-end timing.
	mptEnvelopePositionHandling  bool
	mptEnvelopeReset             bool
	oplNoResetAtEnvelopeEnd      bool
	mptVolumePitchSlides         bool
	mptRealNoteMapping           bool
	mptPitchPanSeparation        bool
	mptPanningReset              bool
	mptITRetrigger               bool
	mptPingPongMode              bool
	mptPingPongLookahead         bool
	mptImprecisePingPong         bool
	mptResetFilterOnPorta        bool
	mptInitialNoteMemory         bool
	mptNoSustainOnPortamento     bool
	mptEmptyNoteMapIgnoreCell    bool
	mptOffsetWithInstrument      bool
	mptDoublePortamentoSlides    bool
	mptCarryAfterNoteOff         bool
	mptNoteCutWithPorta          bool
	mptStoppedFilterEnvAtStart   bool
	mptVolumeSlidePropagation    bool
	mptInstrumentWithoutNote     bool
	mptNNAReset                  bool
	mptCutStopsSample            bool
	mptClearOldNoteAfterCut      bool
	mptPortamentoInstrument      bool
	mptPortamentoSwapResetsPos   bool
	mptOldSwingBehaviour         bool
	mptLegacySwing               bool
	mptLegacyTremor              bool
	mptGlobalVolumeSlideShared   bool
	mptSeparatePortamentoMemory  bool
	mptSharedVolumeColumnMemory  bool
	mptSlidesAtSpeedOne          bool
	mptMultiSampleInstrument     bool
	itVolColFinePortamento       bool
	oplFlexibleNoteOff           bool                  // OpenMPT OPL voices remain controllable while fading.
	itRestoreChannelPan          bool                  // Instrument/sample pan does not permanently replace channel pan.
	itDCTSampleAcrossInstruments bool                  // Experimental legacy MPTM duplicate-sample behavior.
	itPatternLoopProfile         *itPatternLoopProfile // Experimental MPTM loop profile.
	itPanbrelloProfile           *itPanbrelloProfile   // Experimental MPTM sine profile.
	mptMixProfile                *mptMixProfile        // Relative sample/OPL preamps.
	mptMacroConfig               *mptMacroConfig
	mptTempoUnclamped            bool // MPTM tempo slides may exceed 255 BPM.
	mptTempoAsSpeed              bool // MPTM VBlank timing treats Txx as speed.
}

type SongAssets struct {
	proTracker            bool
	s3m                   bool
	s3mSharedEffectMemory bool
	s3mMixGain            float32
	s3mFrequencyPitch     bool
	mptNearestResampling  bool
	mptSincResampling     bool
	mptCubicResampling    bool
	mptRestartOrder       int
	mptSequenceLength     int
	OrderList             []int
	Patterns              []PatternAsset
	Samples               []SampleAsset
	ITInstruments         []ITInstrument
	mptRowsPerBeat        uint16 // Nonzero selects experimental modern timing.
	mptTempoSwing         []uint32
	NumTracks             int
	Panning               [MaxTracks]float32
	IT                    bool
	ITCompatibility       ITCompatibility
	// Deprecated: use ITCompatibility.LegacyModPlug.
	ITLegacyPitchPan bool
	// Deprecated: use ITCompatibility.ExtendedFilter.
	ITExtendedFilter bool
	ITGlobalVolume   uint8
	ITMixGain        float32
	ITChannelVolume  [MaxTracks]uint8
	ITMuted          [MaxTracks]bool
	ITSurround       [MaxTracks]bool
}

// --- Global Components (Singleton) ---

type SequencerClock struct {
	Order            int
	Row              int
	Tick             int
	Speed            int     // Ticks per row (default 6)
	Tempo            float64 // BPM (default 125.0)
	SampleRate       float64
	SamplesPerTick   int
	SamplesRemaining int
	SongFinished     bool
	mptRowsPerBeat   uint32
	mptSwingFactor   uint32
	mptTickRemainder float64
}

// --- Pattern Track Components (Indexed by TrackID) ---

type TrackCursor struct {
	Muted            bool
	ActiveVoice      VoiceID // -1 if none
	Panning          float32
	Surround         bool
	modVolume        uint8
	modCurrentPeriod int
	modVibratoPhase  uint8
	modVibratoSpeed  uint8
	modVibratoDepth  uint8
	modTremoloPhase  uint8
	modTremoloSpeed  uint8
	modTremoloDepth  uint8
	modTargetPeriod  int
	modPortaSpeed    int
	modLoopStart     uint8
	modLoopCount     uint8

	ITFilterCutoff    uint8
	ITFilterResonance uint8

	mptHighPass         bool
	mptMicroTuning      int16
	mptHasNewNoteAction bool
	mptNewNoteAction    uint8

	itLastVolume      float32
	itHasLastVolume   bool
	itRestorePan      float32
	itRestoreSurround bool
	itHasRestorePan   bool

	oplEndedPitch    PitchState
	oplEndedSample   int
	oplHasEndedPitch bool
	oplEndedTicks    uint8
}

type EffectMemory struct {
	LastArpeggio   uint8
	LastInstrument uint8
	LastNote       uint8
	LastOffset     uint8
	LastPortaSpeed uint8
	LastPortaUp    uint8
	LastPortaDown  uint8
	LastRetrig     uint8
	LastVibrato    uint8 // High nibble = speed, low nibble = depth
	LastVolSlide   uint8

	RetrigCount int

	ITFineVolSlide uint8
	ITHighOffset   uint8
	ITLastSpecial  uint8
	ITVibratoPos   uint8
	ITNoteCut      bool
	ITVolSlide     uint8

	itChannelVolSlide  uint8
	itFineVibratoDepth bool
	itGlissando        bool
	itGlobalVolSlide   uint8

	itLoopStart         int
	itLoopCount         uint8
	itPanSlide          uint8
	itPanbrello         uint8
	itPanbrelloPos      uint8
	itPanbrelloOffset   int8
	itPanbrelloWaveform uint8
	itPanbrelloRandom   int8
	itSharedVolParam    uint8
	itSpecialNote       bool
	itTempoSlide        uint8
	itTremorParam       uint8
	itTremorCount       uint8
	itTremolo           uint8
	itTremoloPos        uint8
	itTremoloWaveform   uint8
	itVibratoWaveform   uint8

	mptActiveMacro uint8
}

// --- Voice Components (Indexed by VoiceID) ---

type VoiceLifecycle struct {
	Active      bool
	ParentTrack TrackID
	KeyOff      bool
	FadeOutVol  float32 // 1.0 down to 0.0

	ITInstrument         int // 1-based; zero for sample mode
	ITEnvelopeTick       int
	ITEnvelopeVolume     float32
	ITFade               bool
	ITFilterCutoff       uint8
	ITFilterResonance    uint8
	ITPanEnvelopeTick    int
	ITPanEnvelopeValue   float32
	ITPastChannelVolume  uint8
	ITPastNote           bool
	ITPitchEnvelopeTick  int
	ITPitchEnvelopeValue float32

	itHasNewNoteAction      bool
	itNewNoteAction         uint8
	itOPLLegacyFade         bool
	itPanEnvelopeControl    itEnvelopeControl
	itPitchEnvelopeControl  itEnvelopeControl
	itVolumeEnvelopeControl itEnvelopeControl

	mptEnvelopeReleased       bool
	mptSustainSuppressed      bool
	mptSustainInheritPingPong bool
	mptEnvelopePosition       int
	mptHighPass               bool
	mptPanEnvelopeChanged     bool
	mptReleaseValue           int
	mptResonanceSwing         int16
	mptRestoreResonance       bool
}

type PitchState struct {
	Note          uint8
	BaseFreqHz    float64 // Underlying note pitch (modified by portamento)
	PortaTargetHz float64 // Destination pitch for Tone Portamento (0x03)
	VibratoPhase  float64 // LFO phase in radians [0 .. 2*Pi]
	VibratoOffset float64 // Temporary Hz offset for current tick
	StepIncrement float64 // Final sample step sent to ResamplerSystem

	ITAutoVibPos   uint8
	ITAutoVibDepth uint32
	ITSlidePitch   int32 // IT slide state: frequency, or period for legacy ModPlug.

	itFineTuneSpeed uint32
	itPortamento    bool
	itRealNote      uint8
	itVibratoDelta  int

	mptMicroTuning int16
}

type AmplitudeState struct {
	Volume   float32 // 0.0 to 1.0
	Panning  float32 // -1.0 (left) to +1.0 (right)
	Surround bool

	TargetLeft   float32
	TargetRight  float32
	CurrentLeft  float32 // Ramped gain to prevent audio clicks
	CurrentRight float32

	rampTargetLeft  float32
	rampTargetRight float32
	rampStepLeft    float32
	rampStepRight   float32
	rampRemaining   int
	rampDuration    int
	rampPanbrello   float32

	ITInstrumentVolume float32
	ITPanSwing         int16
	ITVolumeSwing      int16

	itEnvelopeRamping bool
	itSmoothRamp      bool
	itPanbrelloOffset float32
	itTremorMuted     bool
	itTremoloOffset   float32

	mptApplyLegacySwing bool
}

type SamplePlayback struct {
	SampleID        int
	Position        float64 // Fractional sample cursor
	Reverse         bool
	mptLooped       bool
	ScratchBuf      []float32
	RightScratchBuf []float32
	Ended           bool

	oplChannel int // one-based; zero for PCM
}

type ITFilterState struct {
	Enabled    bool
	HighPass   bool
	A0, B0, B1 float64
	Y1, Y2     float64
}

type mptVisitedRow struct {
	order, row    int
	loopCounts    [MaxTracks]uint8
	modLoopStarts [modChannels]uint8
}

// --- ECS World Container ---

type Tracker struct {
	Assets          SongAssets
	Clock           SequencerClock
	ITGlobalVolume  uint8
	ITChannelVolume [MaxTracks]uint8

	modBreakRow      int // -1 when no ProTracker pattern break is pending.
	modJumpOrder     int // -1 when there is no explicit order target.
	modLoopRow       int
	modFilterEnabled bool
	modFilterState   [2][2]float64
	itFlowPending    bool
	itFlowBreak      bool
	itNextOrder      int
	itNextRow        int
	itLoopRow        int
	itLoopChannel    int
	itRandom         *rand.Rand

	mptRowDelay    int // Additional row repetitions.
	mptFrameDelay  int // Additional ticks per repetition.
	mptVisitedRows map[mptVisitedRow]struct{}

	opl       *oplSynth
	oplBuffer []int16
	oplVoices [oplChannels]VoiceID // -1 when free; otherwise owns the voice's oplChannel

	// Track Entity Pools
	TrackCursors  [MaxTracks]TrackCursor
	TrackMemories [MaxTracks]EffectMemory
	PendingCmds   [MaxTracks]ModCommand

	// Voice Entity Pools
	VoiceLives        [MaxVoices]VoiceLifecycle
	VoicePitches      [MaxVoices]PitchState
	VoiceAmps         [MaxVoices]AmplitudeState
	VoiceSamples      [MaxVoices]SamplePlayback
	VoiceFilters      [MaxVoices]ITFilterState
	VoiceRightFilters [MaxVoices]ITFilterState
}

func NewTracker(assets SongAssets, sampleRate float64, maxBlockSize int) *Tracker {
	assets.ITCompatibility.LegacyModPlug = assets.ITCompatibility.LegacyModPlug || assets.ITLegacyPitchPan
	assets.ITCompatibility.ExtendedFilter = assets.ITCompatibility.ExtendedFilter || assets.ITExtendedFilter
	assets.ITLegacyPitchPan = assets.ITCompatibility.LegacyModPlug
	assets.ITExtendedFilter = assets.ITCompatibility.ExtendedFilter
	w := &Tracker{
		Assets:          assets,
		ITGlobalVolume:  assets.ITGlobalVolume,
		ITChannelVolume: assets.ITChannelVolume,
		itLoopRow:       -1,
		itLoopChannel:   -1,
		modBreakRow:     -1,
		modJumpOrder:    -1,
		modLoopRow:      -1,
		itRandom:        rand.New(rand.NewSource(1)),
		Clock: SequencerClock{
			Speed:          6,
			Tempo:          125.0,
			SampleRate:     sampleRate,
			mptRowsPerBeat: uint32(assets.mptRowsPerBeat),
			mptSwingFactor: 1 << 24,
		},
	}
	if assets.ITCompatibility.mptMixProfile != nil {
		w.mptVisitedRows = make(map[mptVisitedRow]struct{})
	}
	for i := range w.TrackCursors {
		w.TrackCursors[i].ActiveVoice = -1
		w.TrackCursors[i].Panning = assets.Panning[i]
		w.TrackCursors[i].Surround = assets.ITSurround[i]
		w.TrackCursors[i].ITFilterCutoff = 127
		if assets.ITCompatibility.mptInitialNoteMemory {
			w.TrackMemories[i].LastNote = 1
		}
	}
	for i := range w.oplVoices {
		w.oplVoices[i] = -1
	}
	stereo := false
	for _, sample := range assets.Samples {
		if len(sample.RightData) != 0 {
			stereo = true
		}
		if sample.opl != nil && w.opl == nil {
			w.opl = newOPLSynth(int(sampleRate))
			w.oplBuffer = make([]int16, maxBlockSize*2)
		}
	}
	for i := range w.VoiceSamples {
		w.VoiceSamples[i].ScratchBuf = make([]float32, maxBlockSize)
		if stereo {
			w.VoiceSamples[i].RightScratchBuf = make([]float32, maxBlockSize)
		}
	}
	return w
}
