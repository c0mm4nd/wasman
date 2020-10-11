package segments

import (
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/leb128decode"
)

type ElemSegment struct {
	TableIndex uint32
	OffsetExpr *expr.Expression
	Init       []uint32
}

func ReadElemSegment(r io.Reader) (*ElemSegment, error) {
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
