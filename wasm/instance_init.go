package wasm

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/leb128decode"
	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
)

// buildIndexSpaces build index spaces of the module with the given external modules
func (ins *Instance) buildIndexSpaces(externModules map[string]*Module) error {
	ins.IndexSpace = &IndexSpace{}

	// resolve imports
	if err := ins.resolveImports(externModules); err != nil {
		return fmt.Errorf("resolve imports: %w", err)
	}

	// append the locally-defined tables after any imported ones. base is
	// captured before the loop because appending mutates the slice length.
	if base := len(ins.IndexSpace.Tables); len(ins.TableSection) > base {
		for i := base; i < len(ins.TableSection); i++ {
			ins.IndexSpace.Tables = append(ins.IndexSpace.Tables, &Table{
				TableType: *ins.TableSection[i],
				Value:     []fn{},
			})
		}
	}

	// append the locally-defined memories after any imported ones.
	if base := len(ins.IndexSpace.Memories); len(ins.MemorySection) > base {
		for i := base; i < len(ins.MemorySection); i++ {
			ins.IndexSpace.Memories = append(ins.IndexSpace.Memories, &Memory{
				MemoryType: *ins.MemorySection[i],
				Value:      []byte{},
			})
		}
	}

	if err := ins.buildGlobalIndexSpace(); err != nil {
		return fmt.Errorf("build global index space: %w", err)
	}
	if err := ins.buildFunctionIndexSpace(); err != nil {
		return fmt.Errorf("build function index space: %w", err)
	}

	// segment application is all-or-nothing: bounds-check every active element
	// AND data segment first, so a failing instantiation (link error) leaves
	// shared (imported) tables and memories untouched.
	ins.sizeTablesAndMemories()
	elemPlans, err := ins.planElemSegments()
	if err != nil {
		return fmt.Errorf("build table index space: %w", err)
	}
	dataPlans, err := ins.planDataSegments()
	if err != nil {
		return fmt.Errorf("build memory index space: %w", err)
	}
	ins.applyElemSegments(elemPlans)
	ins.applyDataSegments(dataPlans)

	// publish this instantiation's spaces on the Module so other modules can
	// import from it (the Module hands out its most recent instantiation);
	// this instance keeps its OWN pointer, so a later re-instantiation of the
	// same Module cannot corrupt it.
	ins.Module.IndexSpace = ins.IndexSpace

	return nil
}

func (ins *Instance) resolveImports(externModules map[string]*Module) error {
	for _, is := range ins.ImportSection {
		em, ok := externModules[is.Module]
		if !ok {
			return fmt.Errorf("failed to resolve import of module name %s", is.Module)
		}

		// an extern module must have been instantiated (or host-defined) before
		// anything can import from it; failing that is a link error, not a panic
		if em.IndexSpace == nil {
			return fmt.Errorf("module %q has not been instantiated", is.Module)
		}

		es, ok := em.ExportSection[is.Name]
		if !ok {
			return fmt.Errorf("%s not exported in module %s", is.Name, is.Module)
		}

		if is.Desc.Kind != es.Desc.Kind {
			return fmt.Errorf("type mismatch on export: got %#x but want %#x", es.Desc.Kind, is.Desc.Kind)
		}
		switch is.Desc.Kind {
		case 0x00: // function
			if err := ins.applyFunctionImport(is, em, es); err != nil {
				return fmt.Errorf("applyFunctionImport failed: %w", err)
			}
		case 0x01: // table
			if err := ins.applyTableImport(is, em, es); err != nil {
				return fmt.Errorf("applyTableImport failed: %w", err)
			}
		case 0x02: // memory
			if err := ins.applyMemoryImport(is, em, es); err != nil {
				return fmt.Errorf("applyMemoryImport: %w", err)
			}
		case 0x03: // global
			if err := ins.applyGlobalImport(is, em, es); err != nil {
				return fmt.Errorf("applyGlobalImport: %w", err)
			}
		default:
			return fmt.Errorf("invalid kind of import: %#x", is.Desc.Kind)
		}
	}
	return nil
}

