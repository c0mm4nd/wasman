package expr

import (
	"bytes"
	"fmt"
	"github.com/c0mm4nd/wasman/leb128"
	"github.com/c0mm4nd/wasman/types"
	"github.com/c0mm4nd/wasman/utils"
	"io"
)

// Expression is sequences of instructions terminated by an end marker.
type Expression struct {
	OpCode OpCode
	Data   []byte
}

// ReadExpression will read an expr.Expression from the io.Reader
func ReadExpression(r io.Reader) (*Expression, error) {
	b := make([]byte, 1)
	_, err := io.ReadFull(r, b)
	if err != nil {
		return nil, fmt.Errorf("read opcodes.OpCode: %w", err)
	}
	buf := new(bytes.Buffer)
	teeR := io.TeeReader(r, buf)

	op := OpCode(b[0])
	switch op {
	case OpCodeI32Const:
		_, _, err = leb128.DecodeInt32(teeR)
	case OpCodeI64Const:
		_, _, err = leb128.DecodeInt64(teeR)
	case OpCodeF32Const:
		_, err = utils.ReadFloat32(teeR)
	case OpCodeF64Const:
		_, err = utils.ReadFloat64(teeR)
	case OpCodeGlobalGet:
		_, _, err = leb128.DecodeUint32(teeR)
	default:
		return nil, fmt.Errorf("%w for opcodes.OpCode: %#x", types.ErrInvalidByte, b[0])
	}

	if err != nil {
		return nil, fmt.Errorf("read value: %w", err)
	}

	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("look for end opcodes.OpCode: %w", err)
	}

	if b[0] != byte(OpCodeEnd) {
		return nil, fmt.Errorf("constant expression has not terminated")
	}

	return &Expression{
		OpCode: op,
		Data:   buf.Bytes(),
	}, nil
}
