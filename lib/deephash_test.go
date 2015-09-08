/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/
package lib

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"io"
	"testing"
)

func check(t *testing.T, expected []byte, data interface{}) {
	h := md5.New()
	DeepHash(data, h)
	got := h.Sum(nil)

	if bytes.Compare(expected, got) != 0 {
		t.Logf("expected %x got %x", expected, got)
		t.Fail()
	}
}

func TestBool(t *testing.T) {
	h := md5.New()
	binary.Write(h, binary.BigEndian, uint8(0))
	expected := h.Sum(nil)
	check(t, expected, false)

	h = md5.New()
	binary.Write(h, binary.BigEndian, uint8(1))
	expected2 := h.Sum(nil)
	check(t, expected2, true)
}

func TestInt(t *testing.T) {
	i := int(123456789)
	h := md5.New()
	binary.Write(h, binary.BigEndian, i)
	expected := h.Sum(nil)
	check(t, expected, i)

	i8 := int8(123)
	h = md5.New()
	binary.Write(h, binary.BigEndian, i8)
	expected = h.Sum(nil)
	check(t, expected, i8)

	// byte is an alias for uint8
	b := byte('Q')
	h = md5.New()
	binary.Write(h, binary.BigEndian, b)
	expected = h.Sum(nil)
	check(t, expected, b)

	// rune is an alias for int32
	r := rune('龍')
	h = md5.New()
	binary.Write(h, binary.BigEndian, r)
	expected = h.Sum(nil)
	check(t, expected, r)

	ui64 := uint64(123456789123456789)
	h = md5.New()
	binary.Write(h, binary.BigEndian, ui64)
	expected = h.Sum(nil)
	check(t, expected, ui64)
}

func TestFloat(t *testing.T) {
	f32 := float32(123456.789)
	h := md5.New()
	binary.Write(h, binary.BigEndian, f32)
	expected := h.Sum(nil)
	check(t, expected, f32)

	f64 := float64(123456789.123456789)
	h = md5.New()
	binary.Write(h, binary.BigEndian, f64)
	expected = h.Sum(nil)
	check(t, expected, f64)
}

func TestComplex(t *testing.T) {
	var c64 complex64 = complex(123456.789, 654321.987)
	h := md5.New()
	binary.Write(h, binary.BigEndian, float32(real(c64)))
	binary.Write(h, binary.BigEndian, float32(imag(c64)))
	expected := h.Sum(nil)
	check(t, expected, c64)

	var c128 complex128 = complex(123456789.123456789, 987654321.987654321)
	h = md5.New()
	binary.Write(h, binary.BigEndian, float64(real(c128)))
	binary.Write(h, binary.BigEndian, float64(imag(c128)))
	expected = h.Sum(nil)
	check(t, expected, c128)
}

func TestMap(t *testing.T) {
	m := map[string]string{"b": "B", "a": "A"}
	h := md5.New()
	io.WriteString(h, "a")
	io.WriteString(h, "A")
	io.WriteString(h, "b")
	io.WriteString(h, "B")
	expected := h.Sum(nil)
	check(t, expected, m)
}

func TestPtr(t *testing.T) {
	i := int8(1)
	h := md5.New()
	binary.Write(h, binary.BigEndian, i)
	expected := h.Sum(nil)

	// Sum should be the same, whether DeepHash(value), or DeepHash(&value).
	check(t, expected, i)
	check(t, expected, &i)
}

func TestPtrNil(t *testing.T) {
	h := md5.New()
	h.Write([]byte{0})
	expected := h.Sum(nil)
	check(t, expected, nil)
}

func TestSlice(t *testing.T) {
	a := []string{"abc", "def"}
	h := md5.New()
	io.WriteString(h, a[0])
	io.WriteString(h, a[1])
	expected := h.Sum(nil)
	check(t, expected, a)

	b := []int{1, 2, 3}
	h = md5.New()
	binary.Write(h, binary.BigEndian, b[0])
	binary.Write(h, binary.BigEndian, b[1])
	binary.Write(h, binary.BigEndian, b[2])
	expected = h.Sum(nil)
	check(t, expected, b)

	p := []*int{}
	x, y, z := int(1), int(2), int(3)
	p = append(p, &x, &y, &z, nil)
	h = md5.New()
	binary.Write(h, binary.BigEndian, x)
	binary.Write(h, binary.BigEndian, y)
	binary.Write(h, binary.BigEndian, z)
	h.Write([]byte{0})
	expected = h.Sum(nil)
	check(t, expected, p)
}

func TestString(t *testing.T) {
	s := "just a string"
	h := md5.New()
	io.WriteString(h, s)
	expected := h.Sum(nil)
	check(t, expected, s)

	type myString string
	var ms myString = myString(s)
	check(t, expected, ms)
}
