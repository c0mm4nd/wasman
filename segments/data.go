package segments

import (
	"bytes"
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/leb128decode"
)

// DataSegment is one unit of the wasman.Module's DataSection, initializing
// a range of memory, at a given offset, with a static vector of bytes
//
// https://www.w3.org/TR/wasm-core-1/#data-segments%E2%91%A0
type DataSegment struct {
	MemoryIndex      uint32 // usually zero (the MVP has a single memory)
	OffsetExpression *expr.Expression
	Init             []byte
	// Passive marks a bulk-memory passive data segment (no offset; it is only
	// materialized on demand via memory.init and is not applied at instantiation).
	Passive bool
}

// ReadDataSegment reads one DataSegment from the io.Reader.
//
// It supports the bulk-memory flags encoding:
//
//	flag 0: active, memory 0        -> offset expr, bytes
//	flag 1: passive                 -> bytes
//	flag 2: active, explicit memidx -> memidx, offset expr, bytes
//
// The MVP encoding (a bare memory index of 0) is identical to flag 0.
func ReadDataSegment(r *bytes.Reader) (*DataSegment, error) {
	flag, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read data segment flag: %w", err)
	}

	seg := &DataSegment{}
	switch flag {
	case 0x00: // active, memory 0
	case 0x01: // passive
		seg.Passive = true
	case 0x02: // active, explicit memory index
		mi, _, err := leb128decode.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read memory index: %w", err)
		}
		seg.MemoryIndex = mi
	default:
		return nil, fmt.Errorf("invalid data segment flag: %d", flag)
	}

	if !seg.Passive {
		expression, err := expr.ReadExpression(r)
		if err != nil {
			return nil, fmt.Errorf("read offset expression: %w", err)
		}
		// an offset is a constant expression: i32.const or global.get of an
		// (imported, immutable) i32 global.
		if expression.OpCode != expr.OpCodeI32Const && expression.OpCode != expr.OpCodeGlobalGet {
			return nil, fmt.Errorf("offset expression must be i32.const or global.get but got %#x", expression.OpCode)
		}
		seg.OffsetExpression = expression
	}

	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get the size of vector: %w", err)
	}

	b := make([]byte, vs)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read bytes for init: %w", err)
	}
	seg.Init = b

	return seg, nil
}
