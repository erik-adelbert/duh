// duh ADL midi driver for OPL3
//
// Based on:
//   Wohlstand libADLMIDI
//   bisqwit adlmidi
//
// Uses:
//   sneakernets DMXOPL wopl definitions
//
// Original:
// Copyright (c) 2010-2014 Joel Yliluoma
// Copyright (c) 2015-2026 Vitaly Novichkov
//
// Banks:
// Copyright (c) 2017 Shannon Freeman
//
// Go implementation and modifications:
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mid "github.com/erik-adelbert/duh/pkg/mid"
)

func TestKeyTimerRejectsOldCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.cache")
	if err := os.WriteFile(path, []byte("DUHMIDI-DURATION-CACHE-FILE-V1.0"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewKeyTimer(path); err == nil || !strings.Contains(err.Error(), "use a fresh cache file") {
		t.Fatalf("old cache error = %v", err)
	}
}

func TestTimeDBPreservesStoredTimingsAndBlankDecision(t *testing.T) {
	stored := mid.NewInsEntry()
	stored.HasStoredTimings = true
	stored.KON, stored.KOFF = 2*time.Second, 3*time.Second
	storedZero := mid.NewInsEntry()
	storedZero.HasStoredTimings = true
	calculated := mid.NewInsEntry()

	db := mid.NewBankDB()
	db.Ins = []*mid.InsEntry{stored, storedZero, calculated}
	timer := &KeyTimer{nworker: 1}
	timer.cache.Store(hash(calculated), KeyInfo{
		KON: 4 * time.Second, KOFF: 5 * time.Second, silent: true,
	})
	timer.TimeDB(db)

	if stored.KON != 2*time.Second || stored.KOFF != 3*time.Second || stored.Flags&mid.FlagSilence != 0 {
		t.Errorf("stored instrument changed: %+v", stored)
	}
	if storedZero.KON != 0 || storedZero.KOFF != 0 || storedZero.Flags&mid.FlagSilence != 0 {
		t.Errorf("zero stored timings or blank decision changed: %+v", storedZero)
	}
	if calculated.KON != 4*time.Second || calculated.KOFF != 5*time.Second || calculated.Flags&mid.FlagSilence == 0 {
		t.Errorf("instrument without stored timings was not analyzed: %+v", calculated)
	}
	if timer.stats.nhit != 1 || timer.stats.nmiss != 0 {
		t.Errorf("cache stats = %d hits, %d misses, want 1 hit and 0 misses", timer.stats.nhit, timer.stats.nmiss)
	}
}

func TestTimeDBCanRecomputeStoredTimings(t *testing.T) {
	stored := mid.NewInsEntry()
	stored.HasStoredTimings = true
	stored.KON, stored.KOFF = time.Second, 2*time.Second
	db := mid.NewBankDB()
	db.Ins = []*mid.InsEntry{stored}
	timer := &KeyTimer{nworker: 1, forceTimings: true}
	timer.cache.Store(hash(stored), KeyInfo{KON: 3 * time.Second, KOFF: 4 * time.Second, silent: true})
	timer.TimeDB(db)
	if stored.KON != 3*time.Second || stored.KOFF != 4*time.Second || stored.Flags&mid.FlagSilence == 0 || stored.HasStoredTimings {
		t.Errorf("explicit recomputation was not applied: %+v", stored)
	}
}
