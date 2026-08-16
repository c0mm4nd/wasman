package wasm

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/leb128decode"
	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
	"github.com/c0mm4nd/wasman/utils"
)

// This file implements module validation following the algorithm in the
// appendix of the WebAssembly core specification: a typed operand stack plus
// a control-frame stack, with `unknown` standing in for the polymorphic types
// produced by unreachable code.

// ErrInvalidModule wraps all validation failures.
var ErrInvalidModule = errors.New("invalid module")

// vtUnknown is the polymorphic value type used after unreachable.
const vtUnknown types.ValueType = 0

const maxMemoryPages = 65536 // a wasm32 linear memory is at most 2^16 pages (4 GiB)

// Validate statically type-checks the whole module: section indices, limits,
// constant expressions, and every function body.
func (m *Module) Validate() error {
	// build the static index spaces (imports first, then local definitions)
	var funcs []*types.FuncType
	var globals []*types.GlobalType
	numTables, numMemories := 0, 0
	for _, imp := range m.ImportSection {
		switch imp.Desc.Kind {
		case segments.KindFunction:
			if imp.Desc.TypeIndexPtr == nil || *imp.Desc.TypeIndexPtr >= uint32(len(m.TypeSection)) {
				return fmt.Errorf("%w: unknown type for imported function", ErrInvalidModule)
			}
			funcs = append(funcs, m.TypeSection[*imp.Desc.TypeIndexPtr])
		case segments.KindTable:
			numTables++
		case segments.KindMem:
			numMemories++
		case segments.KindGlobal:
			globals = append(globals, imp.Desc.GlobalTypePtr)
		}
	}
	for _, ti := range m.FunctionSection {
		if ti >= uint32(len(m.TypeSection)) {
			return fmt.Errorf("%w: unknown type %d for function", ErrInvalidModule, ti)
		}
		funcs = append(funcs, m.TypeSection[ti])
	}
	numTables += len(m.TableSection)
	numMemories += len(m.MemorySection)

	// multiple tables are supported (reference types), but multi-memory is not
	// implemented: loads/stores always target memory 0.
	if numMemories > 1 {
		return fmt.Errorf("%w: multiple memories are not supported", ErrInvalidModule)
	}

	// limits
	for _, mem := range m.MemorySection {
		if mem.Min > maxMemoryPages {
			return fmt.Errorf("%w: memory min %d exceeds %d pages", ErrInvalidModule, mem.Min, maxMemoryPages)
		}
		if mem.Max != nil && (*mem.Max > maxMemoryPages || *mem.Max < mem.Min) {
			return fmt.Errorf("%w: invalid memory limits", ErrInvalidModule)
		}
	}
	for _, tab := range m.TableSection {
		if tab.Limits != nil && tab.Limits.Max != nil && *tab.Limits.Max < tab.Limits.Min {
			return fmt.Errorf("%w: invalid table limits", ErrInvalidModule)
		}
	}

	// global init expressions; each may only refer to previously known globals
	for _, gs := range m.GlobalSection {
		if err := validateConstExpr(gs.Init, gs.Type.ValType, globals); err != nil {
			return fmt.Errorf("%w: global init: %v", ErrInvalidModule, err)
		}
		globals = append(globals, gs.Type)
	}

	// exports
	for name, exp := range m.ExportSection {
		var max int
		switch exp.Desc.Kind {
		case segments.KindFunction:
			max = len(funcs)
		case segments.KindTable:
			max = numTables
		case segments.KindMem:
			max = numMemories
		case segments.KindGlobal:
			max = len(globals)
		default:
			return fmt.Errorf("%w: invalid export kind %#x", ErrInvalidModule, exp.Desc.Kind)
		}
		if int(exp.Desc.Index) >= max {
			return fmt.Errorf("%w: export %q index out of range", ErrInvalidModule, name)
		}
	}

	// start function must exist with signature [] -> []
	for _, id := range m.StartSection {
		if int(id) >= len(funcs) {
			return fmt.Errorf("%w: unknown start function %d", ErrInvalidModule, id)
		}
		ft := funcs[id]
		if len(ft.InputTypes) != 0 || len(ft.ReturnTypes) != 0 {
			return fmt.Errorf("%w: start function must be [] -> []", ErrInvalidModule)
		}
	}

	// element segments
	for _, e := range m.ElementsSection {
		if !e.Passive {
			if int(e.TableIndex) >= numTables {
				return fmt.Errorf("%w: element segment table index out of range", ErrInvalidModule)
			}
			if err := validateConstExpr(e.OffsetExpr, types.ValueTypeI32, globals); err != nil {
				return fmt.Errorf("%w: element offset: %v", ErrInvalidModule, err)
			}
		}
		for _, fi := range e.Init {
			if fi == segments.NullElem {
				continue // a null reference initializes nothing
			}
			if int(fi) >= len(funcs) {
				return fmt.Errorf("%w: element segment function index out of range", ErrInvalidModule)
			}
		}
	}

	// data segments
	for _, d := range m.DataSection {
		if !d.Passive {
			if int(d.MemoryIndex) >= numMemories {
				return fmt.Errorf("%w: data segment memory index out of range", ErrInvalidModule)
			}
			if err := validateConstExpr(d.OffsetExpression, types.ValueTypeI32, globals); err != nil {
				return fmt.Errorf("%w: data offset: %v", ErrInvalidModule, err)
			}
		}
	}

	// function bodies
	for i, code := range m.CodeSection {
		ft := m.TypeSection[m.FunctionSection[i]]
		v := &funcValidator{
			m:           m,
			funcs:       funcs,
			globals:     globals,
			numTables:   numTables,
			numMemories: numMemories,
			sig:         ft,
			locals:      code.LocalDecls,
		}
		if err := v.run(code.Body); err != nil {
			return fmt.Errorf("%w: function #%d: %v", ErrInvalidModule, i, err)
		}
	}

	return nil
}

