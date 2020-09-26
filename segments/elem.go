package segments

import (
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/instr"
	"github.com/c0mm4nd/wasman/leb128"
)

type ElemSegment struct {
	TableIndex uint32
	OffsetExpr *instr.Expr
	Init       []uint32
}

func ReadElemSegment(r io.Reader) (*ElemSegment, error) {
	ti, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get table index: %w", err)
	}

	expression, err := instr.ReadExpr(r)
	if err != nil {
		return nil, fmt.Errorf("read expr for offset: %w", err)
	}

	if expression.OpCode != instr.OpCodeI32Const {
		return nil, fmt.Errorf("offset expression must be i32.const but go %#x", expression.OpCode)
	}

	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get size of vector: %w", err)
	}

	init := make([]uint32, vs)
	for i := range init {
		fIDx, _, err := leb128.DecodeUint32(r)
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
