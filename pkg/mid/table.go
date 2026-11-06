package mid

import "fmt"

// const maxStaticADLIDs = int(DynamicInstrumentTag)

// AsTables converts the bank database into the instrument, meta, and patch
// lookup tables consumed by the MIDI decoder.
func (db *BankDB) AsTables() (*ADLTables, error) {
	if db == nil {
		return nil, fmt.Errorf("derive ADL tables: nil BankDB")
	}

	tables := &ADLTables{
		Instruments: []ADLData{{}},
		Metas:       []ADLMeta{MkADLMeta()},
		Banks:       make([]ADLBank, len(db.Banks)),
	}
	metas := make([]int, len(db.Ins))

	for i, ins := range db.Ins {
		if ins == nil {
			return nil, fmt.Errorf("derive ADL tables: nil instrument at index %d", i)
		}

		if has(ins.Flags, FlagSilence) {
			continue
		}

		npair := 1
		if Is4Op(ins.Flags) {
			npair = 2
		}

		// if len(tables.Metas) >= maxStaticADLIDs {
		// 	return nil, fmt.Errorf("derive ADL tables: too many meta-instruments for static IDs")
		// }

		meta := ADLMeta{
			ADLID0:         0,
			ADLID1:         0,
			VelocityOffset: ins.VelOff,
			SecondFinetune: ins.SecVoice,
			KON:            ins.KON,
			KOFF:           ins.KOFF,
		}

		if has(ins.Flags, FlagFixedTone) {
			meta.Tone = ins.DrumKey
		}

		if IsPseudo4Op(ins.Flags) {
			meta.Flags |= FlagPseudo4op
		}

		for j := range npair {
			mi := ins.Ops[j*2]
			ci := ins.Ops[j*2+1]

			if mi < 0 || mi >= len(db.Ops) {
				return nil, fmt.Errorf("derive ADL tables: instrument %d has invalid operator index %d", i, mi)
			}

			if ci < 0 || ci >= len(db.Ops) {
				return nil, fmt.Errorf("derive ADL tables: instrument %d has invalid operator index %d", i, ci)
			}

			mod, car := db.Ops[mi], db.Ops[ci]

			if mod == nil || car == nil {
				return nil, fmt.Errorf("derive ADL tables: instrument %d references a nil operator", i)
			}

			// if len(tables.Instruments) >= maxStaticADLIDs {
			// 	return nil, fmt.Errorf("derive ADL tables: too many instruments for static IDs")
			// }

			ft := ins.Note.Off1

			if j == 0 {
				ft = ins.Note.Off0
			}

			data := ADLData{
				ModulatorE862: mod.E862,
				CarrierE862:   car.E862,
				Modulator40:   uint8(mod.D40),
				Carrier40:     uint8(car.D40),
				FeedConn:      uint8(ins.FBConn >> (8 * j)),
				Finetune:      ft,
			}

			id := uint16(len(tables.Instruments))
			tables.Instruments = append(tables.Instruments, data)

			if j == 0 {
				meta.ADLID0 = id
			}
			meta.ADLID1 = id
		}

		metas[i] = len(tables.Metas)
		tables.Metas = append(tables.Metas, meta)
	}

	hasMappings := false
	for ii, bank := range db.Banks {
		if bank == nil {
			return nil, fmt.Errorf("derive ADL tables: nil bank at index %d", ii)
		}

		hasMappings = hasMappings || len(bank.Melos)+len(bank.Drums) != 0

		out := ADLBank{
			ID:    bank.ID,
			Name:  bank.Name,
			Setup: bank.Setup,
			// Preserve bank-select variants and their melodic/percussion roles.
			Melos: make([]ADLMidiBank, 0, len(bank.Melos)),
			Drums: make([]ADLMidiBank, 0, len(bank.Drums)),
		}

		for _, i := range bank.Melos {
			midi, err := db.mkMidiBank(i, metas)
			if err != nil {
				return nil, fmt.Errorf("derive ADL tables: bank %d melodic bank: %w", ii, err)
			}
			out.Melos = append(out.Melos, midi)
		}

		for _, i := range bank.Drums {
			midi, err := db.mkMidiBank(i, metas)
			if err != nil {
				return nil, fmt.Errorf("derive ADL tables: bank %d drum bank: %w", ii, err)
			}
			out.Drums = append(out.Drums, midi)
		}
		tables.Banks[ii] = out
	}

	if len(db.Banks) != 0 && !hasMappings {
		return nil, fmt.Errorf("ADL tables: banks contain no MIDI bank references; regenerate the database")
	}

	return tables, nil
}

func (db *BankDB) mkMidiBank(i int, metas []int) (ADLMidiBank, error) {
	if i < 0 || i >= len(db.Midis) {
		return ADLMidiBank{}, fmt.Errorf("invalid MIDI bank index %d", i)
	}

	bank := db.Midis[i]
	if bank == nil {
		return ADLMidiBank{}, fmt.Errorf("nil MIDI bank at index %d", i)
	}

	mb := ADLMidiBank{LSB: bank.LSB, MSB: bank.MSB}
	for i, j := range bank.Ins {
		if j == -1 {
			continue
		}

		if j >= len(metas) {
			return ADLMidiBank{}, fmt.Errorf("patch %d references invalid instrument index %d", i, j)
		}

		mb.Programs[i] = uint16(metas[j])
	}
	return mb, nil
}
