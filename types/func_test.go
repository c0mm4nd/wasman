package types_test

import (
	"bytes"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/c0mm4nd/wasman/types"
)

func TestReadFunctionType(t *testing.T) {
	t.Run("ng", func(t *testing.T) {
		buf := []byte{0x00}
		_, err := types.ReadFuncType(bytes.NewReader(buf))
		if !errors.Is(err, types.ErrInvalidTypeByte) {
			t.Fail()
			t.Log(err)
		}
	})

	for i, c := range []struct {
		bytes []byte
		exp   *types.FuncType
	}{
		{
			bytes: []byte{0x60, 0x0, 0x0},
			exp: &types.FuncType{
				InputTypes:  []types.ValueType{},
				ReturnTypes: []types.ValueType{},
			},
		},
		{
			bytes: []byte{0x60, 0x2, 0x7f, 0x7e, 0x0},
			exp: &types.FuncType{
				InputTypes:  []types.ValueType{types.ValueTypeI32, types.ValueTypeI64},
				ReturnTypes: []types.ValueType{},
			},
		},
		{
			bytes: []byte{0x60, 0x1, 0x7e, 0x2, 0x7f, 0x7e},
			exp: &types.FuncType{
				InputTypes:  []types.ValueType{types.ValueTypeI64},
				ReturnTypes: []types.ValueType{types.ValueTypeI32, types.ValueTypeI64},
			},
		},
		{
			bytes: []byte{0x60, 0x0, 0x2, 0x7f, 0x7e},
			exp: &types.FuncType{
				InputTypes:  []types.ValueType{},
				ReturnTypes: []types.ValueType{types.ValueTypeI32, types.ValueTypeI64},
			},
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual, err := types.ReadFuncType(bytes.NewReader(c.bytes))
			if err != nil {
				t.Fail()
			}
			if !reflect.DeepEqual(c.exp, actual) {
				t.Fail()
			}
		})
	}
}

func TestReadFunctionType_errors(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   string
	}{
		// truncated before the leading 0x60 byte
		{bytes: []byte{}, exp: "read leading byte: EOF"},
		// truncated before the input vector size
		{bytes: []byte{0x60}, exp: "get the size of input value types: EOF"},
		// input vector size 1 but no value type byte follows
		{bytes: []byte{0x60, 0x01}, exp: "read value types of inputs: EOF"},
		// truncated before the output vector size
		{bytes: []byte{0x60, 0x00}, exp: "get the size of output value types: EOF"},
		// output vector size 1 but no value type byte follows
		{bytes: []byte{0x60, 0x00, 0x01}, exp: "read value types of outputs: EOF"},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := types.ReadFuncType(bytes.NewReader(c.bytes))
			if err == nil || err.Error() != c.exp {
				t.Errorf("got %v, want %q", err, c.exp)
			}
		})
	}
}
