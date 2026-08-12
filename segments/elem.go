package segments

import (
	"bytes"
	"fmt"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/leb128decode"
)

// ElemSegment is one unit of the wasm.Module's ElementsSection, initializing
// a subrange of a table, at a given offset, from a static vector of elements.
//
// https://www.w3.org/TR/wasm-core-1/#element-segments%E2%91%A0
type ElemSegment struct {
	TableIndex uint32
	OffsetExpr *expr.Expression
	Init       []uint32
	// Passive marks a passive or declarative element segment: it is not applied
	// to a table at instantiation (it would be used later via table.init).
	Passive bool
}

// ReadElemSegment reads one ElemSegment from the io.Reader.
//
// It supports the bulk-memory / reference-types flags encoding for the funcref
// element kind:
//
//	flag 0: active, table 0            -> offset expr, funcidx vec
//	flag 1: passive                    -> elemkind, funcidx vec
//	flag 2: active, explicit tableidx  -> tableidx, offset expr, elemkind, funcidx vec
//	flag 3: declarative                -> elemkind, funcidx vec
//
// The MVP encoding (a bare table index of 0) is identical to flag 0.
func ReadElemSegment(r *bytes.Reader) (*ElemSegment, error) {
	flag, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read element segment flag: %w", err)
	}

	seg := &ElemSegment{}
	var needOffset, needElemKind bool
	switch flag {
	case 0x00: // active, table 0
		needOffset = true
	case 0x01: // passive
		seg.Passive = true
		needElemKind = true
	case 0x02: // active, explicit table index
		ti, _, err := leb128decode.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read table index: %w", err)
		}
		seg.TableIndex = ti
		needOffset = true
		needElemKind = true
	case 0x03: // declarative
		seg.Passive = true
		needElemKind = true
	default:
		// flags 4-7 use reftype expression element lists (reference types)
		return nil, fmt.Errorf("unsupported element segment flag: %d", flag)
	}

	if needOffset {
		expression, err := expr.ReadExpression(r)
		if err != nil {
			return nil, fmt.Errorf("read expr for offset: %w", err)
		}
		// an offset is a constant expression: i32.const or global.get of an
		// (imported, immutable) i32 global.
		if expression.OpCode != expr.OpCodeI32Const && expression.OpCode != expr.OpCodeGlobalGet {
			return nil, fmt.Errorf("offset expression must be i32.const or global.get but got %#x", expression.OpCode)
		}
		seg.OffsetExpr = expression
	}

	if needElemKind {
		ek, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read elemkind: %w", err)
		}
		if ek != 0x00 { // 0x00 == funcref
			return nil, fmt.Errorf("unsupported elemkind: %#x", ek)
		}
	}

	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get size of vector: %w", err)
	}

	init := make([]uint32, vs)
	for i := range init {
		fIDx, _, err := leb128decode.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read function index: %w", err)
		}
		init[i] = fIDx
	}
	seg.Init = init

	return seg, nil
}
