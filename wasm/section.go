package wasm

import (
	"bytes"
	"errors"
	"fmt"
	"io"

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
	sectionIDStart    sectionID = 8
	sectionIDElement  sectionID = 9
	sectionIDCode     sectionID = 10
	sectionIDData     sectionID = 11
)

func (m *Module) readSections(r *bytes.Reader) error {
	for {
		if err := m.readSection(r); errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}
	}
}

func (m *Module) readSection(r *bytes.Reader) error {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return fmt.Errorf("read section id: %w", err)
	}

	ss, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of section for id=%d: %w", sectionID(b[0]), err)
	}

	switch sectionID(b[0]) {
	case sectionIDCustom:
		// Custom section is ignored here: https://www.w3.org/TR/wasm-core-1/#custom-section
		bb := make([]byte, ss)
		_, err = io.ReadFull(r, bb)
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
	default:
		err = errors.New("invalid section id")
	}

	if err != nil {
		return fmt.Errorf("read section for %d: %w", sectionID(b[0]), err)
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

		m.ExportSection[expDesc.Name] = expDesc
	}

	return nil
}

func (m *Module) readSectionStart(r *bytes.Reader) error {
	vs, _, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}

	m.StartSection = make([]uint32, vs)
	for i := range m.StartSection {
		m.StartSection[i], _, err = leb128decode.DecodeUint32(r)
		if err != nil {
			return fmt.Errorf("read function index: %w", err)
		}
	}

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
