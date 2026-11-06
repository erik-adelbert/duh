package mid

import "time"

type ADLData struct {
	ModulatorE862 uint32
	CarrierE862   uint32

	Modulator40 uint8
	Carrier40   uint8

	FeedConn uint8

	Finetune int8
}

type ADLMeta struct {
	ADLID0 uint16
	ADLID1 uint16

	Tone  uint8
	Flags int

	VelocityOffset int8
	SecondFinetune float64

	KON  time.Duration
	KOFF time.Duration
}

func MkADLMeta() ADLMeta {
	return ADLMeta{
		Flags: FlagSilence,
	}
}

type ADLTables struct {
	Instruments []ADLData
	Metas       []ADLMeta
	Banks       []ADLBank
}

type ADLBank struct {
	ID    int
	Name  string
	Setup uint16 // low byte: volume model, bit 8: deep tremolo, 9: deep vibrato, 10: MT-32
	Melos []ADLMidiBank
	Drums []ADLMidiBank
}

type ADLMidiBank struct {
	Bank
	Programs [128]uint16
}

var (
	ADLIns   []ADLData
	ADLMetas []ADLMeta
	Banks    []ADLBank
)
