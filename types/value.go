package types

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

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

// ReadValueTypes will read num types.ValueType from the io.Reader. The
// buffer is read in bounded chunks so an adversarial num cannot force a
// huge up-front allocation before the (possibly short) reader is consulted.
func ReadValueTypes(r io.Reader, num uint32) ([]ValueType, error) {
	ret := make([]ValueType, 0, min32(num, 1024))
	const chunk = 1024
	buf := make([]byte, chunk)
	for remaining := num; remaining > 0; {
		n := chunk
		if uint32(n) > remaining {
			n = int(remaining)
		}
		if _, err := io.ReadFull(r, buf[:n]); err != nil {
			return nil, err
		}
		for _, v := range buf[:n] {
			switch vt := ValueType(v); vt {
			case ValueTypeI32, ValueTypeF32, ValueTypeI64, ValueTypeF64:
				ret = append(ret, vt)
			default:
				return nil, fmt.Errorf("invalid value type: %d", vt)
			}
		}
		remaining -= uint32(n)
	}
	return ret, nil
}

func min32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// ReadNameValue will read a name string from the io.Reader.
// Per the spec, names are byte vectors that must be valid UTF-8.
func ReadNameValue(r *bytes.Reader) (string, error) {
	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return "", fmt.Errorf("read size of name: %w", err)
	}

	if uint64(vs) > uint64(r.Len()) {
		return "", fmt.Errorf("name length %d exceeds remaining input", vs)
	}
	buf := make([]byte, vs)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", fmt.Errorf("read bytes of name: %w", err)
	}

	if !utf8.Valid(buf) {
		return "", fmt.Errorf("name is not valid UTF-8")
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
