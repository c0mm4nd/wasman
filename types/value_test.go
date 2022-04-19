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
			actual, err := types.ReadValueTypes(bytes.NewBuffer(c.bytes), c.num)
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
	actual, err := types.ReadNameValue(bytes.NewBuffer(buf))
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