// validateConstExpr type-checks a constant expression against the wanted type.
// globals holds the globals visible to the expression (imports and any global
// defined before it); referenced globals must be immutable.
func validateConstExpr(e *expr.Expression, want types.ValueType, globals []*types.GlobalType) error {
	if e == nil {
		return errors.New("missing constant expression")
	}

	globalType := func(data []byte) (types.ValueType, error) {
		id, _, err := leb128decode.DecodeUint32(bytes.NewReader(data))
		if err != nil {
			return 0, fmt.Errorf("read global index: %w", err)
		}
		if int(id) >= len(globals) {
			return 0, errors.New("unknown global in constant expression")
		}
		if globals[id] == nil {
			return 0, errors.New("unknown global type in constant expression")
		}
		if globals[id].Mutable {
			return 0, errors.New("constant expression must not use a mutable global")
		}
		return globals[id].ValType, nil
	}

	if !e.Extended {
		var got types.ValueType
		switch e.OpCode {
		case expr.OpCodeI32Const:
			got = types.ValueTypeI32
		case expr.OpCodeI64Const:
			got = types.ValueTypeI64
		case expr.OpCodeF32Const:
			got = types.ValueTypeF32
		case expr.OpCodeF64Const:
			got = types.ValueTypeF64
		case expr.OpCodeGlobalGet:
			var err error
			got, err = globalType(e.Data)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("non-constant opcode %#x", e.OpCode)
		}
		if got != want {
			return fmt.Errorf("constant expression type mismatch: got %v want %v", got, want)
		}
		return nil
	}

	// extended constant expression: type-check on a small stack
	r := bytes.NewReader(e.Raw)
	var stack []types.ValueType
	for r.Len() > 0 {
		b, err := r.ReadByte()
		if err != nil {
			return err
		}
		switch op := expr.OpCode(b); op {
		case expr.OpCodeI32Const:
			if _, _, err := leb128decode.DecodeInt32(r); err != nil {
				return err
			}
			stack = append(stack, types.ValueTypeI32)
		case expr.OpCodeI64Const:
			if _, _, err := leb128decode.DecodeInt64(r); err != nil {
				return err
			}
			stack = append(stack, types.ValueTypeI64)
		case expr.OpCodeF32Const:
			if _, err := utils.ReadFloat32(r); err != nil {
				return err
			}
			stack = append(stack, types.ValueTypeF32)
		case expr.OpCodeF64Const:
			if _, err := utils.ReadFloat64(r); err != nil {
				return err
			}
			stack = append(stack, types.ValueTypeF64)
		case expr.OpCodeGlobalGet:
			// re-encode the index bytes for globalType
			start := r.Size() - int64(r.Len())
			if _, _, err := leb128decode.DecodeUint32(r); err != nil {
				return err
			}
			end := r.Size() - int64(r.Len())
			buf := make([]byte, end-start)
			if _, err := r.ReadAt(buf, start); err != nil {
				return err
			}
			t, err := globalType(buf)
			if err != nil {
				return err
			}
			stack = append(stack, t)
		case expr.OpCodeI32Add, expr.OpCodeI32Sub, expr.OpCodeI32Mul,
			expr.OpCodeI64Add, expr.OpCodeI64Sub, expr.OpCodeI64Mul:
			t := types.ValueTypeI32
			if op == expr.OpCodeI64Add || op == expr.OpCodeI64Sub || op == expr.OpCodeI64Mul {
				t = types.ValueTypeI64
			}
			if len(stack) < 2 || stack[len(stack)-1] != t || stack[len(stack)-2] != t {
				return errors.New("constant expression type mismatch in arithmetic")
			}
			stack = stack[:len(stack)-1]
		default:
			return fmt.Errorf("non-constant opcode %#x", b)
		}
	}
	if len(stack) != 1 || stack[0] != want {
		return errors.New("constant expression type mismatch")
	}
	return nil
}