func (ins *Instance) applyFunctionImport(importSeg *segments.ImportSegment, externModule *Module, exportSeg *segments.ExportSegment) error {
	if exportSeg.Desc.Index >= uint32(len(externModule.IndexSpace.Functions)) {
		return fmt.Errorf("exported index out of range")
	}

	if importSeg.Desc.TypeIndexPtr == nil {
		return fmt.Errorf("is.Desc.TypeIndexPtr is nill")
	}

	iSig := ins.TypeSection[*importSeg.Desc.TypeIndexPtr]
	f := externModule.IndexSpace.Functions[exportSeg.Desc.Index]
	if !types.HasSameSignature(iSig.ReturnTypes, f.getType().ReturnTypes) {
		return fmt.Errorf("return signature mimatch: %#v != %#v", iSig.ReturnTypes, f.getType().ReturnTypes)
	} else if !types.HasSameSignature(iSig.InputTypes, f.getType().InputTypes) {
		return fmt.Errorf("input signature mimatch: %#v != %#v", iSig.InputTypes, f.getType().InputTypes)
	}
	ins.IndexSpace.Functions = append(ins.IndexSpace.Functions, f)
	return nil
}

// limitsCompatible reports whether the provided (exporter's) limits satisfy
// the required (importer's declared) limits:
// provided.Min >= required.Min and provided.Max <= required.Max (if required).
func limitsCompatible(provided, required *types.Limits) bool {
	var pMin, rMin uint32
	var pMax, rMax *uint32
	if provided != nil {
		pMin, pMax = provided.Min, provided.Max
	}
	if required != nil {
		rMin, rMax = required.Min, required.Max
	}
	if pMin < rMin {
		return false
	}
	if rMax != nil && (pMax == nil || *pMax > *rMax) {
		return false
	}
	return true
}

func (ins *Instance) applyTableImport(importSeg *segments.ImportSegment, externModule *Module, exportSeg *segments.ExportSegment) error {
	if exportSeg.Desc.Index >= uint32(len(externModule.IndexSpace.Tables)) {
		return fmt.Errorf("exported index out of range")
	}

	table := externModule.IndexSpace.Tables[exportSeg.Desc.Index]
	if want := importSeg.Desc.TableTypePtr; want != nil {
		if !limitsCompatible(table.Limits, want.Limits) {
			return fmt.Errorf("incompatible import type: table limits mismatch")
		}
	}

	ins.IndexSpace.Tables = append(ins.IndexSpace.Tables, table)
	return nil
}

func (ins *Instance) applyMemoryImport(importSeg *segments.ImportSegment, externModule *Module, exportSegment *segments.ExportSegment) error {
	if exportSegment.Desc.Index >= uint32(len(externModule.IndexSpace.Memories)) {
		return fmt.Errorf("exported index out of range")
	}

	memory := externModule.IndexSpace.Memories[exportSegment.Desc.Index]
	if want := importSeg.Desc.MemTypePtr; want != nil {
		if !limitsCompatible(&memory.MemoryType, want) {
			return fmt.Errorf("incompatible import type: memory limits mismatch")
		}
	}

	// note: multi-memory is not supported, so this is memory 0
	ins.IndexSpace.Memories = append(ins.IndexSpace.Memories, memory)
	return nil
}

func (ins *Instance) applyGlobalImport(importSeg *segments.ImportSegment, externModule *Module, exportSegment *segments.ExportSegment) error {
	if exportSegment.Desc.Index >= uint32(len(externModule.IndexSpace.Globals)) {
		return fmt.Errorf("exported index out of range")
	}

	gb := externModule.IndexSpace.Globals[exportSegment.Desc.Index]
	if want := importSeg.Desc.GlobalTypePtr; want != nil {
		if gb.GlobalType == nil ||
			gb.GlobalType.ValType != want.ValType ||
			gb.GlobalType.Mutable != want.Mutable {
			return fmt.Errorf("incompatible import type: global type mismatch")
		}
	}

	// append the imported global to THIS instance's index space; using the
	// exporter's globals as the base would misplace every global index.
	// mutable globals share the exporter's storage cell.
	ins.IndexSpace.Globals = append(ins.IndexSpace.Globals, gb)
	return nil
}

