package types

import (
	"errors"
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/leb128"
)

var ErrInvalidByte = errors.New("invalid byte")

type ValueType byte

const (
	ValueTypeI32 ValueType = 0x7f
	ValueTypeI64 ValueType = 0x7e
	ValueTypeF32 ValueType = 0x7d
	ValueTypeF64 ValueType = 0x7c
)

func (v ValueType) String() string {
	switch v {
	case ValueTypeI32:
		return "i32"
	case ValueTypeI64:
		return "i64"
	case ValueTypeF32:
		return "f32"
	case ValueTypeF64:
		return "f64"
	default:
		return "unknown value type"
	}
}

func ReadValueTypes(r io.Reader, num uint32) ([]ValueType, error) {
	ret := make([]ValueType, num)
	buf := make([]byte, num)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}

	for i, v := range buf {
		switch vt := ValueType(v); vt {
		case ValueTypeI32, ValueTypeF32, ValueTypeI64, ValueTypeF64:
			ret[i] = vt
		default:
			return nil, fmt.Errorf("invalid value type: %d", vt)
		}
	}
	return ret, nil
}

func ReadNameValue(r io.Reader) (string, error) {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return "", fmt.Errorf("read size of name: %w", err)
	}

	buf := make([]byte, vs)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", fmt.Errorf("read bytes of name: %w", err)
	}

	return string(buf), nil
}

func HasSameSignature(a []ValueType, b []ValueType) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
