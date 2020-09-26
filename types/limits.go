package types

import (
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/leb128"
)

// https://www.w3.org/TR/wasm-core-1/#syntax-limits
type Limits struct {
	Min uint32
	Max *uint32 // can be nil
}

func ReadLimits(r io.Reader) (*Limits, error) {
	b := make([]byte, 1)
	_, err := io.ReadFull(r, b)
	if err != nil {
		return nil, fmt.Errorf("read leading byte: %w", err)
	}

	ret := &Limits{}
	switch b[0] {
	case 0x00:
		ret.Min, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read min of limit: %w", err)
		}
	case 0x01:
		ret.Min, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read min of limit: %w", err)
		}
		m, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read min of limit: %w", err)
		}
		ret.Max = &m
	default:
		return nil, fmt.Errorf("%w for limits: %#x != 0x00 or 0x01", ErrInvalidByte, b[0])
	}
	return ret, nil
}
