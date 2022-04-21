package segments

import (
	"bytes"
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/leb128decode"
	"github.com/c0mm4nd/wasman/types"
)

// ImportDesc means import descriptions, which describe an import in one wasman.Module
type ImportDesc struct {
	Kind byte

	TypeIndexPtr  *uint32           // => func x
	TableTypePtr  *types.TableType  // => table tt
	MemTypePtr    *types.MemoryType // => mem mt
	GlobalTypePtr *types.GlobalType // => global gt
}

// ReadImportDesc reads one ImportDesc from the io.Reader
func ReadImportDesc(r *bytes.Reader) (*ImportDesc, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read value kind: %w", err)
	}

	switch b[0] {
	case KindFunction:
		tID, _, err := leb128decode.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read typeindex: %w", err)
		}
		return &ImportDesc{
			Kind:         0x00,
			TypeIndexPtr: &tID,
		}, nil
	case KindTable:
		tt, err := types.ReadTableType(r)
		if err != nil {
			return nil, fmt.Errorf("read table type: %w", err)
		}
		return &ImportDesc{
			Kind:         0x01,
			TableTypePtr: tt,
		}, nil
	case KindMem:
		mt, err := types.ReadMemoryType(r)
		if err != nil {
			return nil, fmt.Errorf("read table type: %w", err)
		}
		return &ImportDesc{
			Kind:       0x02,
			MemTypePtr: mt,
		}, nil
	case KindGlobal:
		gt, err := types.ReadGlobalType(r)
		if err != nil {
			return nil, fmt.Errorf("read global type: %w", err)
		}

		return &ImportDesc{
			Kind:          0x03,
			GlobalTypePtr: gt,
		}, nil
	default:
		return nil, fmt.Errorf("%w: invalid byte for importdesc: %#x", types.ErrInvalidTypeByte, b[0])
	}
}

// ImportSegment is one unit of the wasm.Module's ImportSection
type ImportSegment struct {
	Module string
	Name   string
	Desc   *ImportDesc
}

// ReadImportSegment reads one ImportSegment from the io.Reader
func ReadImportSegment(r *bytes.Reader) (*ImportSegment, error) {
	mn, err := types.ReadNameValue(r)
	if err != nil {
		return nil, fmt.Errorf("read name of imported module: %w", err)
	}

	n, err := types.ReadNameValue(r)
	if err != nil {
		return nil, fmt.Errorf("read name of imported module component: %w", err)
	}

	d, err := ReadImportDesc(r)
	if err != nil {
		return nil, fmt.Errorf("read import description : %w", err)
	}

	return &ImportSegment{Module: mn, Name: n, Desc: d}, nil
}
