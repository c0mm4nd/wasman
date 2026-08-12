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
// It supports the full bulk-memory / reference-types flags encoding (flags 0-7):
// active/passive/declarative segments, an explicit table index, and element
// lists given either as function indices or as constant expressions
// (ref.func / ref.null). The MVP encoding (a bare table index of 0) is
// identical to flag 0.
func ReadElemSegment(r *bytes.Reader) (*ElemSegment, error) {
	flag, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read element segment flag: %w", err)
	}

	seg := &ElemSegment{}
	var needOffset, needTableIdx, needKind, exprElems bool
	switch flag {
	case 0x00: // active, table 0, funcidx vec
		needOffset = true
	case 0x01: // passive, funcidx vec
		seg.Passive = true
		needKind = true
	case 0x02: // active, explicit tableidx, funcidx vec
		needTableIdx = true
		needOffset = true
		needKind = true
	case 0x03: // declarative, funcidx vec
		seg.Passive = true
		needKind = true
	case 0x04: // active, table 0, expr vec
		needOffset = true
		exprElems = true
	case 0x05: // passive, reftype, expr vec
		seg.Passive = true
		needKind = true
		exprElems = true
	case 0x06: // active, explicit tableidx, reftype, expr vec
		needTableIdx = true
		needOffset = true
		needKind = true
		exprElems = true
	case 0x07: // declarative, reftype, expr vec
		seg.Passive = true
		needKind = true
		exprElems = true
	default:
		return nil, fmt.Errorf("invalid element segment flag: %d", flag)
	}

	if needTableIdx {
		ti, _, err := leb128decode.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read table index: %w", err)
		}
		seg.TableIndex = ti
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

	if needKind {
		// elemkind (0x00 for funcidx lists) or reftype (0x70 funcref for expr lists)
		k, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read elemkind/reftype: %w", err)
		}
		if k != 0x00 && k != 0x70 {
			return nil, fmt.Errorf("unsupported elemkind/reftype: %#x", k)
		}
	}

	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get size of vector: %w", err)
	}

	init := make([]uint32, vs)
	for i := range init {
		if exprElems {
			idx, err := readElemExpr(r)
			if err != nil {
				return nil, fmt.Errorf("read element expression: %w", err)
			}
			init[i] = idx
		} else {
			fIDx, _, err := leb128decode.DecodeUint32(r)
			if err != nil {
				return nil, fmt.Errorf("read function index: %w", err)
			}
			init[i] = fIDx
		}
	}
	seg.Init = init

	return seg, nil
}

// readElemExpr reads a single element expression (ref.func idx end, or
// ref.null t end) and returns the referenced function index (0 for a null
// reference, which only appears in passive/declarative segments here).
func readElemExpr(r *bytes.Reader) (uint32, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}

	var idx uint32
	switch expr.OpCode(b) {
	case expr.OpCodeFunc: // ref.func
		idx, _, err = leb128decode.DecodeUint32(r)
		if err != nil {
			return 0, fmt.Errorf("read ref.func index: %w", err)
		}
	case expr.OpCodeNull: // ref.null <reftype>
		if _, err := r.ReadByte(); err != nil {
			return 0, fmt.Errorf("read ref.null type: %w", err)
		}
	default:
		return 0, fmt.Errorf("unsupported element expression opcode: %#x", b)
	}

	end, err := r.ReadByte()
	if err != nil {
		return 0, fmt.Errorf("read element expression end: %w", err)
	}
	if expr.OpCode(end) != expr.OpCodeEnd {
		return 0, fmt.Errorf("element expression not terminated: %#x", end)
	}
	return idx, nil
}
