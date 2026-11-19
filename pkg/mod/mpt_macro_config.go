package mod

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

const (
	mptMacroLength           = 32
	mptGlobalMacroCount      = 9
	mptParameterMacroCount   = 16
	mptFixedMacroCount       = 128
	mptDefaultResonanceCount = 16
	mptMacroConfigSize       = (mptGlobalMacroCount + mptParameterMacroCount + mptFixedMacroCount) * mptMacroLength
)

type mptMacroKind uint8

const (
	mptMacroEmpty mptMacroKind = iota
	mptMacroCutoff
	mptMacroResonance
	mptMacroUnsupported
)

type mptMacro struct {
	kind  mptMacroKind
	value byte
}

type mptMacroConfig struct {
	parametered [mptParameterMacroCount]mptMacro
	fixed       [mptFixedMacroCount]mptMacro
}

func defaultMPTMacroConfig() mptMacroConfig {
	var config mptMacroConfig

	config.parametered[0].kind = mptMacroCutoff
	for i := range mptDefaultResonanceCount {
		config.fixed[i] = mptMacro{kind: mptMacroResonance, value: byte(i * 8)}
	}

	return config
}

func parseMPTMacro(raw []byte, parametered bool) mptMacro {
	raw = raw[:mptMacroLength-1]
	if end := bytes.IndexByte(raw, 0); end >= 0 {
		raw = raw[:end]
	}

	text := strings.ReplaceAll(string(raw), " ", "")
	if text == "" {
		return mptMacro{}
	}

	if !strings.HasPrefix(text, "F0F00") || len(text) < 6 {
		return mptMacro{kind: mptMacroUnsupported}
	}

	var kind mptMacroKind
	switch text[5] {
	case '0':
		kind = mptMacroCutoff
	case '1':
		kind = mptMacroResonance
	default:
		return mptMacro{kind: mptMacroUnsupported}
	}

	if parametered && text[6:] == "z" {
		return mptMacro{kind: kind}
	}

	if !parametered && len(text) == 8 {
		hex := func(c byte) (byte, bool) {
			switch {
			case c >= '0' && c <= '9':
				return c - '0', true
			case c >= 'A' && c <= 'F':
				return c - 'A' + 10, true
			default:
				return 0, false
			}
		}

		hi, ok1 := hex(text[6])
		lo, ok2 := hex(text[7])
		if ok1 && ok2 && hi < 8 {
			return mptMacro{kind: kind, value: hi<<4 | lo}
		}
	}

	return mptMacro{kind: mptMacroUnsupported}
}

// Parametered macros may use any SFx slot, while fixed macros address one slot.
func (config *mptMacroConfig) candidates(arg byte) []mptMacro {
	if arg < 0x80 {
		return config.parametered[:]
	}

	i := int(arg - 0x80)
	return config.fixed[i : i+1]
}

func (config *mptMacroConfig) validateCommand(smooth bool, arg byte, version uint32, compatibleFilter bool) error {
	for _, macro := range config.candidates(arg) {
		if macro.kind == mptMacroUnsupported {
			command := "Z"

			if smooth {
				command = "\\"
			}

			return fmt.Errorf("unsupported embedded MIDI macros for %s%02X", command, arg)
		}

		if macro.kind == mptMacroResonance && version >= 0x01260000 && !compatibleFilter {
			return errors.New("resonance macro with legacy filter behavior is not supported")
		}
	}
	return nil
}

func readMPTMacroConfig(data []byte, propertiesStart int) (mptMacroConfig, error) {
	u16 := func(off int) int { return int(binary.LittleEndian.Uint16(data[off:])) }
	u32 := func(off int) int { return int(binary.LittleEndian.Uint32(data[off:])) }

	if u16(44)&0x80 == 0 && u16(46)&8 == 0 {
		return defaultMPTMacroConfig(), nil
	}

	var config mptMacroConfig

	count := u16(34) + u16(36) + u16(38)
	start := 192 + u16(32)
	pos := start + count*4
	first := propertiesStart

	for i := range count {
		ptr := int(u32(start + 4*i))

		if ptr > 0 && ptr < first {
			first = ptr
		}
	}

	if data[46]&1 != 0 && u16(54) != 0 {
		first = min(first, int(u32(56)))
	}
	// Sample data can precede its own header.
	for i := range u16(36) {
		ptr := int(u32(start + 4*(u16(34)+i)))

		if ptr != 0 && u32(ptr+48) != 0 {
			first = min(first, int(u32(ptr+72)))
		}
	}

	if data[46]&2 != 0 && pos+2 <= first {
		historyEnd := pos + 2 + 8*int(u16(pos))
		if historyEnd <= first {
			pos = historyEnd
		}
	}

	if pos > first || mptMacroConfigSize > first-pos {
		return config, errors.New("truncated or overlapping MPTM embedded MIDI macros")
	}

	block := data[pos : pos+mptMacroConfigSize]
	for i := range config.parametered {
		offset := (mptGlobalMacroCount + i) * mptMacroLength
		config.parametered[i] = parseMPTMacro(block[offset:offset+mptMacroLength], true)
	}

	for i := range config.fixed {
		offset := (mptGlobalMacroCount + mptParameterMacroCount + i) * mptMacroLength
		config.fixed[i] = parseMPTMacro(block[offset:offset+mptMacroLength], false)
	}

	return config, nil
}
