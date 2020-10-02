package segments

import (
	"fmt"
	"io"
	"io/ioutil"

	"github.com/c0mm4nd/wasman/instr"
	"github.com/c0mm4nd/wasman/leb128"
)

type CodeSegment struct {
	NumLocals uint32
	Body      []byte
}

func ReadCodeSegment(r io.Reader) (*CodeSegment, error) {
	ss, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get the size of code segment: %w", err)
	}

	r = io.LimitReader(r, int64(ss))

	// parse locals
	ls, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get the size locals: %w", err)
	}

	var numLocals uint32
	b := make([]byte, 1)
	for i := uint32(0); i < ls; i++ {
		n, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read n of locals: %w", err)
		}
		numLocals += n

		if _, err := io.ReadFull(r, b); err != nil {
			return nil, fmt.Errorf("read type of local")
		}
	}

	// extract body
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if body[len(body)-1] != byte(instr.OpCodeEnd) {
		return nil, fmt.Errorf("expr not end with opcodes.OpCodeEnd")
	}

	return &CodeSegment{
		Body:      body[:len(body)-1],
		NumLocals: numLocals,
	}, nil
}
