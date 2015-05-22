/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package oid

import (
	"fmt"
	"reflect"
	"testing"
)

var vars = []Variable{
	Variable{OID{2}, ""},
	Variable{OID{5}, ""},
	Variable{OID{5, 5}, ""},
	Variable{OID{7, 7}, ""},
	Variable{OID{10}, ""},
	Variable{OID{10, 9}, ""},
	Variable{OID{10, 10}, ""},
	Variable{OID{10, 10, 10}, ""},
	Variable{OID{10, 10, 11}, ""},
	Variable{OID{10, 11}, ""},
	Variable{OID{10, 12}, ""},
	Variable{OID{14}, ""},
	Variable{OID{14, 8}, ""},
	Variable{OID{14, 9}, ""},
	Variable{OID{14, 10}, ""},
}

func TestComesBefore(t *testing.T) {
	e := "it is untrue that %v ComesBefore %v"
	e2 := "it is true that %v ComesBefore %v"

	for i := 0; i < len(vars)-1; i++ {
		for j := 0; j < len(vars)-1; j++ {
			a, b := vars[i], vars[j]
			r := a.Name.ComesBefore(b.Name)
			if i < j && !r {
				fmt.Println(i, a.Name, j, b.Name, r)
				t.Errorf(e2, a, b)
			} else if i >= j && r {
				fmt.Println(i, a.Name, j, b.Name, r)
				t.Errorf(e, a, b)
			}
		}
	}

	a, b := OID{1}, OID{2}
	if !a.ComesBefore(b) {
		t.Errorf(e, a, b)
	}

	a, b = OID{1}, OID{1, 1}
	if !a.ComesBefore(b) {
		t.Errorf(e, a, b)
	}

	a, b = OID{1}, OID{1}
	if a.ComesBefore(b) {
		t.Errorf(e, a, b)
	}
}

func TestHasPrefix(t *testing.T) {
	e := "it is untrue that %v HasPrefix %v"

	a, b := OID{1}, OID{1}
	if !a.HasPrefix(b) {
		t.Errorf(e, a, b)
	}

	a, b = OID{1, 1}, OID{1}
	if !a.HasPrefix(b) {
		t.Errorf(e, a, b)
	}
}

func TestIsEqualTo(t *testing.T) {
	e := "it is untrue that %v IsEqualTo %v"

	a, b := OID{1}, OID{1}
	if !a.IsEqualTo(b) {
		t.Errorf(e, a, b)
	}

	a, b = OID{1, 1}, OID{1, 1}
	if !a.IsEqualTo(b) {
		t.Errorf(e, a, b)
	}
}

var o = VariableSet{
	vars: vars,
}

func TestAddVariable(t *testing.T) {
	var o VariableSet
	o.AddVariable(OID{1}, "\"value\"")

	if o.Size() != 1 {
		t.Error("Size didn't change after calling o.AddVariable()")
	}

	v, exists := o.GetVariable(OID{1})
	if !exists {
		t.Error("OID doesn't exist")
	}
	if v.NameAsString() != "1" {
		t.Error("Wrong name stored calling o.AddVariable()")
	}
	if v.Value == "\"value\"" {
		t.Error("Matching double-quotes stored in value calling o.AddVariable()")
	}
	if v.Value != "value" {
		t.Error("Wrong value stored calling o.AddVariable()")
	}
}

func TestGetSubtree(t *testing.T) {
	e := "called o.GetSubtree(\"%v\")\n expected %v\n got %v"

	f := func(prefix OID, expected []Variable) {
		subtree := o.GetSubtree(prefix)
		if !reflect.DeepEqual(expected, subtree.vars) {
			t.Errorf(e, prefix, expected, subtree.vars)
		}
	}

	f(OID{1}, []Variable{})
	f(OID{2}, o.vars[0:1])
	f(OID{5}, o.vars[1:3])
	f(OID{5, 5}, o.vars[2:3])
	f(OID{7}, o.vars[3:4])
	f(OID{10}, o.vars[4:11])
	f(NewOID("10.10"), o.vars[6:9])
	f(NewOID(".10.10"), o.vars[6:9])
	f(NewOID("10.10."), o.vars[6:9])
	f(NewOID(".10.10."), o.vars[6:9])
}
