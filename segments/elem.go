package segments

import (
	"bytes"
	"fmt"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/leb128decode"
)

// ElemSegment is one unit of the wasm.Module's ElementsSection, initializing
// a subrange of a table, at a given offset, from a static vector of elements.
//
// https://www.w3.org/TR/wasm-core-1/#element-segments%E2%91%A0
type ElemSegment struct {
	TableIndex uint32
	OffsetExpr *expr.Expression
	Init       []uint32
}

// ReadElemSegment reads one ElemSegment from the io.Reader
func ReadElemSegment(r *bytes.Reader) (*ElemSegment, error) {
	ti, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get table index: %w", err)
	}

	expression, err := expr.ReadExpression(r)
	if err != nil {
		return nil, fmt.Errorf("read expr for offset: %w", err)
	}

	if expression.OpCode != expr.OpCodeI32Const {
		return nil, fmt.Errorf("offset expression must be i32.const but go %#x", expression.OpCode)
	}

	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get size of vector: %w", err)
	}

	init := make([]uint32, vs)
	for i := range init {
		fIDx, _, err := leb128decode.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read function index: %w", err)
		}
		init[i] = fIDx
	}

	return &ElemSegment{
		TableIndex: ti,
		OffsetExpr: expression,
		Init:       init,
	}, nil
}
