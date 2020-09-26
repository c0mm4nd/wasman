package types

import (
	"fmt"
	"io"
)

type TableType struct {
	Elem   byte
	Limits *Limits
}

func ReadTableType(r io.Reader) (*TableType, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read leading byte: %w", err)
	}

	if b[0] != 0x70 {
		return nil, fmt.Errorf("%w: invalid element type %#x != %#x", ErrInvalidByte, b[0], 0x70)
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