// ctrlFrame is one entry of the control stack.
type ctrlFrame struct {
	opcode      expr.OpCode // Block / Loop / If / Else (0 for the function frame)
	startTypes  []types.ValueType
	endTypes    []types.ValueType
	height      int
	unreachable bool
}

// funcValidator validates one function body.
type funcValidator struct {
	m           *Module
	funcs       []*types.FuncType
	globals     []*types.GlobalType
	numTables   int
	numMemories int
	sig         *types.FuncType
	locals      []segments.LocalDecl

	vals  []types.ValueType
	ctrls []ctrlFrame
}

func (v *funcValidator) pushVal(t types.ValueType) { v.vals = append(v.vals, t) }

func (v *funcValidator) popVal() (types.ValueType, error) {
	if len(v.ctrls) == 0 {
		return 0, errors.New("type mismatch: control stack underflow")
	}
	frame := &v.ctrls[len(v.ctrls)-1]
	if len(v.vals) == frame.height {
		if frame.unreachable {
			return vtUnknown, nil
		}
		return 0, errors.New("type mismatch: operand stack underflow")
	}
	t := v.vals[len(v.vals)-1]
	v.vals = v.vals[:len(v.vals)-1]
	return t, nil
}

func (v *funcValidator) popExpect(want types.ValueType) error {
	got, err := v.popVal()
	if err != nil {
		return err
	}
	if got != want && got != vtUnknown && want != vtUnknown {
		return fmt.Errorf("type mismatch: got %v want %v", got, want)
	}
	return nil
}

func (v *funcValidator) popVals(ts []types.ValueType) error {
	for i := len(ts) - 1; i >= 0; i-- {
		if err := v.popExpect(ts[i]); err != nil {
			return err
		}
	}
	return nil
}

func (v *funcValidator) pushVals(ts []types.ValueType) {
	for _, t := range ts {
		v.pushVal(t)
	}
}

func (v *funcValidator) pushCtrl(op expr.OpCode, in, out []types.ValueType) {
	v.ctrls = append(v.ctrls, ctrlFrame{
		opcode:     op,
		startTypes: in,
		endTypes:   out,
		height:     len(v.vals),
	})
	v.pushVals(in)
}

