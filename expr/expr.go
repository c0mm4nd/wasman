package expr

import (
	"bytes"
	"fmt"

	"github.com/c0mm4nd/wasman/leb128decode"
	"github.com/c0mm4nd/wasman/types"
	"github.com/c0mm4nd/wasman/utils"
)

// Expression is sequences of instructions terminated by an end marker.
type Expression struct {
	OpCode OpCode
	Data   []byte
}

// ReadExpression will read an expr.Expression from the io.Reader
func ReadExpression(r *bytes.Reader) (*Expression, error) {
	b, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read opcode: %v", err)
	}

	remainingBeforeData := int64(r.Len())
	offsetAtData := r.Size() - remainingBeforeData

	op := OpCode(b)

	switch op {
	case OpCodeI32Const:
		_, _, err = leb128decode.DecodeInt32(r)
	case OpCodeI64Const:
		_, _, err = leb128decode.DecodeInt64(r)
	case OpCodeF32Const:
		_, err = utils.ReadFloat32(r)
	case OpCodeF64Const:
		_, err = utils.ReadFloat64(r)
	case OpCodeGlobalGet:
		_, _, err = leb128decode.DecodeUint32(r)
	default:
		return nil, fmt.Errorf("%v for opcodes.OpCode: %#x", types.ErrInvalidTypeByte, b)
	}

	if err != nil {
		return nil, fmt.Errorf("read value: %v", err)
	}

	if b, err = r.ReadByte(); err != nil {
		return nil, fmt.Errorf("look for end opcode: %v", err)
	}

	if b != byte(OpCodeEnd) {
		return nil, fmt.Errorf("constant expression has not terminated")
	}

	data := make([]byte, remainingBeforeData-int64(r.Len())-1)
	if _, err := r.ReadAt(data, offsetAtData); err != nil {
		return nil, fmt.Errorf("error re-buffering Expression Data")
	}

	return &Expression{
		OpCode: op,
		Data:   data,
	}, nil
}
