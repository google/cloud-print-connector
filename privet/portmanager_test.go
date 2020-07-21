/*
Copyright 2016 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package privet

import "testing"

const portLow = 26000

func TestListen_available1(t *testing.T) {
	pm := newPortManager(portLow, portLow)

	l1, err := pm.listen()
	if err != nil {
		t.Fatal(err)
	}
	if l1.port() != portLow {
		t.Logf("Expected port %d, got port %d", portLow, l1.port())
		l1.Close()
		t.FailNow()
	}

	l2, err := pm.listen()
	if err == nil {
		l1.Close()
		l2.Close()
		t.Fatal("Expected error when too many ports opened")
	}

	err = l1.Close()
	if err != nil {
		t.Fatal(err)
	}

	l3, err := pm.listen()
	if err != nil {
		t.Fatal(err)
	}
	if l3.port() != portLow {
		t.Logf("Expected port %d, got port %d", portLow, l3.port())
	}
	l3.Close()
}

func TestListen_available2(t *testing.T) {
	pm := newPortManager(portLow, portLow+1)

	l1, err := pm.listen()
	if err != nil {
		t.Fatal(err)
	}
	if l1.port() != portLow {
		t.Logf("Expected port %d, got port %d", portLow, l1.port())
		l1.Close()
		t.FailNow()
	}

	l2, err := pm.listen()
	if err != nil {
		t.Fatal(err)
	}
	if l2.port() != portLow+1 {
		t.Logf("Expected port %d, got port %d", portLow+1, l2.port())
		l2.Close()
		t.FailNow()
	}

	l3, err := pm.listen()
	if err == nil {
		l1.Close()
		l2.Close()
		l3.Close()
		t.Fatal("Expected error when too many ports opened")
	}

	err = l2.Close()
	if err != nil {
		l1.Close()
		t.Fatal(err)
	}

	l4, err := pm.listen()
	if err != nil {
		t.Fatal(err)
	}
	if l4.port() != portLow+1 {
		t.Logf("Expected port %d, got port %d", portLow+1, l4.port())
	}

	l5, err := pm.listen()
	if err == nil {
		l1.Close()
		l4.Close()
		l5.Close()
		t.Fatal("Expected error when too many ports opened")
	}

	err = l1.Close()
	if err != nil {
		l4.Close()
		t.Fatal(err)
	}

	l6, err := pm.listen()
	if err != nil {
		t.Fatal(err)
	}
	if l6.port() != portLow {
		t.Logf("Expected port %d, got port %d", portLow, l6.port())
	}
	l4.Close()
	l6.Close()
}

// openPorts attempts to open n ports, where m are available.
func openPorts(n, m uint16) {
	pm := newPortManager(portLow, portLow+m-1)
	for i := uint16(0); i < n; i++ {
		l, err := pm.listen()
		if err == nil {
			defer l.Close()
		}
	}
}

func BenchmarkListen_range1_available1(*testing.B) {
	openPorts(1, 1)
}

func BenchmarkListen_range10_available10(*testing.B) {
	openPorts(10, 10)
}

func BenchmarkListen_range100_available100(*testing.B) {
	openPorts(100, 100)
}

func BenchmarkListen_range1000_available1000(*testing.B) {
	openPorts(1000, 1000)
}

func BenchmarkListen_range10_available1(*testing.B) {
	openPorts(10, 1)
}

func BenchmarkListen_range100_available10(*testing.B) {
	openPorts(100, 10)
}

func BenchmarkListen_range1000_available100(*testing.B) {
	openPorts(1000, 100)
}
