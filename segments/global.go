package segments

import (
	"bytes"
	"fmt"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/types"
)

// GlobalSegment is one unit of the wasm.Module's GlobalSection
type GlobalSegment struct {
	Type *types.GlobalType
	Init *expr.Expression
}

// ReadGlobalSegment reads one GlobalSegment from the io.Reader
func ReadGlobalSegment(r *bytes.Reader) (*GlobalSegment, error) {
	gt, err := types.ReadGlobalType(r)
	if err != nil {
		return nil, fmt.Errorf("read global type: %w", err)
	}

	init, err := expr.ReadExpression(r)
	if err != nil {
		return nil, fmt.Errorf("get init expression: %w", err)
	}

	return &GlobalSegment{
		Type: gt,
		Init: init,
	}, nil
}
