package wasm

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/c0mm4nd/wasman/config"
	"io"

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
	*config.ModuleConfig

	// sections
	TypesSection     []*types.FuncType
	ImportsSection   []*segments.ImportSegment
	FunctionsSection []uint32
	TablesSection    []*types.TableType
	MemorySection    []*types.MemoryType
	GlobalsSection   []*segments.GlobalSegment
	ExportsSection   map[string]*segments.ExportSegment
	StartSection     []uint32
	ElementsSection  []*segments.ElemSegment
	CodesSection     []*segments.CodeSegment
	DataSection      []*segments.DataSegment

	// index spaces
	IndexSpace *IndexSpace
}

// index to the imports
type IndexSpace struct {
	Functions []fn
	Globals   []*global
	Tables    [][]*uint32
	Memories  [][]byte
}

type global struct {
	Type *types.GlobalType
	Val  interface{}
}

// NewModule reads bytes from the io.Reader and read all sections, finally return a wasman.Module entity if no error
func NewModule(r io.Reader, config *config.ModuleConfig) (*Module, error) {
	// magic number
	buf := make([]byte, 4)
	if n, err := io.ReadFull(r, buf); err != nil || n != 4 || !bytes.Equal(buf, magic) {
		return nil, ErrInvalidMagicNumber
	}

	// version
	if n, err := io.ReadFull(r, buf); err != nil || n != 4 || !bytes.Equal(buf, version) {
		return nil, ErrInvalidVersion
	}

	module := &Module{}
	if err := module.readSections(r); err != nil {
		return nil, fmt.Errorf("readSections failed: %w", err)
	}

	if config != nil {
		module.ModuleConfig = config
	}

	return module, nil
}
