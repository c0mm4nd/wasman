package wasm

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"

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
	// DataCountSection holds the value of the bulk-memory data count section,
	// if present; it must then match the number of data segments.
	DataCountSection *uint32

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

	// cross-section consistency checks
	if len(module.FunctionSection) != len(module.CodeSection) {
		return nil, errors.New("function and code section have inconsistent lengths")
	}
	if module.DataCountSection != nil && int(*module.DataCountSection) != len(module.DataSection) {
		return nil, errors.New("data count and data section have inconsistent lengths")
	}

	// full static validation (type checking of all function bodies etc.)
	if !config.SkipValidation {
		if err := module.Validate(); err != nil {
			return nil, err
		}
	}

	return module, nil
}

// ExportInfo describes one export of a module.
type ExportInfo struct {
	Name string
	Kind segments.Kind
	// Type is the function signature for KindFunction exports, nil otherwise.
	Type *types.FuncType
}

// Exports lists the module's exports (sorted by name), resolving function
// signatures statically — usable right after NewModule, before instantiation.
func (m *Module) Exports() []ExportInfo {
	// the function index space: imported functions first, then local ones
	var funcTypes []*types.FuncType
	for _, imp := range m.ImportSection {
		if imp.Desc.Kind == segments.KindFunction && imp.Desc.TypeIndexPtr != nil &&
			int(*imp.Desc.TypeIndexPtr) < len(m.TypeSection) {
			funcTypes = append(funcTypes, m.TypeSection[*imp.Desc.TypeIndexPtr])
		}
	}
	for _, ti := range m.FunctionSection {
		if int(ti) < len(m.TypeSection) {
			funcTypes = append(funcTypes, m.TypeSection[ti])
		}
	}

	out := make([]ExportInfo, 0, len(m.ExportSection))
	for name, exp := range m.ExportSection {
		info := ExportInfo{Name: name, Kind: exp.Desc.Kind}
		if exp.Desc.Kind == segments.KindFunction && int(exp.Desc.Index) < len(funcTypes) {
			info.Type = funcTypes[exp.Desc.Index]
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
