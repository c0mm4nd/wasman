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
			actual, err := segments.ReadDataSegment(bytes.NewBuffer(c.bytes))
			if err != nil {
				t.Fail()
			}
			if !reflect.DeepEqual(c.exp, actual) {
				t.Fail()
			}
		})
	}
}
