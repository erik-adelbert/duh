package s3m

import "github.com/erik-adelbert/duh/internal/audio"

type Pattern [][]Event

type Event struct {
	Note, Instr, Vol byte
	VolCmd           byte
	Cmd, Param       byte
}

const (
	VCmdVolume byte = iota + 1
	VCmdPanning
	VCmdSlideUp
	VCmdSlideDown
	VCmdFineVolUp
	VCmdFineVolDown
	VCmdVibratoSpeed
	VCmdVibrato
	VCmdPanSlideLeft
	VCmdPanSlideRight
	VCmdTonePortamento
	VCmdPortaUp
	VCmdPortaDown
)

func CmdFromS3M(s3mCmd uint16, isIT bool) (op, arg byte) {
	cmds := []byte{
		'A': audio.CmdSpeed,
		'B': audio.CmdPosJump,
		'C': audio.CmdPatternBreak,
		'D': audio.CmdVolSlide,
		'E': audio.CmdPortaDown,
		'F': audio.CmdPortaUp,
		'G': audio.CmdPortamento,
		'H': audio.CmdVibrato,
		'I': audio.CmdTremor,
		'J': audio.CmdArpeggio,
		'K': audio.CmdVolVibra,
		'L': audio.CmdVolPorta,
		'M': audio.CmdChanVol,
		'N': audio.CmdChanVolSlide,
		'O': audio.CmdSampleOffset,
		'P': audio.CmdPanSlide,
		'Q': audio.CmdRetrigger,
		'R': audio.CmdTremolo,
		'S': audio.CmdExS3M,
		'T': audio.CmdTempo,
		'U': audio.CmdFineVibra,
		'V': audio.CmdGlobalVol,
		'W': audio.CmdGlobalVolSlide,
		'X': audio.CmdPanning,
		'Y': audio.CmdPanbrello,
		'Z': audio.CmdMIDI,
	}
	op = 0x40 + byte(s3mCmd>>8)
	if 'A' <= s3mCmd && s3mCmd <= 'Z' {
		op = cmds[s3mCmd]
	}
	arg = byte(s3mCmd & 0xFF)
	if !isIT {
		arg = (arg>>4)*10 + (arg & 0x0F)
	}
	return
}
