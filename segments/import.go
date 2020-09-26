package segments

import (
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/leb128"
	"github.com/c0mm4nd/wasman/types"
)

type ImportDesc struct {
	Kind byte

	TypeIndexPtr  *uint32
	TableTypePtr  *types.TableType
	MemTypePtr    *types.MemoryType
	GlobalTypePtr *types.GlobalType
}

func ReadImportDesc(r io.Reader) (*ImportDesc, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read value kind: %w", err)
	}

	switch b[0] {
	case 0x00:
		tID, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read typeindex: %w", err)
		}
		return &ImportDesc{
			Kind:         0x00,
			TypeIndexPtr: &tID,
		}, nil
	case 0x01:
		tt, err := types.ReadTableType(r)
		if err != nil {
			return nil, fmt.Errorf("read table type: %w", err)
		}
		return &ImportDesc{
			Kind:         0x01,
			TableTypePtr: tt,
		}, nil
	case 0x02:
		mt, err := types.ReadMemoryType(r)
		if err != nil {
			return nil, fmt.Errorf("read table type: %w", err)
		}
		return &ImportDesc{
			Kind:       0x02,
			MemTypePtr: mt,
		}, nil
	case 0x03:
		gt, err := types.ReadGlobalType(r)
		if err != nil {
			return nil, fmt.Errorf("read global type: %w", err)
		}

		return &ImportDesc{
			Kind:          0x03,
			GlobalTypePtr: gt,
		}, nil
	default:
		return nil, fmt.Errorf("%w: invalid byte for importdesc: %#x", types.ErrInvalidByte, b[0])
	}
}

type ImportSegment struct {
	Module string
	Name   string
	Desc   *ImportDesc
}

func ReadImportSegment(r io.Reader) (*ImportSegment, error) {
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