func (v *funcValidator) popCtrl() (ctrlFrame, error) {
	if len(v.ctrls) == 0 {
		return ctrlFrame{}, errors.New("type mismatch: control stack underflow")
	}
	frame := v.ctrls[len(v.ctrls)-1]
	if err := v.popVals(frame.endTypes); err != nil {
		return ctrlFrame{}, err
	}
	if len(v.vals) != frame.height {
		return ctrlFrame{}, errors.New("type mismatch: values remaining on the block stack")
	}
	v.ctrls = v.ctrls[:len(v.ctrls)-1]
	return frame, nil
}

func (v *funcValidator) setUnreachable() {
	if len(v.ctrls) == 0 {
		return
	}
	frame := &v.ctrls[len(v.ctrls)-1]
	v.vals = v.vals[:frame.height]
	frame.unreachable = true
}

// labelTypes gives the types a branch to the frame carries: a loop's params,
// otherwise the block results.
func labelTypes(f *ctrlFrame) []types.ValueType {
	if f.opcode == expr.OpCodeLoop {
		return f.startTypes
	}
	return f.endTypes
}

func (v *funcValidator) frameAt(depth uint32) (*ctrlFrame, error) {
	if uint64(depth) >= uint64(len(v.ctrls)) {
		return nil, errors.New("unknown label: branch depth out of range")
	}
	return &v.ctrls[len(v.ctrls)-1-int(depth)], nil
}

func (v *funcValidator) localType(idx uint32) (types.ValueType, error) {
	i := uint64(idx)
	if i < uint64(len(v.sig.InputTypes)) {
		return v.sig.InputTypes[i], nil
	}
	i -= uint64(len(v.sig.InputTypes))
	for _, d := range v.locals {
		if i < uint64(d.Count) {
			return d.Type, nil
		}
		i -= uint64(d.Count)
	}
	return 0, fmt.Errorf("unknown local %d", idx)
}

// readBlockType resolves a block type: an empty/value type or a type index.
func (v *funcValidator) readBlockType(r *bytes.Reader) (*types.FuncType, error) {
	raw, _, err := leb128decode.DecodeInt33AsInt64(r)
	if err != nil {
		return nil, fmt.Errorf("decode block type: %w", err)
	}
	switch raw {
	case -64:
		return &types.FuncType{}, nil
	case -1:
		return &types.FuncType{ReturnTypes: []types.ValueType{types.ValueTypeI32}}, nil
	case -2:
		return &types.FuncType{ReturnTypes: []types.ValueType{types.ValueTypeI64}}, nil
	case -3:
		return &types.FuncType{ReturnTypes: []types.ValueType{types.ValueTypeF32}}, nil
	case -4:
		return &types.FuncType{ReturnTypes: []types.ValueType{types.ValueTypeF64}}, nil
	default:
		if raw < 0 || raw >= int64(len(v.m.TypeSection)) {
			return nil, fmt.Errorf("unknown block type: %d", raw)
		}
		return v.m.TypeSection[raw], nil
	}
}

// numericSig describes the operand/result types of the plain numeric opcodes.
type numericSig struct {
	in  []types.ValueType
	out types.ValueType
}

var numericSigs = buildNumericSigs()

