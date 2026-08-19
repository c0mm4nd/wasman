package types_test

import (
	"bytes"
	"reflect"
	"strconv"
	"testing"

	"github.com/c0mm4nd/wasman/types"
)

func TestReadValueTypes(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		num   uint32
		exp   []types.ValueType
	}{
		{
			bytes: []byte{0x7e}, num: 1, exp: []types.ValueType{types.ValueTypeI64},
		},
		{
			bytes: []byte{0x7f, 0x7e}, num: 2, exp: []types.ValueType{types.ValueTypeI32, types.ValueTypeI64},
		},
		{
			bytes: []byte{0x7f, 0x7e, 0x7d}, num: 2, exp: []types.ValueType{types.ValueTypeI32, types.ValueTypeI64},
		},
		{
			bytes: []byte{0x7f, 0x7e, 0x7d, 0x7c}, num: 4,
			exp: []types.ValueType{types.ValueTypeI32, types.ValueTypeI64, types.ValueTypeF32, types.ValueTypeF64},
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual, err := types.ReadValueTypes(bytes.NewReader(c.bytes), c.num)
			if err != nil {
				t.Fail()
			}
			if !reflect.DeepEqual(c.exp, actual) {
				t.Fail()
			}
		})
	}
}

func TestReadNameValue(t *testing.T) {
	exp := "abcdefgh你好"
	l := len(exp)
	buf := []byte{byte(l)}
	buf = append(buf, exp...)
	actual, err := types.ReadNameValue(bytes.NewReader(buf))
	if err != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(exp, actual) {
		t.Fail()
	}
}

func TestHasSameValues(t *testing.T) {
	for _, c := range []struct {
		a, b []types.ValueType
		exp  bool
	}{
		{a: []types.ValueType{}, exp: true},
		{a: []types.ValueType{}, b: []types.ValueType{}, exp: true},
		{a: []types.ValueType{types.ValueTypeF64}, exp: false},
		{a: []types.ValueType{types.ValueTypeF64}, b: []types.ValueType{types.ValueTypeF64}, exp: true},
	} {
		if !reflect.DeepEqual(c.exp, types.HasSameSignature(c.a, c.b)) {
			t.Fail()
		}
	}
}

func TestValueTypeString(t *testing.T) {
	for _, c := range []struct {
		v   types.ValueType
		exp string
	}{
		{v: types.ValueTypeI32, exp: "i32"},
		{v: types.ValueTypeI64, exp: "i64"},
		{v: types.ValueTypeF32, exp: "f32"},
		{v: types.ValueTypeF64, exp: "f64"},
		{v: types.ValueType(0x00), exp: "unknown value type"},
	} {
		if c.v.String() != c.exp {
			t.Errorf("got %q, want %q", c.v.String(), c.exp)
		}
	}
}

func TestReadValueTypes_large(t *testing.T) {
	// more than one 1024-byte chunk, so the bounded chunking loop runs twice
	num := uint32(1025)
	buf := bytes.Repeat([]byte{0x7f}, int(num))
	actual, err := types.ReadValueTypes(bytes.NewReader(buf), num)
	if err != nil {
		t.Fatal(err)
	}
	if uint32(len(actual)) != num {
		t.Fatalf("got %d value types, want %d", len(actual), num)
	}
	for i, vt := range actual {
		if vt != types.ValueTypeI32 {
			t.Fatalf("actual[%d] = %#x, want i32", i, vt)
		}
	}
}

func TestReadValueTypes_errors(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		num   uint32
		exp   string
	}{
		// no bytes at all
		{bytes: []byte{}, num: 1, exp: "EOF"},
		// fewer bytes than requested
		{bytes: []byte{0x7f}, num: 2, exp: "unexpected EOF"},
		// 0x01 is not a value type
		{bytes: []byte{0x01}, num: 1, exp: "invalid value type: 1"},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := types.ReadValueTypes(bytes.NewReader(c.bytes), c.num)
			if err == nil || err.Error() != c.exp {
				t.Errorf("got %v, want %q", err, c.exp)
			}
		})
	}
}

func TestReadNameValue_errors(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   string
	}{
		// truncated before the size
		{bytes: []byte{}, exp: "read size of name: EOF"},
		// declared length larger than the remaining input
		{bytes: []byte{0x05, 'a'}, exp: "name length 5 exceeds remaining input"},
		// invalid UTF-8 bytes
		{bytes: []byte{0x02, 0xff, 0xfe}, exp: "name is not valid UTF-8"},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := types.ReadNameValue(bytes.NewReader(c.bytes))
			if err == nil || err.Error() != c.exp {
				t.Errorf("got %v, want %q", err, c.exp)
			}
		})
	}
}

func TestHasSameValues_mismatch(t *testing.T) {
	// same length but different element types
	a := []types.ValueType{types.ValueTypeI32}
	b := []types.ValueType{types.ValueTypeI64}
	if types.HasSameSignature(a, b) {
		t.Fail()
	}
}
