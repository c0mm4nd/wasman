package types

import (
	"bytes"
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/leb128decode"
)

// Limits classify the size range of resizeable storage associated with memory types and table types
// https://www.w3.org/TR/wasm-core-1/#limits%E2%91%A0
type Limits struct {
	Min uint32
	Max *uint32 // can be nil
}

// ReadLimits will read a types.Limits from the io.Reader
func ReadLimits(r *bytes.Reader) (*Limits, error) {
	b := make([]byte, 1)
	_, err := io.ReadFull(r, b)
	if err != nil {
		return nil, fmt.Errorf("read leading byte: %w", err)
	}

	ret := &Limits{}
	switch b[0] {
	case 0x00:
		ret.Min, _, err = leb128decode.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read min of limit: %w", err)
		}
	case 0x01:
		ret.Min, _, err = leb128decode.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read min of limit: %w", err)
		}
		m, _, err := leb128decode.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read min of limit: %w", err)
		}
		ret.Max = &m
	default:
		return nil, fmt.Errorf("%w for limits: %#x != 0x00 or 0x01", ErrInvalidTypeByte, b[0])
	}
	return ret, nil
}