func buildNumericSigs() map[expr.OpCode]numericSig {
	const (
		i32 = types.ValueTypeI32
		i64 = types.ValueTypeI64
		f32 = types.ValueTypeF32
		f64 = types.ValueTypeF64
	)
	sigs := map[expr.OpCode]numericSig{}
	set := func(from, to expr.OpCode, in []types.ValueType, out types.ValueType) {
		for op := from; op <= to; op++ {
			sigs[op] = numericSig{in, out}
		}
	}

	set(0x45, 0x45, []types.ValueType{i32}, i32)      // i32.eqz
	set(0x46, 0x4f, []types.ValueType{i32, i32}, i32) // i32 comparisons
	set(0x50, 0x50, []types.ValueType{i64}, i32)      // i64.eqz
	set(0x51, 0x5a, []types.ValueType{i64, i64}, i32) // i64 comparisons
	set(0x5b, 0x60, []types.ValueType{f32, f32}, i32) // f32 comparisons
	set(0x61, 0x66, []types.ValueType{f64, f64}, i32) // f64 comparisons

	set(0x67, 0x69, []types.ValueType{i32}, i32)      // i32 clz/ctz/popcnt
	set(0x6a, 0x78, []types.ValueType{i32, i32}, i32) // i32 arithmetic
	set(0x79, 0x7b, []types.ValueType{i64}, i64)      // i64 clz/ctz/popcnt
	set(0x7c, 0x8a, []types.ValueType{i64, i64}, i64) // i64 arithmetic
	set(0x8b, 0x91, []types.ValueType{f32}, f32)      // f32 unary
	set(0x92, 0x98, []types.ValueType{f32, f32}, f32) // f32 arithmetic
	set(0x99, 0x9f, []types.ValueType{f64}, f64)      // f64 unary
	set(0xa0, 0xa6, []types.ValueType{f64, f64}, f64) // f64 arithmetic

	set(0xa7, 0xa7, []types.ValueType{i64}, i32) // i32.wrap_i64
	set(0xa8, 0xa9, []types.ValueType{f32}, i32) // i32.trunc_f32
	set(0xaa, 0xab, []types.ValueType{f64}, i32) // i32.trunc_f64
	set(0xac, 0xad, []types.ValueType{i32}, i64) // i64.extend_i32
	set(0xae, 0xaf, []types.ValueType{f32}, i64) // i64.trunc_f32
	set(0xb0, 0xb1, []types.ValueType{f64}, i64) // i64.trunc_f64
	set(0xb2, 0xb3, []types.ValueType{i32}, f32) // f32.convert_i32
	set(0xb4, 0xb5, []types.ValueType{i64}, f32) // f32.convert_i64
	set(0xb6, 0xb6, []types.ValueType{f64}, f32) // f32.demote_f64
	set(0xb7, 0xb8, []types.ValueType{i32}, f64) // f64.convert_i32
	set(0xb9, 0xba, []types.ValueType{i64}, f64) // f64.convert_i64
	set(0xbb, 0xbb, []types.ValueType{f32}, f64) // f64.promote_f32
	set(0xbc, 0xbc, []types.ValueType{f32}, i32) // i32.reinterpret_f32
	set(0xbd, 0xbd, []types.ValueType{f64}, i64) // i64.reinterpret_f64
	set(0xbe, 0xbe, []types.ValueType{i32}, f32) // f32.reinterpret_i32
	set(0xbf, 0xbf, []types.ValueType{i64}, f64) // f64.reinterpret_i64

	set(0xc0, 0xc1, []types.ValueType{i32}, i32) // i32.extend8_s/16_s
	set(0xc2, 0xc4, []types.ValueType{i64}, i64) // i64.extend8/16/32_s

	return sigs
}

// memOp describes a load/store: value type and the maximum (natural) alignment.
type memOp struct {
	t        types.ValueType
	maxAlign uint32
	store    bool
}

var memOps = map[expr.OpCode]memOp{
	0x28: {types.ValueTypeI32, 2, false}, // i32.load
	0x29: {types.ValueTypeI64, 3, false}, // i64.load
	0x2a: {types.ValueTypeF32, 2, false}, // f32.load
	0x2b: {types.ValueTypeF64, 3, false}, // f64.load
	0x2c: {types.ValueTypeI32, 0, false}, // i32.load8_s
	0x2d: {types.ValueTypeI32, 0, false}, // i32.load8_u
	0x2e: {types.ValueTypeI32, 1, false}, // i32.load16_s
	0x2f: {types.ValueTypeI32, 1, false}, // i32.load16_u
	0x30: {types.ValueTypeI64, 0, false}, // i64.load8_s
	0x31: {types.ValueTypeI64, 0, false}, // i64.load8_u
	0x32: {types.ValueTypeI64, 1, false}, // i64.load16_s
	0x33: {types.ValueTypeI64, 1, false}, // i64.load16_u
	0x34: {types.ValueTypeI64, 2, false}, // i64.load32_s
	0x35: {types.ValueTypeI64, 2, false}, // i64.load32_u
	0x36: {types.ValueTypeI32, 2, true},  // i32.store
	0x37: {types.ValueTypeI64, 3, true},  // i64.store
	0x38: {types.ValueTypeF32, 2, true},  // f32.store
	0x39: {types.ValueTypeF64, 3, true},  // f64.store
	0x3a: {types.ValueTypeI32, 0, true},  // i32.store8
	0x3b: {types.ValueTypeI32, 1, true},  // i32.store16
	0x3c: {types.ValueTypeI64, 0, true},  // i64.store8
	0x3d: {types.ValueTypeI64, 1, true},  // i64.store16
	0x3e: {types.ValueTypeI64, 2, true},  // i64.store32
}