func (ins *Instance) buildGlobalIndexSpace() error {
	for _, gs := range ins.GlobalSection {
		v, err := ins.execExpr(gs.Init)
		if err != nil {
			return fmt.Errorf("execution failed: %w", err)
		}
		ins.IndexSpace.Globals = append(ins.IndexSpace.Globals, &Global{
			GlobalType: gs.Type,
			Val:        v,
		})
	}
	return nil
}

func (ins *Instance) buildFunctionIndexSpace() error {
	for codeIndex, typeIndex := range ins.FunctionSection {
		if typeIndex >= uint32(len(ins.TypeSection)) {
			return fmt.Errorf("function type index out of range")
		} else if codeIndex >= len(ins.CodeSection) {
			return fmt.Errorf("code index out of range")
		}

		funcIdx := uint32(len(ins.IndexSpace.Functions))
		name := ins.FunctionNames[funcIdx]
		if name == "" {
			name = fmt.Sprintf("func[%d]", funcIdx)
		}
		f := &wasmFunc{
			signature: ins.TypeSection[typeIndex],
			body:      ins.CodeSection[codeIndex].Body,
			NumLocal:  ins.CodeSection[codeIndex].NumLocals,
			owner:     ins,
			name:      name,
		}

		brs, err := ins.parseBlocks(f, f.body)
		if err != nil {
			return fmt.Errorf("parse blocks: %w", err)
		}

		f.Blocks = brs
		if ins.ModuleConfig.EnableJIT {
			ins.compileNative(f)
		}
		ins.IndexSpace.Functions = append(ins.IndexSpace.Functions, f)
	}

	return nil
}

// sizeTablesAndMemories grows every table and memory to its declared minimum,
// so active segments are bounds-checked against the real initial size.
func (ins *Instance) sizeTablesAndMemories() {
	for _, table := range ins.IndexSpace.Tables {
		if table.Limits != nil && int(table.Limits.Min) > len(table.Value) {
			table.Value = append(table.Value, make([]fn, int(table.Limits.Min)-len(table.Value))...)
		}
	}
	for _, memory := range ins.IndexSpace.Memories {
		minBytes := int(memory.Min) * config.DefaultMemoryPageSize
		if len(memory.Value) < minBytes {
			memory.Value = append(memory.Value, make([]byte, minBytes-len(memory.Value))...)
		}
	}
}

// elemPlan is a bounds-checked, resolved active element segment ready to apply.
type elemPlan struct {
	table  *Table
	offset uint64
	funcs  []fn // nil entry = null reference (slot untouched)
}

// dataPlan is a bounds-checked active data segment ready to apply.
type dataPlan struct {
	memory *Memory
	offset uint64
	init   []byte
}

