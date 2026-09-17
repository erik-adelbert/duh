// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vgmo3

import (
	"encoding/binary"
	"fmt"
)

func hi8(x int) byte {
	return byte((x >> 8) & 0xff)
}

func hi4(x uint8) uint8 {
	return (x >> 4) & 0xf
}

func lo8(x int) byte {
	return byte(x & 0xff)
}

func lo4(x uint8) uint8 {
	return x & 0xf
}

type bcd uint32

func (b bcd) String() string {
	return fmt.Sprintf("%x", uint32(b))
}

var ble = binary.LittleEndian
