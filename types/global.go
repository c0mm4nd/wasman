package types

import (
	"fmt"
	"io"
)

type GlobalType struct {
	ValType ValueType
	Mutable bool
}

func ReadGlobalType(r io.Reader) (*GlobalType, error) {
	vt, err := ReadValueTypes(r, 1)
	if err != nil {
		return nil, fmt.Errorf("read value type: %w", err)
	}

	ret := &GlobalType{
		ValType: vt[0],
	}

	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read mutablity: %w", err)
	}

	switch mut := b[0]; mut {
	case 0x00:
	case 0x01:
		ret.Mutable = true
	default:
		return nil, fmt.Errorf("%w for mutability: %#x != 0x00 or 0x01", ErrInvalidByte, mut)
	}
	return ret, nil
}
