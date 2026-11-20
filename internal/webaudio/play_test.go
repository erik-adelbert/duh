// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package webaudio

import "testing"

func TestPlayRejectsInvalidByteRate(t *testing.T) {
	if err := Play(nil, nil, 0); err == nil {
		t.Fatal("expected an error for a zero audio byte rate")
	}
}
