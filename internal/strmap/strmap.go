// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// strmap package provides string-keyed map based on a radix trie.
package strmap

import "fmt"

func Example() {
	m := NewMap[int]()
	m.Set("foo", 42)
	m.Set("bar", 7)

	v, ok := m.Get("foo")
	if ok {
		fmt.Println("foo:", v)
	}

	v, ok = m.Get("bar")
	if ok {
		fmt.Println("bar:", v)
	}

	m.Delete("foo")

	_, ok = m.Get("foo")
	if !ok {
		fmt.Println("foo has been deleted")
	}
}
