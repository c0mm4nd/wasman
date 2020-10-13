package types

import (
	"errors"
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/leb128decode"
)

// ErrInvalidTypeByte means the type byte mismatches the one from wasm binary
var ErrInvalidTypeByte = errors.New("invalid byte")

// ValueType classifies the individual values that WebAssembly code can compute with and the values that a variable accepts
// https://www.w3.org/TR/wasm-core-1/#value-types%E2%91%A0
type ValueType byte

const (
	// ValueTypeI32 classify 32 bit integers
	ValueTypeI32 ValueType = 0x7f
	// ValueTypeI64 classify 64 bit integers
	// Integers are not inherently signed or unsigned, the interpretation is determined by individual operations
	ValueTypeI64 ValueType = 0x7e
	// ValueTypeF32 classify 32 bit floating-point data, known as single
	ValueTypeF32 ValueType = 0x7d
	// ValueTypeF64 classify 64 bit floating-point data, known as double
	ValueTypeF64 ValueType = 0x7c
)

// String will convert the types.ValueType into a string
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

// ReadValueTypes will read a types.ValueType from the io.Reader
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

// ReadNameValue will read a name string from the io.Reader
func ReadNameValue(r io.Reader) (string, error) {
	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return "", fmt.Errorf("read size of name: %w", err)
	}

	buf := make([]byte, vs)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", fmt.Errorf("read bytes of name: %w", err)
	}

	return string(buf), nil
}

// HasSameSignature will verify whether the two types.ValueType are same
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
