package segments_test

import (
	"bytes"
	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
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
	actual, err := segments.ReadGlobalSegment(bytes.NewBuffer(buf))
	require.NoError(t, err)
	assert.Equal(t, exp, actual)
}
