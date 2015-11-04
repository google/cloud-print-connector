/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package log

import (
	"sort"
	"strconv"
	"testing"
)

func checkSort(s sortableNumberStrings) (bool, error) {
	for i := 0; i < s.Len()-1; i++ {
		a, err := strconv.Atoi(s[i])
		if err != nil {
			return false, err
		}
		b, err := strconv.Atoi(s[i+1])
		if err != nil {
			return false, err
		}
		if a > b {
			return false, nil
		}
	}
	return true, nil
}

func testSort(t *testing.T, s sortableNumberStrings) {
	sort.Sort(s)
	res, err := checkSort(s)
	if err != nil {
		t.Log(err)
		t.Fail()
	} else if !res {
		t.Logf("sort failed: %v", s)
		t.Fail()
	}
}

func TestSortableNumberStrings(t *testing.T) {
	s := sortableNumberStrings{"2", "1"}
	testSort(t, s)

	s = sortableNumberStrings{"100", "10", "1", "11"}
	testSort(t, s)

	s = sortableNumberStrings{"0100", "10", "01", "11"}
	testSort(t, s)

	s = sortableNumberStrings{"0100", "10", "11", "10"}
	testSort(t, s)
}
