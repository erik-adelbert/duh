// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package module

import "math/bits"

type Format uint32

const (
	FormatMOD  Format = 1 << iota // ProTracker/NoiseTracker MOD (Amiga)
	FormatS3M                     // Scream Tracker 3 (PC)
	FormatXM                      // FastTracker 2 (PC)
	FormatMED                     // OctaMED (Amiga)
	FormatMTM                     // MultiTracker Module (PC)
	FormatIT                      // Impulse Tracker (PC)
	Format669                     // Composer 669 (PC)
	FormatULT                     // UltraTracker (PC)
	FormatSTM                     // Scream Tracker 2 (PC)
	FormatFAR                     // Farandole Composer (PC)
	FormatWAV                     // Waveform Audio File Format (PCM audio)
	FormatAMF                     // DSMI Advanced Module Format (PC)
	FormatAMS                     // Extreme's Advanced Module System (PC)
	FormatDSM                     // Digital Sound Module (PC)
	FormatMDL                     // DigiTracker MDL (PC)
	FormatOKT                     // Oktalyzer (Amiga)
	FormatMID                     // Standard MIDI File
	FormatDMF                     // Delusion Digital Music File (PC)
	FormatPTM                     // PolyTracker Module (PC)
	FormatDBM                     // DigiBooster Pro Module (Amiga)
	FormatMT2                     // MadTracker 2 (PC)
	FormatAMF0                    // ASYLUM Music Format 0 (PC)
	FormatPSM                     // Epic Megagames' ProTracker Studio Module (PC)
	FormatJ2B                     // Jazz Jackrabbit 2 Music (PC)
	FormatABC                     // ABC Notation (text-based music notation)
	FormatPAT                     // Gravis Ultrasound Patch (instrument sample)
	FormatUMX                     // Unreal Music Package (container, various formats)
	FormatNone Format = 0         // No format/unknown
)

var descriptions = []string{
	"Unsupported Format",                                // FormatNone
	"ProTracker/NoiseTracker MOD (Amiga)",               // FormatMOD
	"Scream Tracker 3 (PC)",                             // FormatS3M
	"FastTracker 2 (PC)",                                // FormatXM
	"OctaMED (Amiga)",                                   // FormatMED
	"MultiTracker Module (PC)",                          // FormatMTM
	"Impulse Tracker (PC)",                              // FormatIT
	"Composer 669 (PC)",                                 // Format669
	"UltraTracker (PC)",                                 // FormatULT
	"Scream Tracker 2 (PC)",                             // FormatSTM
	"Farandole Composer (PC)",                           // FormatFAR
	"Waveform Audio File Format (PCM audio)",            // FormatWAV
	"DSMI Advanced Module Format (PC)",                  // FormatAMF
	"Extreme's Advanced Module System (PC)",             // FormatAMS
	"Digital Sound Module (PC)",                         // FormatDSM
	"DigiTracker MDL (PC)",                              // FormatMDL
	"Oktalyzer (Amiga)",                                 // FormatOKT
	"Standard MIDI File",                                // FormatMID
	"Delusion Digital Music File (PC)",                  // FormatDMF
	"PolyTracker Module (PC)",                           // FormatPTM
	"DigiBooster Pro Module (Amiga)",                    // FormatDBM
	"MadTracker 2 (PC)",                                 // FormatMT2
	"ASYLUM Music Format 0 (PC)",                        // FormatAMF0
	"Epic Megagames' ProTracker Studio Module (PC)",     // FormatPSM
	"Jazz Jackrabbit 2 Music (PC)",                      // FormatJ2B
	"ABC Notation (text-based music notation)",          // FormatABC
	"Gravis Ultrasound Patch (instrument sample)",       // FormatPAT
	"Unreal Music Package (container, various formats)", // FormatUMX
}

func (f Format) String() string {
	i := 1 + bits.TrailingZeros32(uint32(f))

	if i >= len(descriptions) {
		return descriptions[0] // FormatNone
	}

	return descriptions[i]
}
