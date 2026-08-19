package types_test

import (
	"bytes"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/c0mm4nd/wasman/types"
)

func TestReadGlobalType(t *testing.T) {
	t.Run("ng", func(t *testing.T) {
		buf := []byte{0x7e, 0x3}
		_, err := types.ReadGlobalType(bytes.NewReader(buf))
		if !errors.Is(err, types.ErrInvalidTypeByte) {
			t.Log(err)
			t.Fail()
		}
	})

	for i, c := range []struct {
		bytes []byte
		exp   *types.GlobalType
	}{
		{bytes: []byte{0x7e, 0x00}, exp: &types.GlobalType{ValType: types.ValueTypeI64, Mutable: false}},
		{bytes: []byte{0x7e, 0x01}, exp: &types.GlobalType{ValType: types.ValueTypeI64, Mutable: true}},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual, err := types.ReadGlobalType(bytes.NewReader(c.bytes))
			if err != nil {
				t.Fail()
			}
			if !reflect.DeepEqual(c.exp, actual) {
				t.Fail()
			}
		})
	}
}

func TestReadGlobalType_errors(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   string
	}{
		// truncated before the value type
		{bytes: []byte{}, exp: "read value type: EOF"},
		// truncated before the mutability byte
		{bytes: []byte{0x7e}, exp: "read mutablity: EOF"},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := types.ReadGlobalType(bytes.NewReader(c.bytes))
			if err == nil || err.Error() != c.exp {
				t.Errorf("got %v, want %q", err, c.exp)
			}
		})
	}
}
