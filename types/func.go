package types

import (
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/leb128"
)

type FuncType struct {
	InputTypes  []ValueType
	ReturnTypes []ValueType
}

func ReadFuncType(r io.Reader) (*FuncType, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read leading byte: %w", err)
	}

	if b[0] != 0x60 {
		return nil, fmt.Errorf("%w: %#x != 0x60", ErrInvalidByte, b[0])
	}

	s, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get the size of input value types: %w", err)
	}

	ip, err := ReadValueTypes(r, s)
	if err != nil {
		return nil, fmt.Errorf("read value types of inputs: %w", err)
	}

	s, _, err = leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get the size of output value types: %w", err)
	}

	op, err := ReadValueTypes(r, s)
	if err != nil {
		return nil, fmt.Errorf("read value types of outputs: %w", err)
	}

	return &FuncType{
		InputTypes:  ip,
		ReturnTypes: op,
	}, nil
}
