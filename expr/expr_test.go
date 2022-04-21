package expr_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/c0mm4nd/wasman/expr"
)

func TestReadExpr(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		for _, b := range [][]byte{
			{}, {0xaa}, {0x41, 0x1}, {0x41, 0x01, 0x41}, // all invalid
		} {
			_, err := expr.ReadExpression(bytes.NewReader(b))
			t.Log(err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		for _, c := range []struct {
			bytes []byte
			exp   *expr.Expression
		}{
			{
				bytes: []byte{0x42, 0x01, 0x0b},
				exp:   &expr.Expression{OpCode: expr.OpCodeI64Const, Data: []byte{0x01}},
			},
			{
				bytes: []byte{0x43, 0x40, 0xe1, 0x47, 0x40, 0x0b},
				exp:   &expr.Expression{OpCode: expr.OpCodeF32Const, Data: []byte{0x40, 0xe1, 0x47, 0x40}},
			},
			{
				bytes: []byte{0x23, 0x01, 0x0b},
				exp:   &expr.Expression{OpCode: expr.OpCodeGlobalGet, Data: []byte{0x01}},
			},
		} {
			actual, err := expr.ReadExpression(bytes.NewReader(c.bytes))
			if err != nil {
				t.Fail()
			}
			if !reflect.DeepEqual(c.exp, actual) {
				t.Fail()
			}
		}
	})
}
