package segments

import (
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/instr"
	"github.com/c0mm4nd/wasman/leb128"
)

type DataSegment struct {
	MemoryIndex      uint32 // supposed to be zero
	OffsetExpression *instr.Expr
	Init             []byte
}

func ReadDataSegment(r io.Reader) (*DataSegment, error) {
	d, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read memory index: %w", err)
	}

	if d != 0 {
		return nil, fmt.Errorf("invalid memory index: %d", d)
	}

	expression, err := instr.ReadExpr(r)
	if err != nil {
		return nil, fmt.Errorf("read offset expression: %w", err)
	}

	if expression.OpCode != instr.OpCodeI32Const {
		return nil, fmt.Errorf("offset expression must have i32.const opcodes.OpCode but go %#x", expression.OpCode)
	}

	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get the size of vector: %w", err)
	}

	b := make([]byte, vs)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read bytes for init: %w", err)
	}

	return &DataSegment{
		OffsetExpression: expression,
		Init:             b,
	}, nil
}
