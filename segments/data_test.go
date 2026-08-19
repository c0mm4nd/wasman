package segments_test

import (
	"bytes"
	"reflect"
	"strconv"
	"testing"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/segments"
)

func TestDataSegment(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   *segments.DataSegment
	}{
		{
			bytes: []byte{0x0, 0x41, 0x1, 0x0b, 0x02, 0x05, 0x07},
			exp: &segments.DataSegment{
				OffsetExpression: &expr.Expression{
					OpCode: expr.OpCodeI32Const,
					Data:   []byte{0x01},
				},
				Init: []byte{5, 7},
			},
		},
		{
			bytes: []byte{0x0, 0x41, 0x04, 0x0b, 0x01, 0x0a},
			exp: &segments.DataSegment{
				OffsetExpression: &expr.Expression{
					OpCode: expr.OpCodeI32Const,
					Data:   []byte{0x04},
				},
				Init: []byte{0x0a},
			},
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual, err := segments.ReadDataSegment(bytes.NewReader(c.bytes))
			if err != nil {
				t.Fail()
			}
			if !reflect.DeepEqual(c.exp, actual) {
				t.Fail()
			}
		})
	}
}

func TestDataSegment_flags(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   *segments.DataSegment
	}{
		{
			// flag 1: passive, bytes only
			bytes: []byte{0x01, 0x02, 0x05, 0x07},
			exp: &segments.DataSegment{
				Passive: true,
				Init:    []byte{5, 7},
			},
		},
		{
			// flag 2: active with explicit memory index 3
			bytes: []byte{0x02, 0x03, 0x41, 0x01, 0x0b, 0x01, 0x0a},
			exp: &segments.DataSegment{
				MemoryIndex: 3,
				OffsetExpression: &expr.Expression{
					OpCode: expr.OpCodeI32Const,
					Data:   []byte{0x01},
				},
				Init: []byte{0x0a},
			},
		},
		{
			// flag 0 with a global.get offset expression
			bytes: []byte{0x00, 0x23, 0x02, 0x0b, 0x01, 0x0a},
			exp: &segments.DataSegment{
				OffsetExpression: &expr.Expression{
					OpCode: expr.OpCodeGlobalGet,
					Data:   []byte{0x02},
				},
				Init: []byte{0x0a},
			},
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual, err := segments.ReadDataSegment(bytes.NewReader(c.bytes))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(c.exp, actual) {
				t.Errorf("got %#v, want %#v", actual, c.exp)
			}
		})
	}
}

func TestDataSegment_errors(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   string
	}{
		// truncated before the flag
		{bytes: []byte{}, exp: "read data segment flag: EOF"},
		// 5 is not a valid data segment flag
		{bytes: []byte{0x05}, exp: "invalid data segment flag: 5"},
		// flag 2 but the memory index is missing
		{bytes: []byte{0x02}, exp: "read memory index: EOF"},
		// flag 0 but the offset expression is missing
		{bytes: []byte{0x00}, exp: "read offset expression: read opcode: EOF"},
		// i64.const is not a valid offset expression
		{bytes: []byte{0x00, 0x42, 0x01, 0x0b}, exp: "offset expression must be i32.const or global.get but got 0x42"},
		// truncated before the init vector size
		{bytes: []byte{0x00, 0x41, 0x01, 0x0b}, exp: "get the size of vector: EOF"},
		// init vector size larger than the remaining input
		{bytes: []byte{0x00, 0x41, 0x01, 0x0b, 0x05}, exp: "data segment size 5 exceeds remaining input"},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := segments.ReadDataSegment(bytes.NewReader(c.bytes))
			if err == nil || err.Error() != c.exp {
				t.Errorf("got %v, want %q", err, c.exp)
			}
		})
	}
}
