package wasm

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/c0mm4nd/wasman/leb128decode"
	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
)

type sectionID byte

const (
	sectionIDCustom   sectionID = 0
	sectionIDType     sectionID = 1
	sectionIDImport   sectionID = 2
	sectionIDFunction sectionID = 3
	sectionIDTable    sectionID = 4
	sectionIDMemory   sectionID = 5
	sectionIDGlobal   sectionID = 6
	sectionIDExport   sectionID = 7
	sectionIDStart     sectionID = 8
	sectionIDElement   sectionID = 9
	sectionIDCode      sectionID = 10
	sectionIDData      sectionID = 11
	sectionIDDataCount sectionID = 12
)

// sectionOrder gives the mandatory ordering rank of the non-custom sections;
// they must appear at most once, in strictly increasing rank. Note the
// DataCount section is ordered between Element and Code.
var sectionOrder = map[sectionID]int{
	sectionIDType:      1,
	sectionIDImport:    2,
	sectionIDFunction:  3,
	sectionIDTable:     4,
	sectionIDMemory:    5,
	sectionIDGlobal:    6,
	sectionIDExport:    7,
	sectionIDStart:     8,
	sectionIDElement:   9,
	sectionIDDataCount: 10,
	sectionIDCode:      11,
	sectionIDData:      12,
}

func (m *Module) readSections(r *bytes.Reader) error {
	prevRank := 0
	for r.Len() > 0 {
		rank, err := m.readSection(r, prevRank)
		if err != nil {
			return err
		}
		if rank > 0 {
			prevRank = rank
		}
	}
	return nil
}

func (m *Module) readSection(r *bytes.Reader, prevRank int) (int, error) {
	id, err := r.ReadByte()
	if err != nil {
		return 0, fmt.Errorf("read section id: %w", err)
	}

	ss, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return 0, fmt.Errorf("get size of section for id=%d: %w", sectionID(id), err)
	}
	if uint64(ss) > uint64(r.Len()) {
		return 0, fmt.Errorf("section for %d: size %d out of bounds (unexpected end)", sectionID(id), ss)
	}

	rank := 0
	if sectionID(id) != sectionIDCustom {
		var ok bool
		rank, ok = sectionOrder[sectionID(id)]
		if !ok {
			return 0, errors.New("invalid section id")
		}
		if rank <= prevRank {
			return 0, fmt.Errorf("section for %d: out of order or duplicated", sectionID(id))
		}
	}

	before := r.Len()
	switch sectionID(id) {
	case sectionIDCustom:
		err = m.readSectionCustom(r, ss)
	case sectionIDType:
		err = m.readSectionTypes(r)
	case sectionIDImport:
		err = m.readSectionImports(r)
	case sectionIDFunction:
		err = m.readSectionFunctions(r)
	case sectionIDTable:
		err = m.readSectionTables(r)
	case sectionIDMemory:
		err = m.readSectionMemories(r)
	case sectionIDGlobal:
		err = m.readSectionGlobals(r)
	case sectionIDExport:
		err = m.readSectionExports(r)
	case sectionIDStart:
		err = m.readSectionStart(r)
	case sectionIDElement:
		err = m.readSectionElement(r)
	case sectionIDCode:
		err = m.readSectionCodes(r)
	case sectionIDData:
		err = m.readSectionData(r)
	case sectionIDDataCount:
		// The bulk-memory data count section: the number of data segments,
		// cross-checked against the data section after all sections are read.
		var dc uint32
		dc, _, err = leb128decode.DecodeUint32(r)
		if err == nil {
			m.DataCountSection = &dc
		}
	}

	if err != nil {
		return 0, fmt.Errorf("read section for %d: %w", sectionID(id), err)
	}

	// the declared section size must exactly frame its content
	if consumed := before - r.Len(); consumed != int(ss) {
		return 0, fmt.Errorf("section for %d: size mismatch (declared %d, consumed %d)", sectionID(id), ss, consumed)
	}

	return rank, nil
}

// readSectionCustom consumes a custom section, validating that its name field
// is well-formed (in-bounds length, valid UTF-8) as the spec requires. The
// payload itself is opaque and ignored.
// https://www.w3.org/TR/wasm-core-1/#custom-section
func (m *Module) readSectionCustom(r *bytes.Reader, size uint32) error {
	body := make([]byte, size)
	if _, err := io.ReadFull(r, body); err != nil {
		return fmt.Errorf("read custom section body: %w", err)
	}

	br := bytes.NewReader(body)
	nameLen, l, err := leb128decode.DecodeUint32(br)
	if err != nil {
		return fmt.Errorf("read custom section name length: %w", err)
	}
	if uint64(nameLen)+l > uint64(size) {
		return fmt.Errorf("custom section name length out of bounds")
	}
	name := body[l : uint64(l)+uint64(nameLen)]
	if !utf8.Valid(name) {
		return fmt.Errorf("custom section name is not valid UTF-8")
	}

	return nil
}

