/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

// Package oid implements SNMP OID and related data structures.
package oid

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// OID reprents a numeric object ID.
type OID []uint

// NewOID creates a new OID from a string. If the name argument contains
// invalid characters, like non-integers, then the returned OID is OID{}.
func NewOID(name string) OID {
	if strings.HasPrefix(name, ".") {
		name = name[1:]
	}
	if strings.HasSuffix(name, ".") {
		name = name[:len(name)-1]
	}
	q := strings.Split(name, ".")
	var o = make([]uint, len(q))
	for i := range q {
		value, err := strconv.ParseUint(q[i], 10, 32)
		if err != nil {
			// TODO is this what I want to do?
			return OID{}
		}
		o[i] = uint(value)
	}

	return o
}

// AsString formats the OID as a string.
func (o OID) AsString() string {
	q := make([]string, len(o))
	for i := range o {
		q[i] = fmt.Sprintf("%d", o[i])
	}
	return strings.Join(q, ".")
}

// HasPrefix answers the question "does this OID have this prefix?"
func (a OID) HasPrefix(b OID) bool {
	if len(a) < len(b) {
		return false
	}

	for i := 0; i < len(b); i++ {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// IsEqualTo checks whether this OID == that OID.
func (a OID) IsEqualTo(b OID) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// ComesBefore answers the question "does this OID sort before that OID?"
func (a OID) ComesBefore(b OID) bool {
	var size int
	if len(a) < len(b) {
		size = len(a)
	} else {
		size = len(b)
	}

	for i := 0; i < size; i++ {
		if a[i] < b[i] {
			return true
		}
		if a[i] > b[i] {
			return false
		}
	}

	if len(a) < len(b) {
		return true
	}

	return false
}

// Variable represents an OID name:value pair.
type Variable struct {
	Name  OID
	Value string
}

// NameAsString formats OID.Name as a string.
func (v *Variable) NameAsString() string {
	return v.Name.AsString()
}

// VariableSet represents an ordered set of OID variables.
type VariableSet struct {
	vars []Variable
}

// Size gets the current size of this set.
func (vs *VariableSet) Size() int {
	return len(vs.vars)
}

// Returns the variables in this set.
func (vs *VariableSet) Variables() []Variable {
	return vs.vars
}

// AddVariable adds a variable to this set.
func (vs *VariableSet) AddVariable(name OID, value string) {
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		value = value[1 : len(value)-1]
	}
	vs.vars = append(vs.vars, Variable{name, value})
}

// GetSubtree gets the subtree of variables whose OID name has prefix.
func (vs *VariableSet) GetSubtree(prefix OID) *VariableSet {
	head := sort.Search(len(vs.vars), func(i int) bool { return !vs.vars[i].Name.ComesBefore(prefix) })
	tail := head
	for tail < len(vs.vars) && vs.vars[tail].Name.HasPrefix(prefix) {
		tail++
	}
	return &VariableSet{vs.vars[head:tail]}
}

// GetValues gets all values in this set.
func (vs *VariableSet) GetValues() []string {
	values := make([]string, len(vs.vars))
	for i := range vs.vars {
		values[i] = vs.vars[i].Value
	}
	return values
}

// GetVariable gets a single variable, by OID.
func (vs *VariableSet) GetVariable(o OID) (*Variable, bool) {
	subtree := vs.GetSubtree(o)
	if len(subtree.vars) > 0 && subtree.vars[0].Name.IsEqualTo(o) {
		return &subtree.vars[0], true
	}
	return nil, false
}

// GetValue gets the value of a single variable, by OID.
func (vs *VariableSet) GetValue(o OID) (string, bool) {
	if v, exists := vs.GetVariable(o); exists {
		return v.Value, true
	}
	return "", false
}
