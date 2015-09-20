/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package lib

import (
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"reflect"
	"sort"
)

// DeepHash writes an object's values to h.
// Struct member names are ignored, values are written to h.
// Map keys and values are written to h.
// Slice inde and values are written to h.
// Pointers are followed once.
// Recursive pointer references cause panic.
func DeepHash(data interface{}, h hash.Hash) {
	visited := map[uintptr]struct{}{}
	deepHash(h, reflect.ValueOf(data), visited)
}

func binWrite(h hash.Hash, d interface{}) {
	binary.Write(h, binary.BigEndian, d)
}

type sortableValues []reflect.Value

func (sv sortableValues) Len() int      { return len(sv) }
func (sv sortableValues) Swap(i, j int) { sv[i], sv[j] = sv[j], sv[i] }
func (sv sortableValues) Less(i, j int) bool {
	switch sv[i].Kind() {
	case reflect.Bool:
		return sv[i].Bool() == false && sv[j].Bool() == true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return sv[i].Int() < sv[i].Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return sv[i].Uint() < sv[i].Uint()
	case reflect.Float32, reflect.Float64:
		return sv[i].Float() < sv[i].Float()
	case reflect.String:
		return sv[i].String() < sv[j].String()
	case reflect.Ptr:
		return sv[i].Pointer() < sv[i].Pointer()
	default:
		panic(fmt.Sprintf("Cannot compare type %s", sv[i].Kind().String()))
	}
}

func deepHash(h hash.Hash, v reflect.Value, visited map[uintptr]struct{}) {
	switch v.Kind() {
	case reflect.Invalid:
		h.Write([]byte{0})
	case reflect.Bool:
		if v.Bool() {
			binWrite(h, uint8(1))
		} else {
			binWrite(h, uint8(0))
		}
	case reflect.Int:
		binWrite(h, int(v.Int()))
	case reflect.Int8:
		binWrite(h, int8(v.Int()))
	case reflect.Int16:
		binWrite(h, int16(v.Int()))
	case reflect.Int32:
		binWrite(h, int32(v.Int()))
	case reflect.Int64:
		binWrite(h, int64(v.Int()))
	case reflect.Uint:
		binWrite(h, uint(v.Uint()))
	case reflect.Uint8:
		binWrite(h, uint8(v.Uint()))
	case reflect.Uint16:
		binWrite(h, uint16(v.Uint()))
	case reflect.Uint32:
		binWrite(h, uint32(v.Uint()))
	case reflect.Uint64:
		binWrite(h, uint64(v.Uint()))
	case reflect.Float32:
		binWrite(h, float32(v.Float()))
	case reflect.Float64:
		binWrite(h, float64(v.Float()))
	case reflect.Complex64:
		binWrite(h, float32(real(v.Complex())))
		binWrite(h, float32(imag(v.Complex())))
	case reflect.Complex128:
		binWrite(h, float64(real(v.Complex())))
		binWrite(h, float64(imag(v.Complex())))
	case reflect.Map:
		keys := make(sortableValues, 0, v.Len())
		for _, key := range v.MapKeys() {
			keys = append(keys, key)
		}
		sort.Sort(keys)
		for _, key := range keys {
			io.WriteString(h, key.String())
			deepHash(h, v.MapIndex(key), visited)
		}
	case reflect.Ptr:
		if _, exists := visited[v.Pointer()]; exists {
			panic("Cannot hash recursive structure")
		} else {
			visited[v.Pointer()] = struct{}{}
			deepHash(h, v.Elem(), visited)
			delete(visited, v.Pointer())
		}
	case reflect.Slice, reflect.Array:
		for i, l := 0, v.Len(); i < l; i++ {
			binWrite(h, i)
			deepHash(h, v.Index(i), visited)
		}
	case reflect.String:
		io.WriteString(h, v.String())
	case reflect.Struct:
		for i, n := 0, v.NumField(); i < n; i++ {
			io.WriteString(h, v.Type().Field(i).Name)
			deepHash(h, v.Field(i), visited)
		}
	default:
		message := fmt.Sprintf("DeepHash not implemented for '%s' type", v.Kind().String())
		panic(message)
	}
}
