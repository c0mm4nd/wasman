package segments

import (
	"bytes"
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/leb128decode"
)

// CodeSegment is one unit in the wasman.Module's CodeSection
type CodeSegment struct {
	NumLocals uint32
	Body      []byte
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

	var numLocals uint32
	var n uint32
	for i := uint32(0); i < ls; i++ {
		n, bytesRead, err = leb128decode.DecodeUint32(r)
		remaining -= int64(bytesRead) + 1 // +1 for the subsequent ReadByte
		if err != nil {
			return nil, fmt.Errorf("read n of locals: %w", err)
		} else if remaining < 0 {
			return nil, io.EOF
		}
		numLocals += n

		if _, err := r.ReadByte(); err != nil {
			return nil, fmt.Errorf("read type of local") // TODO: save read localType
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
		Body:      body[:len(body)-1],
		NumLocals: numLocals,
	}, nil
}
