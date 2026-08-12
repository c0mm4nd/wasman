package segments

import (
	"bytes"
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/leb128decode"
	"github.com/c0mm4nd/wasman/types"
)

// LocalDecl is one run-length local declaration in a code segment.
type LocalDecl struct {
	Count uint32
	Type  types.ValueType
}

// CodeSegment is one unit in the wasman.Module's CodeSection
type CodeSegment struct {
	NumLocals uint32
	// LocalDecls preserves the (run-length) local declarations, excluding
	// parameters. Used by validation; kept compressed so an adversarial local
	// count cannot force a huge allocation.
	LocalDecls []LocalDecl
	Body       []byte
}

// ReadCodeSegment reads one CodeSegment from the io.Reader
func ReadCodeSegment(r *bytes.Reader) (*CodeSegment, error) {
	ss, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get the size of code segment: %w", err)
	}
	remaining := int64(ss)

	// parse locals
	ls, bytesRead, err := leb128decode.DecodeUint32(r)
	remaining -= int64(bytesRead)
	if err != nil {
		return nil, fmt.Errorf("get the size locals: %w", err)
	} else if remaining < 0 {
		return nil, io.EOF
	}

	var numLocals uint64
	var n uint32
	var localDecls []LocalDecl
	for i := uint32(0); i < ls; i++ {
		n, bytesRead, err = leb128decode.DecodeUint32(r)
		remaining -= int64(bytesRead) + 1 // +1 for the subsequent ReadByte
		if err != nil {
			return nil, fmt.Errorf("read n of locals: %w", err)
		} else if remaining < 0 {
			return nil, io.EOF
		}
		numLocals += uint64(n)
		if numLocals > 0xFFFFFFFF { // the total number of locals must fit in u32
			return nil, fmt.Errorf("too many locals: %d", numLocals)
		}

		lt, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read type of local")
		}
		switch t := types.ValueType(lt); t {
		case types.ValueTypeI32, types.ValueTypeI64, types.ValueTypeF32, types.ValueTypeF64:
			localDecls = append(localDecls, LocalDecl{Count: n, Type: t})
		default:
			return nil, fmt.Errorf("invalid local type: %#x", lt)
		}
	}

	// extract body
	body := make([]byte, remaining)
	_, err = io.ReadFull(r, body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if body[len(body)-1] != byte(expr.OpCodeEnd) {
		return nil, fmt.Errorf("expr not end with opcodes.OpCodeEnd")
	}

	return &CodeSegment{
		Body:       body[:len(body)-1],
		NumLocals:  uint32(numLocals),
		LocalDecls: localDecls,
	}, nil
}
