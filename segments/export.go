package segments

import (
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/leb128decode"
	"github.com/c0mm4nd/wasman/types"
)

// ExportDesc means export descriptions, which describe an export in one wasman.Module
type ExportDesc struct {
	Kind  byte
	Index uint32
}

// ReadExportDesc reads one ExportDesc from the io.Reader
func ReadExportDesc(r io.Reader) (*ExportDesc, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read value kind: %w", err)
	}

	kind := b[0]
	if kind >= 0x04 {
		return nil, fmt.Errorf("%w: invalid byte for exportdesc: %#x", types.ErrInvalidTypeByte, kind)
	}

	id, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read funcidx: %w", err)
	}

	return &ExportDesc{
		Kind:  kind,
		Index: id,
	}, nil
}

// ExportSegment is one unit of the wasm.Module's ExportSection
type ExportSegment struct {
	Name string
	Desc *ExportDesc
}

// ReadExportSegment reads one ExportSegment from the io.Reader
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