// run validates the whole function body.
func (v *funcValidator) run(body []byte) error {
	r := bytes.NewReader(body)

	// the implicit function frame; note ReadCodeSegment strips the final end.
	v.pushCtrl(0, nil, v.sig.ReturnTypes)
	// params are addressed as locals, not operands: drop them off the stack
	v.vals = v.vals[:0]
	v.ctrls[0].startTypes = nil
	v.ctrls[0].height = 0

	readU32 := func() (uint32, error) {
		x, _, err := leb128decode.DecodeUint32(r)
		return x, err
	}

	for r.Len() > 0 {
		b, err := r.ReadByte()
		if err != nil {
			return err
		}
		op := expr.OpCode(b)

		// numeric fast path
		if sig, ok := numericSigs[op]; ok {
			for i := len(sig.in) - 1; i >= 0; i-- {
				if err := v.popExpect(sig.in[i]); err != nil {
					return err
				}
			}
			v.pushVal(sig.out)
			continue
		}

		// memory loads/stores
		if mo, ok := memOps[op]; ok {
			if v.numMemories == 0 {
				return errors.New("unknown memory: no memory defined")
			}
			align, err := readU32()
			if err != nil {
				return err
			}
			if _, err := readU32(); err != nil { // offset
				return err
			}
			if align > mo.maxAlign {
				return fmt.Errorf("alignment must not be larger than natural (2^%d)", mo.maxAlign)
			}
			if mo.store {
				if err := v.popExpect(mo.t); err != nil {
					return err
				}
				if err := v.popExpect(types.ValueTypeI32); err != nil {
					return err
				}
			} else {
				if err := v.popExpect(types.ValueTypeI32); err != nil {
					return err
				}
				v.pushVal(mo.t)
			}
			continue
		}

		switch op {
		case expr.OpCodeUnreachable:
			v.setUnreachable()

		case expr.OpCodeNop:

		case expr.OpCodeBlock, expr.OpCodeLoop:
			bt, err := v.readBlockType(r)
			if err != nil {
				return err
			}
			if err := v.popVals(bt.InputTypes); err != nil {
				return err
			}
			v.pushCtrl(op, bt.InputTypes, bt.ReturnTypes)

		case expr.OpCodeIf:
			bt, err := v.readBlockType(r)
			if err != nil {
				return err
			}
			if err := v.popExpect(types.ValueTypeI32); err != nil {
				return err
			}
			if err := v.popVals(bt.InputTypes); err != nil {
				return err
			}
			v.pushCtrl(op, bt.InputTypes, bt.ReturnTypes)

		case expr.OpCodeElse:
			frame, err := v.popCtrl()
			if err != nil {
				return err
			}
			if frame.opcode != expr.OpCodeIf {
				return errors.New("else without a matching if")
			}
			v.pushCtrl(expr.OpCodeElse, frame.startTypes, frame.endTypes)

		case expr.OpCodeEnd:
			frame, err := v.popCtrl()
			if err != nil {
				return err
			}
			// an if without an else must have matching input/output types
			if frame.opcode == expr.OpCodeIf && !types.HasSameSignature(frame.startTypes, frame.endTypes) {
				return errors.New("type mismatch: if without else must not change types")
			}
			v.pushVals(frame.endTypes)
			// the end that closes the function body's implicit frame must be
			// the last byte; anything after it would operate on an empty
			// control stack
			if len(v.ctrls) == 0 {
				if r.Len() != 0 {
					return errors.New("unexpected bytes after function end")
				}
				return nil
			}

		case expr.OpCodeBr:
			l, err := readU32()
			if err != nil {
				return err
			}
			frame, err := v.frameAt(l)
			if err != nil {
				return err
			}
			if err := v.popVals(labelTypes(frame)); err != nil {
				return err
			}
			v.setUnreachable()

		case expr.OpCodeBrIf:
			l, err := readU32()
			if err != nil {
				return err
			}
			frame, err := v.frameAt(l)
			if err != nil {
				return err
			}
			if err := v.popExpect(types.ValueTypeI32); err != nil {
				return err
			}
			lt := labelTypes(frame)
			if err := v.popVals(lt); err != nil {
				return err
			}
			v.pushVals(lt)

		case expr.OpCodeBrTable:
			n, err := readU32()
			if err != nil {
				return err
			}
			targets := make([]uint32, n)
			for i := range targets {
				if targets[i], err = readU32(); err != nil {
					return err
				}
			}
			ln, err := readU32()
			if err != nil {
				return err
			}
			if err := v.popExpect(types.ValueTypeI32); err != nil {
				return err
			}
			defFrame, err := v.frameAt(ln)
			if err != nil {
				return err
			}
			defTypes := labelTypes(defFrame)
			for _, l := range targets {
				frame, err := v.frameAt(l)
				if err != nil {
					return err
				}
				lt := labelTypes(frame)
				if len(lt) != len(defTypes) {
					return errors.New("type mismatch: br_table targets differ in arity")
				}
				if err := v.popVals(lt); err != nil {
					return err
				}
				v.pushVals(lt)
			}
			if err := v.popVals(defTypes); err != nil {
				return err
			}
			v.setUnreachable()

		case expr.OpCodeReturn:
			if err := v.popVals(v.sig.ReturnTypes); err != nil {
				return err
			}
			v.setUnreachable()

		case expr.OpCodeCall:
			x, err := readU32()
			if err != nil {
				return err
			}
			if int(x) >= len(v.funcs) {
				return fmt.Errorf("unknown function %d", x)
			}
			ft := v.funcs[x]
			if err := v.popVals(ft.InputTypes); err != nil {
				return err
			}
			v.pushVals(ft.ReturnTypes)

		case expr.OpCodeCallIndirect:
			x, err := readU32()
			if err != nil {
				return err
			}
			ti, err := readU32()
			if err != nil {
				return err
			}
			if int(ti) >= v.numTables {
				return errors.New("unknown table: call_indirect requires a table")
			}
			if int(x) >= len(v.m.TypeSection) {
				return fmt.Errorf("unknown type %d", x)
			}
			ft := v.m.TypeSection[x]
			if err := v.popExpect(types.ValueTypeI32); err != nil {
				return err
			}
			if err := v.popVals(ft.InputTypes); err != nil {
				return err
			}
			v.pushVals(ft.ReturnTypes)

		case expr.OpCodeDrop:
			if _, err := v.popVal(); err != nil {
				return err
			}

		case expr.OpCodeSelect:
			if err := v.popExpect(types.ValueTypeI32); err != nil {
				return err
			}
			t1, err := v.popVal()
			if err != nil {
				return err
			}
			t2, err := v.popVal()
			if err != nil {
				return err
			}
			if t1 != t2 && t1 != vtUnknown && t2 != vtUnknown {
				return errors.New("type mismatch: select operands differ")
			}
			if t1 == vtUnknown {
				v.pushVal(t2)
			} else {
				v.pushVal(t1)
			}

		case expr.OpCodeLocalGet:
			x, err := readU32()
			if err != nil {
				return err
			}
			t, err := v.localType(x)
			if err != nil {
				return err
			}
			v.pushVal(t)

		case expr.OpCodeLocalSet:
			x, err := readU32()
			if err != nil {
				return err
			}
			t, err := v.localType(x)
			if err != nil {
				return err
			}
			if err := v.popExpect(t); err != nil {
				return err
			}

		case expr.OpCodeLocalTee:
			x, err := readU32()
			if err != nil {
				return err
			}
			t, err := v.localType(x)
			if err != nil {
				return err
			}
			if err := v.popExpect(t); err != nil {
				return err
			}
			v.pushVal(t)

		case expr.OpCodeGlobalGet:
			x, err := readU32()
			if err != nil {
				return err
			}
			if int(x) >= len(v.globals) || v.globals[x] == nil {
				return fmt.Errorf("unknown global %d", x)
			}
			v.pushVal(v.globals[x].ValType)

		case expr.OpCodeGlobalSet:
			x, err := readU32()
			if err != nil {
				return err
			}
			if int(x) >= len(v.globals) || v.globals[x] == nil {
				return fmt.Errorf("unknown global %d", x)
			}
			if !v.globals[x].Mutable {
				return fmt.Errorf("global %d is immutable", x)
			}
			if err := v.popExpect(v.globals[x].ValType); err != nil {
				return err
			}

		case expr.OpCodeMemorySize:
			if _, err := readU32(); err != nil {
				return err
			}
			if v.numMemories == 0 {
				return errors.New("unknown memory: no memory defined")
			}
			v.pushVal(types.ValueTypeI32)

		case expr.OpCodeMemoryGrow:
			if _, err := readU32(); err != nil {
				return err
			}
			if v.numMemories == 0 {
				return errors.New("unknown memory: no memory defined")
			}
			if err := v.popExpect(types.ValueTypeI32); err != nil {
				return err
			}
			v.pushVal(types.ValueTypeI32)

		case expr.OpCodeI32Const:
			if _, _, err := leb128decode.DecodeInt32(r); err != nil {
				return err
			}
			v.pushVal(types.ValueTypeI32)

		case expr.OpCodeI64Const:
			if _, _, err := leb128decode.DecodeInt64(r); err != nil {
				return err
			}
			v.pushVal(types.ValueTypeI64)

		case expr.OpCodeF32Const:
			if _, err := utils.ReadFloat32(r); err != nil {
				return err
			}
			v.pushVal(types.ValueTypeF32)

		case expr.OpCodeF64Const:
			if _, err := utils.ReadFloat64(r); err != nil {
				return err
			}
			v.pushVal(types.ValueTypeF64)

		case expr.OpCodeMiscPrefix:
			sub, err := readU32()
			if err != nil {
				return err
			}
			switch sub {
			case expr.OpCodeMiscMemoryCopy:
				// two memory-index immediates (dst, src); the operands
				// are (dst, src, n): three i32 in, nothing out
				if _, err := readU32(); err != nil {
					return err
				}
				if _, err := readU32(); err != nil {
					return err
				}
				for i := 0; i < 3; i++ {
					if err := v.popExpect(types.ValueTypeI32); err != nil {
						return err
					}
				}
				continue
			case expr.OpCodeMiscMemoryFill:
				// one memory-index immediate; operands (dst, val, n)
				if _, err := readU32(); err != nil {
					return err
				}
				for i := 0; i < 3; i++ {
					if err := v.popExpect(types.ValueTypeI32); err != nil {
						return err
					}
				}
				continue
			}
			if sub > expr.OpCodeMiscI64TruncSatF64U {
				return fmt.Errorf("unknown misc instruction 0xfc %d", sub)
			}
			from := types.ValueTypeF32
			if sub == 2 || sub == 3 || sub == 6 || sub == 7 {
				from = types.ValueTypeF64
			}
			to := types.ValueTypeI32
			if sub >= 4 {
				to = types.ValueTypeI64
			}
			if err := v.popExpect(from); err != nil {
				return err
			}
			v.pushVal(to)

		default:
			return fmt.Errorf("illegal opcode %#x", b)
		}
	}

	// close the implicit function frame
	if len(v.ctrls) != 1 {
		return errors.New("type mismatch: unclosed block")
	}
	if _, err := v.popCtrl(); err != nil {
		return err
	}
	if len(v.vals) != 0 {
		return errors.New("type mismatch: values remaining on the stack")
	}
	return nil
}
