package types

import (
	"fmt"
	"io"
)

// TableType classify tables over elements of element types within a size range.
// https://www.w3.org/TR/wasm-core-1/#table-types%E2%91%A0
type TableType struct {
	Elem   byte
	Limits *Limits
}

// ReadTableType will read a types.TableType from the io.Reader
func ReadTableType(r io.Reader) (*TableType, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read leading byte: %w", err)
	}

	if b[0] != 0x70 {
		return nil, fmt.Errorf("%w: invalid element type %#x != %#x", ErrInvalidTypeByte, b[0], 0x70)
	}

	lm, err := ReadLimits(r)
	if err != nil {
		return nil, fmt.Errorf("read limits: %w", err)
	}

	return &TableType{
		Elem:   0x70,
		Limits: lm,
	}, nil
}
