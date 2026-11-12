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
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	mid "github.com/erik-adelbert/duh/pkg/mid"
	"github.com/erik-adelbert/duh/pkg/opl3"
)

const cacheMagic = "DUHMIDI-DURATION-CACHE-FILE-V2.0"

type Key [10]int32

type KeyInfo struct {
	peak float64
	amp0 float64

	dtpeak time.Duration

	KON  time.Duration
	KOFF time.Duration

	silent bool
}

type KeyTimer struct {
	cache        Cache[Key, KeyInfo]
	forceTimings bool

	stats struct {
		nhit  int
		nmiss int
	}

	nworker int
	dump    string
}

func NewKeyTimer(cache string) (*KeyTimer, error) {

	kt := new(KeyTimer{
		nworker: runtime.GOMAXPROCS(0),
		dump:    cache,
	})

	err := kt.loadcache(cache)

	if err != nil {
		switch {
		case os.IsNotExist(err):
			fmt.Fprintf(os.Stderr, "Cache file does not exist, will create: %s\n", cache)
		default:
			return nil, err
		}
	}

	return kt, nil
}

func (kt *KeyTimer) TimeDB(db *mid.BankDB) {
	var (
		wg          sync.WaitGroup
		nhit, nmiss atomic.Int64
	)

	jobs := make(chan *mid.InsEntry, kt.nworker)

	for range kt.nworker {
		wg.Go(func() {
			synth := NewSynth()

			for ie := range jobs {
				k := hash(ie)

				var info KeyInfo

				if ki, ok := kt.cache.Load(k); ok {
					nhit.Add(1)

					info = ki
				} else {
					nmiss.Add(1)

					info = keyInfo(synth, db, ie)
					kt.cache.Store(k, info)
				}

				ie.KON, ie.KOFF = info.KON, info.KOFF
				ie.HasStoredTimings = false

				if info.silent {
					ie.Flags |= mid.FlagSilence
				}
			}
		})
	}

	for _, ie := range db.Ins {
		if ie.HasStoredTimings && !kt.forceTimings {
			continue
		}
		jobs <- ie
	}
	close(jobs)

	wg.Wait()

	kt.stats.nhit = int(nhit.Load())
	kt.stats.nmiss = int(nmiss.Load())
}

func (kt *KeyTimer) Close() error {
	if kt.dump != "" {
		return kt.DumpCache(kt.dump)
	}
	return nil
}

func (kt *KeyTimer) loadcache(cache string) error {
	if cache == "" {
		return nil
	}

	f, err := os.Open(cache)

	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck // can't do anything if it fails

	var buf [32]byte

	_, err = io.ReadFull(f, buf[:])

	if err != nil {
		return err
	}

	if string(buf[:]) != cacheMagic {
		return fmt.Errorf("incompatible duration cache %q; use a fresh cache file", cache)
	}

	var nitem uint32

	err = fread(f, &nitem)

	if err != nil {
		return err
	}

	kt.cache.Clear()

	for range nitem {
		var k Key

		err = fread(f, &k)

		if err != nil {
			return err
		}

		_, err = io.ReadFull(f, buf[:17])

		if err != nil {
			return err
		}

		ki := KeyInfo{
			KON:  time.Duration(u64(buf[0:8])),
			KOFF: time.Duration(u64(buf[8:16])),

			silent: buf[16] == 0x01,
		}

		kt.cache.Store(k, ki)
	}

	fmt.Fprintf(os.Stderr, "Loaded %d items from cache\n", nitem)

	return nil
}

func (kt *KeyTimer) DumpCache(cache string) error {
	fmt.Fprintf(os.Stderr, "Dumping cache to: %s\n", cache)
	if cache == "" {
		return nil
	}

	f, err := os.Create(cache)

	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck // can't do anything if it fails

	_, err = f.Write([]byte(cacheMagic))

	if err != nil {
		return err
	}

	var nitem uint32

	var keys []Key

	kt.cache.Range(func(k Key, _ KeyInfo) bool {
		keys = append(keys, k)
		return true
	})

	nitem = uint32(len(keys))

	slices.SortFunc(keys, func(a, b Key) int {
		for i := range a {
			if a[i] != b[i] {
				return int(a[i] - b[i])
			}
		}

		return 0
	})

	err = fwrite(f, nitem)

	if err != nil {
		return err
	}

	for _, k := range keys {
		err = fwrite(f, k)
		if err != nil {
			return err
		}

		v, _ := kt.cache.Load(k)

		err = fwrite(f, v.KON)
		if err != nil {
			return err
		}

		err = fwrite(f, v.KOFF)
		if err != nil {
			return err
		}

		err = fwrite(f, v.silent)
		if err != nil {
			return err
		}
	}

	return nil
}

