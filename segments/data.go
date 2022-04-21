package segments

import (
	"bytes"
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/leb128decode"
)

// DataSegment is one unit of the wasman.Module's DataSection, initializing
// a range of memory, at a given offset, with a static vector of bytes
//
// https://www.w3.org/TR/wasm-core-1/#data-segments%E2%91%A0
type DataSegment struct {
	MemoryIndex      uint32 // supposed to be zero
	OffsetExpression *expr.Expression
	Init             []byte
}

// ReadDataSegment reads one DataSegment from the io.Reader
func ReadDataSegment(r *bytes.Reader) (*DataSegment, error) {
	d, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read memory index: %w", err)
	}

	if d != 0 {
		return nil, fmt.Errorf("invalid memory index: %d", d)
	}

	expression, err := expr.ReadExpression(r)
	if err != nil {
		return nil, fmt.Errorf("read offset expression: %w", err)
	}

	if expression.OpCode != expr.OpCodeI32Const {
		return nil, fmt.Errorf("offset expression must have i32.const opcodes.OpCode but go %#x", expression.OpCode)
	}

	vs, _, err := leb128decode.DecodeUint32(r)
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