// planElemSegments bounds-checks and resolves every active element segment
// without touching any table, so a failing segment leaves the store unchanged.
func (ins *Instance) planElemSegments() ([]elemPlan, error) {
	var plans []elemPlan
	for _, elem := range ins.ElementsSection {
		// passive/declarative segments are not applied at instantiation.
		if elem.Passive {
			continue
		}
		// note: the table may be imported, so validate against the index space
		// (which includes imported tables) rather than the local section.
		if elem.TableIndex >= uint32(len(ins.IndexSpace.Tables)) {
			return nil, fmt.Errorf("index out of range of index space")
		}

		table := ins.IndexSpace.Tables[elem.TableIndex]

		rawOffset, err := ins.execExpr(elem.OffsetExpr)
		if err != nil {
			return nil, fmt.Errorf("calculate offset: %w", err)
		}

		offset32, ok := rawOffset.(int32)
		if !ok {
			return nil, fmt.Errorf("type assertion failed")
		}

		// an active element segment must fit within the (min-sized) table,
		// otherwise instantiation fails. offsets are unsigned.
		offset := uint64(uint32(offset32))
		if offset+uint64(len(elem.Init)) > uint64(len(table.Value)) {
			return nil, fmt.Errorf("element segment out of bounds")
		}

		// resolve function references up front: on a shared (imported) table a
		// raw index would later be re-resolved in the calling module's function
		// index space, which is wrong across modules.
		funcs := make([]fn, len(elem.Init))
		for i, fi := range elem.Init {
			if fi == segments.NullElem {
				continue // a null reference leaves the slot uninitialized
			}
			if int(fi) >= len(ins.IndexSpace.Functions) {
				return nil, fmt.Errorf("element function index out of range")
			}
			funcs[i] = ins.IndexSpace.Functions[fi]
		}

		plans = append(plans, elemPlan{table: table, offset: offset, funcs: funcs})
	}
	return plans, nil
}

func (ins *Instance) applyElemSegments(plans []elemPlan) {
	for _, p := range plans {
		for i, f := range p.funcs {
			if f != nil {
				p.table.Value[uint64(i)+p.offset] = f
			}
		}
	}
}

// planDataSegments bounds-checks every active data segment without writing.
func (ins *Instance) planDataSegments() ([]dataPlan, error) {
	var plans []dataPlan
	for _, d := range ins.Module.DataSection {
		// passive segments are not applied at instantiation (they are used later
		// via memory.init).
		if d.Passive {
			continue
		}
		// note: the memory may be imported, so validate against the index space
		// (which includes imported memories) rather than the local section.
		if d.MemoryIndex >= uint32(len(ins.IndexSpace.Memories)) {
			return nil, fmt.Errorf("index out of range of index space")
		}

		memory := ins.IndexSpace.Memories[d.MemoryIndex]

		rawOffset, err := ins.execExpr(d.OffsetExpression)
		if err != nil {
			return nil, fmt.Errorf("calculate offset: %w", err)
		}

		offset, ok := rawOffset.(int32)
		if !ok {
			return nil, fmt.Errorf("type assertion failed")
		}

		// an active data segment must fit within the (min-sized) memory,
		// otherwise instantiation traps. addresses are unsigned.
		off := uint64(uint32(offset))
		if off+uint64(len(d.Init)) > uint64(len(memory.Value)) {
			return nil, fmt.Errorf("data segment out of bounds")
		}

		plans = append(plans, dataPlan{memory: memory, offset: off, init: d.Init})
	}
	return plans, nil
}

func (ins *Instance) applyDataSegments(plans []dataPlan) {
	for _, p := range plans {
		copy(p.memory.Value[p.offset:], p.init)
	}
}

// buildMemoryIndexSpace sizes memories and applies the data segments; kept as
// a self-contained helper (buildIndexSpaces uses the split plan/apply flow so
// element and data segments are checked together before either is applied).
func (ins *Instance) buildMemoryIndexSpace() error {
	ins.sizeTablesAndMemories()
	plans, err := ins.planDataSegments()
	if err != nil {
		return err
	}
	ins.applyDataSegments(plans)
	return nil
}

// buildTableIndexSpace sizes tables and applies the element segments; see
// buildMemoryIndexSpace for why buildIndexSpaces uses the split flow instead.
func (ins *Instance) buildTableIndexSpace() error {
	ins.sizeTablesAndMemories()
	plans, err := ins.planElemSegments()
	if err != nil {
		return err
	}
	ins.applyElemSegments(plans)
	return nil
}

type blockType = types.FuncType

