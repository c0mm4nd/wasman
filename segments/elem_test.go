package segments_test

import (
	"bytes"
	"reflect"
	"strconv"
	"testing"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/segments"
)

func TestReadElementSegment(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   *segments.ElemSegment
	}{
		{
			// flag 0: active, table 0, offset expr, funcidx vec
			bytes: []byte{0x00, 0x41, 0x1, 0x0b, 0x02, 0x05, 0x07},
			exp: &segments.ElemSegment{
				TableIndex: 0,
				OffsetExpr: &expr.Expression{
					OpCode: expr.OpCodeI32Const,
					Data:   []byte{0x01},
				},
				Init: []uint32{5, 7},
			},
		},
		{
			// flag 2: active, explicit tableidx 3, offset expr, elemkind, funcidx vec
			bytes: []byte{0x02, 0x03, 0x41, 0x04, 0x0b, 0x00, 0x01, 0x0a},
			exp: &segments.ElemSegment{
				TableIndex: 3,
				OffsetExpr: &expr.Expression{
					OpCode: expr.OpCodeI32Const,
					Data:   []byte{0x04},
				},
				Init: []uint32{10},
			},
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual, err := segments.ReadElemSegment(bytes.NewReader(c.bytes))
			if err != nil {
				t.Fail()
			}
			if !reflect.DeepEqual(c.exp, actual) {
				t.Fail()
			}
		})
	}
}
