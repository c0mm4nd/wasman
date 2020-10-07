package instr_test

import (
	"bytes"
	"encoding/binary"
	"github.com/c0mm4nd/wasman/instr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func TestReadExpr(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		for _, b := range [][]byte{
			{}, {0xaa}, {0x41, 0x1}, {0x41, 0x01, 0x41}, // all invalid
		} {
			_, err := instr.ReadExpr(bytes.NewBuffer(b))
			assert.Error(t, err)
			t.Log(err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		for _, c := range []struct {
			bytes []byte
			exp   *instr.Expr
		}{
			{
				bytes: []byte{0x42, 0x01, 0x0b},
				exp:   &instr.Expr{OpCode: instr.OpCodeI64Const, Data: []byte{0x01}},
			},
			{
				bytes: []byte{0x43, 0x40, 0xe1, 0x47, 0x40, 0x0b},
				exp:   &instr.Expr{OpCode: instr.OpCodeF32Const, Data: []byte{0x40, 0xe1, 0x47, 0x40}},
			},
			{
				bytes: []byte{0x23, 0x01, 0x0b},
				exp:   &instr.Expr{OpCode: instr.OpCodeGlobalGet, Data: []byte{0x01}},
			},
		} {
			actual, err := instr.ReadExpr(bytes.NewBuffer(c.bytes))
			assert.NoError(t, err)
			assert.Equal(t, c.exp, actual)
		}
	})
}

func TestReadFloat32(t *testing.T) {
	var exp float32 = 3.1231231231
	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, math.Float32bits(exp))
	actual, err := instr.ReadFloat32(bytes.NewBuffer(bs))
	require.NoError(t, err)
	assert.Equal(t, exp, actual)
}

func TestReadFloat64(t *testing.T) {
	exp := 3.1231231231
	bs := make([]byte, 8)
	binary.LittleEndian.PutUint64(bs, math.Float64bits(exp))

	actual, err := instr.ReadFloat64(bytes.NewBuffer(bs))
	require.NoError(t, err)
	assert.Equal(t, exp, actual)
}
