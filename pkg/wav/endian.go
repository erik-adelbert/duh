// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wav

import "encoding/binary"

var (
	// sugars
	put16 = binary.LittleEndian.PutUint16
	put32 = binary.LittleEndian.PutUint32

	get16 = binary.LittleEndian.Uint16
	get32 = binary.LittleEndian.Uint32
)
