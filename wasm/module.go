package wasm

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/config"

	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
)

var (
	magic   = []byte{0x00, 0x61, 0x73, 0x6D} // aka header
	version = []byte{0x01, 0x00, 0x00, 0x00} // version 1, https://www.w3.org/TR/wasm-core-1/
)

// errors on parsing module
var (
	ErrInvalidMagicNumber = errors.New("invalid magic number")
	ErrInvalidVersion     = errors.New("invalid version header")
)

// Module is a standard wasm module implement according to wasm v1, https://www.w3.org/TR/wasm-core-1/#syntax-module%E2%91%A0
type Module struct {
	config.ModuleConfig

	// sections
	TypeSection     []*types.FuncType
	ImportSection   []*segments.ImportSegment
	FunctionSection []uint32
	TableSection    []*types.TableType
	MemorySection   []*types.MemoryType
	GlobalSection   []*segments.GlobalSegment
	ExportSection   map[string]*segments.ExportSegment
	StartSection    []uint32
	ElementsSection []*segments.ElemSegment
	CodeSection     []*segments.CodeSegment
	DataSection     []*segments.DataSegment

	// index spaces
	IndexSpace *IndexSpace
}

// IndexSpace is the indeices to the imports
type IndexSpace struct {
	Functions []fn
	Globals   []*Global
	Tables    []*Table
	Memories  []*Memory
}

// NewModule reads bytes from the io.Reader and read all sections, finally return a wasman.Module entity if no error
func NewModule(config config.ModuleConfig, r *bytes.Reader) (*Module, error) {
	// magic number
	buf := make([]byte, 4)
	if n, err := io.ReadFull(r, buf); err != nil || n != 4 || !bytes.Equal(buf, magic) {
		return nil, ErrInvalidMagicNumber
	}

	// version
	if n, err := io.ReadFull(r, buf); err != nil || n != 4 || !bytes.Equal(buf, version) {
		return nil, ErrInvalidVersion
	}

	module := &Module{
		ModuleConfig: config,
	}

	if err := module.readSections(r); err != nil {
		return nil, fmt.Errorf("readSections failed: %w", err)
	}

	return module, nil
}
