package segments_test

import (
	"bytes"
	"reflect"
	"strconv"
	"testing"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
)

func TestReadGlobalSegment(t *testing.T) {
	exp := &segments.GlobalSegment{
		Type: &types.GlobalType{ValType: types.ValueTypeI64, Mutable: false},
		Init: &expr.Expression{
			OpCode: expr.OpCodeI64Const,
			Data:   []byte{0x01},
		},
	}

	buf := []byte{0x7e, 0x00, 0x42, 0x01, 0x0b}
	actual, err := segments.ReadGlobalSegment(bytes.NewReader(buf))
	if err != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(exp, actual) {
		t.Fail()
	}
}

func TestReadGlobalSegment_errors(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   string
	}{
		// truncated before the global type
		{bytes: []byte{}, exp: "read global type: read value type: EOF"},
		// global type present but the init expression is missing
		{bytes: []byte{0x7e, 0x00}, exp: "get init expression: read opcode: EOF"},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := segments.ReadGlobalSegment(bytes.NewReader(c.bytes))
			if err == nil || err.Error() != c.exp {
				t.Errorf("got %v, want %q", err, c.exp)
			}
		})
	}
}
