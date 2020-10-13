package segments_test

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/segments"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadElementSegment(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   *segments.ElemSegment
	}{
		{
			bytes: []byte{0xa, 0x41, 0x1, 0x0b, 0x02, 0x05, 0x07},
			exp: &segments.ElemSegment{
				TableIndex: 10,
				OffsetExpr: &expr.Expression{
					OpCode: expr.OpCodeI32Const,
					Data:   []byte{0x01},
				},
				Init: []uint32{5, 7},
			},
		},
		{
			bytes: []byte{0x3, 0x41, 0x04, 0x0b, 0x01, 0x0a},
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
			actual, err := segments.ReadElemSegment(bytes.NewBuffer(c.bytes))
			require.NoError(t, err)
			assert.Equal(t, c.exp, actual)
		})
	}
}
