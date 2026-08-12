package expr

import (
	"bytes"
	"fmt"

	"github.com/c0mm4nd/wasman/leb128decode"
	"github.com/c0mm4nd/wasman/types"
	"github.com/c0mm4nd/wasman/utils"
)

// Expression is a (constant) sequence of instructions terminated by an end marker.
type Expression struct {
	OpCode OpCode
	Data   []byte
	// Extended is set for extended constant expressions, i.e. those made of more
	// than a single instruction (e.g. i32.add of two i32.const). Raw then holds
	// the whole instruction sequence (without the terminating end) for the
	// evaluator; OpCode/Data still describe the first instruction.
	Extended bool
	Raw      []byte
}

// ReadExpression will read an expr.Expression from the io.Reader.
//
// It accepts both the plain MVP constant expressions (a single i32/i64/f32/f64
// const or global.get) and the extended constant expressions (arithmetic over
// constants: i32/i64 add/sub/mul).
func ReadExpression(r *bytes.Reader) (*Expression, error) {
	total := r.Size()
	startOff := total - int64(r.Len())

	var firstOp OpCode
	var firstData []byte
	instrCount := 0

	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read opcode: %w", err)
		}
		op := OpCode(b)
		if op == OpCodeEnd {
			break
		}

		immStart := total - int64(r.Len())
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
		case OpCodeI32Add, OpCodeI32Sub, OpCodeI32Mul,
			OpCodeI64Add, OpCodeI64Sub, OpCodeI64Mul:
			// extended-const arithmetic: no immediate
		default:
			return nil, fmt.Errorf("%v for opcodes.OpCode: %#x", types.ErrInvalidTypeByte, b)
		}
		if err != nil {
			return nil, fmt.Errorf("read value: %w", err)
		}
		immEnd := total - int64(r.Len())

		if instrCount == 0 {
			firstOp = op
			firstData = make([]byte, immEnd-immStart)
			if _, err := r.ReadAt(firstData, immStart); err != nil {
				return nil, fmt.Errorf("error re-buffering Expression Data")
			}
		}
		instrCount++
	}

	if instrCount == 0 {
		return nil, fmt.Errorf("empty constant expression")
	}

	expression := &Expression{
		OpCode: firstOp,
		Data:   firstData,
	}

	// only extended (multi-instruction) expressions need the raw sequence; the
	// single-instruction fast path keeps the historical shape.
	if instrCount != 1 {
		endOff := total - int64(r.Len()) // just after the end opcode
		raw := make([]byte, endOff-1-startOff)
		if _, err := r.ReadAt(raw, startOff); err != nil {
			return nil, fmt.Errorf("error re-buffering Expression")
		}
		expression.Extended = true
		expression.Raw = raw
	}

	return expression, nil
}
