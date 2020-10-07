package instr

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/c0mm4nd/wasman/leb128"
	"github.com/c0mm4nd/wasman/types"
)

type Expr struct {
	OpCode OpCode
	Data   []byte
}

func ReadExpr(r io.Reader) (*Expr, error) {
	b := make([]byte, 1)
	_, err := io.ReadFull(r, b)
	if err != nil {
		return nil, fmt.Errorf("read opcodes.OpCode: %w", err)
	}
	buf := new(bytes.Buffer)
	teeR := io.TeeReader(r, buf)

	op := OpCode(b[0])
	switch op {
	case OpCodeI32Const:
		_, _, err = leb128.DecodeInt32(teeR)
	case OpCodeI64Const:
		_, _, err = leb128.DecodeInt64(teeR)
	case OpCodeF32Const:
		_, err = ReadFloat32(teeR)
	case OpCodeF64Const:
		_, err = ReadFloat64(teeR)
	case OpCodeGlobalGet:
		_, _, err = leb128.DecodeUint32(teeR)
	default:
		return nil, fmt.Errorf("%w for opcodes.OpCode: %#x", types.ErrInvalidByte, b[0])
	}

	if err != nil {
		return nil, fmt.Errorf("read value: %w", err)
	}

	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("look for end opcodes.OpCode: %w", err)
	}

	if b[0] != byte(OpCodeEnd) {
		return nil, fmt.Errorf("constant expression has not terminated")
	}

	return &Expr{
		OpCode: op,
		Data:   buf.Bytes(),
	}, nil
}

// IEEE 754
func ReadFloat32(r io.Reader) (float32, error) {
	buf := make([]byte, 4)

	_, err := io.ReadFull(r, buf)
	if err != nil {
		return 0, err
	}

	raw := binary.LittleEndian.Uint32(buf)
	return math.Float32frombits(raw), nil
}

// IEEE 754
func ReadFloat64(r io.Reader) (float64, error) {
	buf := make([]byte, 8)

	_, err := io.ReadFull(r, buf)
	if err != nil {
		return 0, err
	}

	raw := binary.LittleEndian.Uint64(buf)
	return math.Float64frombits(raw), nil
}