func (m *Module) readSectionTypes(r *bytes.Reader) error {
	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}

	m.TypeSection = make([]*types.FuncType, vs)
	for i := range m.TypeSection {
		m.TypeSection[i], err = types.ReadFuncType(r)
		if err != nil {
			return fmt.Errorf("read %d-th function type: %w", i, err)
		}
	}

	return nil
}

func (m *Module) readSectionImports(r *bytes.Reader) error {
	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}

	m.ImportSection = make([]*segments.ImportSegment, vs)
	for i := range m.ImportSection {
		m.ImportSection[i], err = segments.ReadImportSegment(r)
		if err != nil {
			return fmt.Errorf("read import: %w", err)
		}
	}

	return nil
}

func (m *Module) readSectionFunctions(r *bytes.Reader) error {
	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}

	m.FunctionSection = make([]uint32, vs)
	for i := range m.FunctionSection {
		m.FunctionSection[i], _, err = leb128decode.DecodeUint32(r)
		if err != nil {
			return fmt.Errorf("get typeidx: %w", err)
		}
	}

	return nil
}

func (m *Module) readSectionTables(r *bytes.Reader) error {
	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}

	m.TableSection = make([]*types.TableType, vs)
	for i := range m.TableSection {
		m.TableSection[i], err = types.ReadTableType(r)
		if err != nil {
			return fmt.Errorf("read table type: %w", err)
		}
	}

	return nil
}

func (m *Module) readSectionMemories(r *bytes.Reader) error {
	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}

	m.MemorySection = make([]*types.MemoryType, vs)
	for i := range m.MemorySection {
		m.MemorySection[i], err = types.ReadMemoryType(r)
		if err != nil {
			return fmt.Errorf("read memory type: %w", err)
		}
	}

	return nil
}

func (m *Module) readSectionGlobals(r *bytes.Reader) error {
	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}

	m.GlobalSection = make([]*segments.GlobalSegment, vs)
	for i := range m.GlobalSection {
		m.GlobalSection[i], err = segments.ReadGlobalSegment(r)
		if err != nil {
			return fmt.Errorf("read global segment: %w ", err)
		}
	}

	return nil
}

func (m *Module) readSectionExports(r *bytes.Reader) error {
	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}

	m.ExportSection = make(map[string]*segments.ExportSegment, vs)
	for i := uint32(0); i < vs; i++ {
		expDesc, err := segments.ReadExportSegment(r)
		if err != nil {
			return fmt.Errorf("read export: %w", err)
		}

		// export names must be unique; the map would silently drop duplicates
		if _, ok := m.ExportSection[expDesc.Name]; ok {
			return fmt.Errorf("duplicate export name: %q", expDesc.Name)
		}
		m.ExportSection[expDesc.Name] = expDesc
	}

	return nil
}

func (m *Module) readSectionStart(r *bytes.Reader) error {
	// The start section is a single function index, not a vector.
	// https://www.w3.org/TR/wasm-core-1/#start-section
	idx, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read start function index: %w", err)
	}

	m.StartSection = []uint32{idx}

	return nil
}

func (m *Module) readSectionElement(r *bytes.Reader) error {
	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}

	m.ElementsSection = make([]*segments.ElemSegment, vs)
	for i := range m.ElementsSection {
		m.ElementsSection[i], err = segments.ReadElemSegment(r)
		if err != nil {
			return fmt.Errorf("read element: %w", err)
		}
	}

	return nil
}

func (m *Module) readSectionCodes(r *bytes.Reader) error {
	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}

	m.CodeSection = make([]*segments.CodeSegment, vs)	
	for i := range m.CodeSection {
		m.CodeSection[i], err = segments.ReadCodeSegment(r)
		if err != nil {
			return fmt.Errorf("read code segment: %w", err)
		}
	}

	return nil
}

func (m *Module) readSectionData(r *bytes.Reader) error {
	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}

	m.DataSection = make([]*segments.DataSegment, vs)
	for i := range m.DataSection {
		m.DataSection[i], err = segments.ReadDataSegment(r)
		if err != nil {
			return fmt.Errorf("read data segment: %w", err)
		}
	}

	return nil
}