func keyInfo(synth *Synth, banks *mid.BankDB, ie *mid.InsEntry) KeyInfo {
	const (
		ndiv = 150

		spi   = opl3.YMF262SampleRate / ndiv
		dt    = 1 * time.Second
		dtwin = dt / ndiv

		histsz = opl3.YMF262SampleRate / 10 // 100 ms of history
	)

	hist := newHistory(histsz)

	const (
		maxon  = 40 * time.Second
		maxoff = 60 * time.Second
	)

	synth.Reset()
	synth.SetInstrument(banks, ie)
	synth.KeyOn()

	if synth.rhythm {
		return KeyInfo{
			KON:    time.Duration(1000) * time.Millisecond,
			KOFF:   time.Duration(10) * time.Millisecond,
			silent: false,
		}
	}

	var (
		win [histsz]int16
		han [histsz]float64
	)

	const (
		con  = 0.008
		coff = 0.003
	)

	var nwin int

	var (
		amp0   float64
		peak0  float64
		dtpeak time.Duration

		KON      = maxon
		KONFound bool

		KOFF          time.Duration
		KOFFTimeFound bool
	)

	var (
		maxpeak    float64
		smin, smax int16
		hsz        int
		t0         time.Duration
	)

	const maxsilent = 6 * time.Second

	step := func() {
		var (
			frame [2 * spi]int16
			mono  [spi]int16
		)

		synth.Stream2Ch(frame[:], len(frame))

		for i := range mono {
			mono[i] = int16((int32(frame[2*i]) + int32(frame[2*i+1])) / 2)
		}

		hist.add(mono[:])
		smin = min(smin, slices.Min(mono[:]))
		smax = max(smax, slices.Max(mono[:]))
	}

	measure := func() float64 {
		w := hist.window(win[:])

		if len(w) != hsz {
			hsz = len(w)
			hann(han[:hsz])
		}

		return rms(w, han[:hsz])
	}

	for t := t0; t < maxon; t += dtwin {
		nwin++

		step()
		peak := measure()
		switch {
		case t == 0:
			amp0 = peak
			peak0 = peak

		case peak > peak0:
			peak0 = peak
			dtpeak = t

			KONFound = false
		case !KONFound && peak <= peak0*con:
			KON = t
			KONFound = true
		}

		maxpeak = max(maxpeak, peak)

		silent := peak < maxpeak*con
		silent = silent || (smin >= -1 && smax <= 1)

		if t > maxsilent && silent {
			break
		}
	}

	ontime := time.Duration(nwin) * dtwin

	if !KONFound {
		KON = ontime
	}

	if ontime < maxon {
		synth.Reset()
		synth.SetInstrument(banks, ie)
		synth.KeyOn()

		hist.reset()

		smin0, smax0 := smin, smax

		dtreplay := max(dtpeak, dtwin)
		dtreplay = min(dtreplay, maxon)

		for t := t0; t < dtreplay; t += dtwin {
			step()
		}

		smin, smax = smin0, smax0
	}

	synth.KeyOff()

	for t := t0; t < maxoff; t += dtwin {
		step()
		peak := measure()
		if !KOFFTimeFound && peak <= peak0*coff {
			KOFF, KOFFTimeFound = t, true
		}

		if peak < maxpeak*coff {
			break
		}

		if t > maxsilent && (smin >= -1 && smax <= 1) {
			break
		}
	}

	silent := peak0 < 0.5 || (smin >= -19 && smax <= 18)

	return KeyInfo{
		peak: peak0,
		amp0: amp0,

		dtpeak: dtpeak,

		KON:  KON,
		KOFF: KOFF,

		silent: silent,
	}
}

func hann(win []float64) {
	const π = math.Pi

	n := len(win)
	for i := range win {
		r := float64(i) / float64(n-1)
		win[i] = 0.5 * (1 - math.Cos(2*π*r))
	}
}

func rms(win []int16, han []float64) float64 {
	wlen := len(win)

	if wlen == 0 {
		return 0
	}

	var sum float64

	for i, x := range win {
		f := float64(x)
		sum += f * f * han[i]
	}

	return math.Sqrt(sum / float64(wlen))
}

func hash(ie *mid.InsEntry) Key {
	return Key{
		int32(ie.Ops[0]),
		int32(ie.Ops[1]),
		int32(ie.Ops[2]),
		int32(ie.Ops[3]),
		int32(ie.FBConn),
		int32(ie.Note.Off0),
		int32(ie.Note.Off1),
		int32(ie.DrumKey),
		int32(ie.Flags),
		int32(ie.SecVoice),
	}
}
