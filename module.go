package wasman

import (
	"bytes"
	"fmt"
	"io"

	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
)

// https://www.w3.org/TR/wasm-core-1/#syntax-module
type Module struct {
	*ModuleConfig

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
	indexSpace *indexSpace
}

// index to the imports
type indexSpace struct {
	Functions []fn
	Globals   []*global
	Tables    [][]*uint32
	Memories  [][]byte
}

type global struct {
	Type *types.GlobalType
	Val  interface{}
}

func NewModule(r io.Reader, config *ModuleConfig) (*Module, error) {
	// magic number
	buf := make([]byte, 4)
	if n, err := io.ReadFull(r, buf); err != nil || n != 4 {
		return nil, ErrInvalidMagicNumber
	}
	for !bytes.Equal(buf, magic) {
		return nil, ErrInvalidMagicNumber
	}

	// version
	if n, err := io.ReadFull(r, buf); err != nil || n != 4 {
		panic(err)
	}
	for !bytes.Equal(buf, version) {
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