func (ins *Instance) readBlockType(r *bytes.Reader) (*blockType, uint64, error) {
	raw, l, err := leb128decode.DecodeInt33AsInt64(r)
	if err != nil {
		return nil, 0, fmt.Errorf("decode int33: %w", err)
	}

	var ret *blockType
	switch raw {
	case -64: // 0x40 in original byte = nil
		ret = &blockType{}
	case -1: // 0x7f in original byte = i32
		ret = &blockType{ReturnTypes: []types.ValueType{types.ValueTypeI32}}
	case -2: // 0x7e in original byte = i64
		ret = &blockType{ReturnTypes: []types.ValueType{types.ValueTypeI64}}
	case -3: // 0x7d in original byte = f32
		ret = &blockType{ReturnTypes: []types.ValueType{types.ValueTypeF32}}
	case -4: // 0x7c in original byte = f64
		ret = &blockType{ReturnTypes: []types.ValueType{types.ValueTypeF64}}
	default:
		if raw < 0 || (raw >= int64(len(ins.TypeSection))) {
			return nil, 0, fmt.Errorf("invalid block type: %d", raw)
		}
		ret = ins.TypeSection[raw]
	}
	return ret, l, nil
}

// parseBlocks scans a function body once, resolving block structure AND
// pre-decoding every immediate so the exec loop never touches LEB128, a
// bytes.Reader or a map at run time.
func (ins *Instance) parseBlocks(f *wasmFunc, body []byte) (map[uint64]*funcBlock, error) {
	ret := map[uint64]*funcBlock{}
	stack := make([]*funcBlock, 0)
	f.imms = make([]uint64, len(body))
	f.pcEnd = make([]uint32, len(body))
	f.brFast = make([]uint32, len(body))
	for pc := uint64(0); pc < uint64(len(body)); pc++ {
		rawOc := body[pc]
		op0 := pc                           // PC of the opcode byte, for the immediate tables
		if 0x28 <= rawOc && rawOc <= 0x3e { // memory load,store
			pc++
			// align (validated statically; unused at run time)
			_, l, err := leb128decode.DecodeUint32(bytes.NewReader(body[pc:]))
			if err != nil {
				return nil, fmt.Errorf("read memory align: %w", err)
			}
			pc += l
			// offset
			off, l, err := leb128decode.DecodeUint32(bytes.NewReader(body[pc:]))
			if err != nil {
				return nil, fmt.Errorf("read memory offset: %w", err)
			}
			pc += l - 1
			f.imms[op0] = uint64(off)
			f.pcEnd[op0] = uint32(pc)
			continue
		} else if 0x41 <= rawOc && rawOc <= 0x44 { // const instructions
			pc++
			switch expr.OpCode(rawOc) {
			case expr.OpCodeI32Const:
				v, l, err := leb128decode.DecodeInt32(bytes.NewReader(body[pc:]))
				if err != nil {
					return nil, fmt.Errorf("read immediate: %w", err)
				}
				pc += l - 1
				f.imms[op0] = uint64(v)
			case expr.OpCodeI64Const:
				v, l, err := leb128decode.DecodeInt64(bytes.NewReader(body[pc:]))
				if err != nil {
					return nil, fmt.Errorf("read immediate: %w", err)
				}
				pc += l - 1
				f.imms[op0] = uint64(v)
			case expr.OpCodeF32Const:
				f.imms[op0] = uint64(binary.LittleEndian.Uint32(body[pc:]))
				pc += 3
			case expr.OpCodeF64Const:
				f.imms[op0] = binary.LittleEndian.Uint64(body[pc:])
				pc += 7
			}
			f.pcEnd[op0] = uint32(pc)
			continue
		} else if (0x3f <= rawOc && rawOc <= 0x40) || // memory grow,size
			(0x20 <= rawOc && rawOc <= 0x24) || // variable instructions
			(0x0c <= rawOc && rawOc <= 0x0d) || // br,br_if instructions
			(0x10 <= rawOc && rawOc <= 0x11) { // call,call_indirect
			pc++
			v, l, err := leb128decode.DecodeUint32(bytes.NewReader(body[pc:]))
			if err != nil {
				return nil, fmt.Errorf("read immediate: %w", err)
			}
			pc += l - 1
			f.imms[op0] = uint64(v)
			if rawOc == 0x0c || rawOc == 0x0d { // br / br_if
				// if the target is an enclosing loop, precompute the direct
				// jump past its opcode and blocktype
				if int(v) < len(stack) {
					target := stack[len(stack)-1-int(v)]
					if body[target.StartAt] == 0x03 { // loop
						f.brFast[op0] = uint32(target.StartAt+target.BlockTypeBytes) + 1
					}
				}
			}
			if rawOc == 0x11 { // call_indirect has a second immediate: the table index
				pc++
				ti, l2, err := leb128decode.DecodeUint32(bytes.NewReader(body[pc:]))
				if err != nil {
					return nil, fmt.Errorf("read call_indirect table index: %w", err)
				}
				pc += l2 - 1
				f.imms[op0] = uint64(v)<<32 | uint64(ti)
			}
			f.pcEnd[op0] = uint32(pc)
			continue
		} else if rawOc == 0xfc { // misc prefix (saturating trunc, bulk memory, ...)
			pc++
			sub, l, err := leb128decode.DecodeUint32(bytes.NewReader(body[pc:]))
			if err != nil {
				return nil, fmt.Errorf("read misc subopcode: %w", err)
			}
			if sub > expr.OpCodeMiscI64TruncSatF64U {
				// bulk-memory ops (memory.init, data.drop, ...) are not implemented
				return nil, fmt.Errorf("unknown misc instruction: 0xfc %d", sub)
			}
			pc += l - 1
			f.imms[op0] = uint64(sub)
			f.pcEnd[op0] = uint32(pc)
			continue
		} else if rawOc == 0x0e { // br_table
			pc++
			r := bytes.NewReader(body[pc:])
			nl, num, err := leb128decode.DecodeUint32(r)
			if err != nil {
				return nil, fmt.Errorf("read immediate: %w", err)
			}

			plan := &brPlan{targets: make([]uint32, nl)}
			for i := uint32(0); i < nl; i++ {
				li, n, err := leb128decode.DecodeUint32(r)
				if err != nil {
					return nil, fmt.Errorf("read immediate: %w", err)
				}
				num += n
				plan.targets[i] = li
			}

			def, l, err := leb128decode.DecodeUint32(r)
			if err != nil {
				return nil, fmt.Errorf("read immediate: %w", err)
			}
			plan.def = def
			if f.brPlans == nil {
				f.brPlans = map[uint64]*brPlan{}
			}
			f.brPlans[op0] = plan
			pc += l + num - 1
			continue
		}

		switch expr.OpCode(rawOc) {
		case expr.OpCodeBlock, expr.OpCodeIf, expr.OpCodeLoop:
			bt, l, err := ins.readBlockType(bytes.NewReader(body[pc+1:]))
			if err != nil {
				return nil, fmt.Errorf("read block: %w", err)
			}
			stack = append(stack, &funcBlock{
				StartAt:        pc,
				BlockType:      bt,
				BlockTypeBytes: l,
			})
			pc += l
		case expr.OpCodeElse:
			if len(stack) == 0 {
				return nil, fmt.Errorf("else outside of an if block")
			}
			stack[len(stack)-1].ElseAt = pc
		case expr.OpCodeEnd:
			if len(stack) == 0 {
				continue // the end of the function body itself
			}
			bl := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			bl.EndAt = pc
			ret[bl.StartAt] = bl
		default:
			// every remaining opcode is a plain single-byte instruction; it must
			// exist in the dispatch table, otherwise the binary is malformed.
			if instructions[expr.OpCode(rawOc)] == nil {
				return nil, fmt.Errorf("illegal opcode: %#x", rawOc)
			}
		}
	}

	if len(stack) > 0 {
		return nil, fmt.Errorf("ill-nested block exists")
	}

	f.blocksAt = make([]*funcBlock, len(body))
	for at, bl := range ret {
		f.blocksAt[at] = bl
	}

	return ret, nil
}
