package mid

import (
	"bytes"
	_ "embed"
	"encoding/gob"
	"fmt"
)

//go:generate make -C cmd/genadlmidi build

//go:embed adlmidi.dat
var builtinDatabase []byte

var builtin *BankDB
var builtinErr error

func init() {
	builtin = NewBankDB()
	if err := gob.NewDecoder(bytes.NewReader(builtinDatabase)).Decode(builtin); err != nil {
		builtinErr = fmt.Errorf("decode built-in bank database: %w", err)
		return
	}

	tables, err := builtin.AsTables()
	if err != nil {
		builtinErr = fmt.Errorf("derive built-in ADL tables: %w", err)
		return
	}

	ADLIns = tables.Instruments
	ADLMetas = tables.Metas
	Banks = tables.Banks
}
