package segments

import (
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/instr"
	"github.com/c0mm4nd/wasman/types"
)

type GlobalSegment struct {
	Type *types.GlobalType
	Init *instr.Expr
}

func ReadGlobalSegment(r io.Reader) (*GlobalSegment, error) {
	gt, err := types.ReadGlobalType(r)
	if err != nil {
		return nil, fmt.Errorf("read global type: %w", err)
	}

	init, err := instr.ReadExpr(r)
	if err != nil {
		return nil, fmt.Errorf("get init expression: %w", err)
	}

	return &GlobalSegment{
		Type: gt,
		Init: init,
	}, nil
}
