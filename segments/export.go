package segments

import (
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/leb128"
	"github.com/c0mm4nd/wasman/types"
)

const (
	ExportKindFunction byte = 0x00
	ExportKindTable    byte = 0x01
	ExportKindMem      byte = 0x02
	ExportKindGlobal   byte = 0x03
)

type ExportDesc struct {
	Kind  byte
	Index uint32
}

func ReadExportDesc(r io.Reader) (*ExportDesc, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read value kind: %w", err)
	}

	kind := b[0]
	if kind >= 0x04 {
		return nil, fmt.Errorf("%w: invalid byte for exportdesc: %#x", types.ErrInvalidByte, kind)
	}

	id, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read funcidx: %w", err)
	}

	return &ExportDesc{
		Kind:  kind,
		Index: id,
	}, nil

}

type ExportSegment struct {
	Name string
	Desc *ExportDesc
}

func ReadExportSegment(r io.Reader) (*ExportSegment, error) {
	name, err := types.ReadNameValue(r)
	if err != nil {
		return nil, fmt.Errorf("read name of export module: %w", err)
	}

	d, err := ReadExportDesc(r)
	if err != nil {
		return nil, fmt.Errorf("read export description: %w", err)
	}

	return &ExportSegment{Name: name, Desc: d}, nil
}
