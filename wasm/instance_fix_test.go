package wasm

import (
	"errors"
	"testing"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/stacks"
	"github.com/c0mm4nd/wasman/types"
)

// Test_memoryBase_accessWidth checks a partially out-of-range access traps
// with ErrPtrOutOfBounds instead of panicking on the slice access.
func Test_memoryBase_accessWidth(t *testing.T) {
	// body: opcode(i32.load placeholder), align=0, offset=0
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{body: []byte{0x28, 0x00, 0x00}},
		},
		Memory:       &Memory{Value: make([]byte, 4)},
		OperandStack: stacks.NewOperandStack(),
	}
	vm.OperandStack.Push(2) // base 2 + width 4 > len 4

	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panicked instead of trapping: %v", r)
			}
		}()
		return i32Load(vm)
	}()
	if !errors.Is(err, ErrPtrOutOfBounds) {
		t.Errorf("expected ErrPtrOutOfBounds, got %v", err)
	}

	// fully in-range access still works
	vm.Active.PC = 0
	vm.OperandStack.Push(0)
	if err := i32Load(vm); err != nil {
		t.Errorf("in-range load failed: %v", err)
	}
}

// Test_applyGlobalImport_indexSpace guards against using the exporter's
// globals as the base of the importer's index space, which misplaced every
// global index when the exporter had more than one global.
func Test_applyGlobalImport_indexSpace(t *testing.T) {
	exporter := &Module{
		IndexSpace: &IndexSpace{
			Globals: []*Global{
				{GlobalType: &types.GlobalType{ValType: types.ValueTypeI32}, Val: int32(111)},
				{GlobalType: &types.GlobalType{ValType: types.ValueTypeI32}, Val: int32(222)},
				{GlobalType: &types.GlobalType{ValType: types.ValueTypeI32}, Val: int32(333)},
			},
		},
	}

	importer := &Instance{Module: &Module{}, IndexSpace: &IndexSpace{}}
	// import the exporter's global #1 -> must become the importer's global #0
	err := importer.applyGlobalImport(
		&segments.ImportSegment{Desc: &segments.ImportDesc{Kind: segments.KindGlobal}},
		exporter,
		&segments.ExportSegment{
			Desc: &segments.ExportDesc{Kind: segments.KindGlobal, Index: 1},
		})
	if err != nil {
		t.Fatal(err)
	}

	if n := len(importer.IndexSpace.Globals); n != 1 {
		t.Fatalf("importer index space has %d globals, want 1", n)
	}
	if got := importer.IndexSpace.Globals[0].Val; got != int32(222) {
		t.Errorf("imported global value = %v, want 222", got)
	}

	// a locally defined global must land at index 1, right after the import
	importer.Module.GlobalSection = []*segments.GlobalSegment{{
		Type: &types.GlobalType{ValType: types.ValueTypeI32},
		Init: &expr.Expression{OpCode: expr.OpCodeI32Const, Data: []byte{0x2A}}, // 42
	}}
	if err := importer.buildGlobalIndexSpace(); err != nil {
		t.Fatal(err)
	}
	if n := len(importer.IndexSpace.Globals); n != 2 {
		t.Fatalf("importer index space has %d globals, want 2", n)
	}
	if got := importer.IndexSpace.Globals[1].Val; got != int32(42) {
		t.Errorf("local global value = %v, want 42", got)
	}
}

// Test_elemSegment_outOfBounds checks an active element segment that does not
// fit the (min-sized) table fails instantiation instead of silently growing
// the table.
func Test_elemSegment_outOfBounds(t *testing.T) {
	m := &Module{
		ElementsSection: []*segments.ElemSegment{{
			TableIndex: 0,
			OffsetExpr: &expr.Expression{OpCode: expr.OpCodeI32Const, Data: []byte{0x05}},
			Init:       []uint32{0},
		}},
		TableSection: []*types.TableType{{Limits: &types.Limits{Min: 1}}},
		IndexSpace: &IndexSpace{Tables: []*Table{
			{TableType: types.TableType{Limits: &types.Limits{Min: 1}}, Value: []fn{}},
		}},
	}
	if err := (&Instance{Module: m, IndexSpace: m.IndexSpace}).buildTableIndexSpace(); err == nil {
		t.Error("expected an out-of-bounds error for the element segment")
	}
}

// Test_trapStackRollback checks a trapped call leaves the operand stack at its
// pre-call height (repeated traps must not grow the stack unboundedly).
func Test_trapStackRollback(t *testing.T) {
	// body: unreachable
	f := &wasmFunc{
		signature: &types.FuncType{},
		body:      []byte{0x00},
		Blocks:    map[uint64]*funcBlock{},
	}
	ins := &Instance{
		Module:       &Module{},
		OperandStack: stacks.NewOperandStack(),
		FrameStack: &stacks.Stack[*Frame]{
			Ptr:    -1,
			Values: make([]*Frame, stacks.InitialLabelStackHeight),
		},
	}
	for i := 0; i < 5; i++ {
		if err := f.call(ins); err == nil {
			t.Fatal("expected a trap")
		}
		if ins.OperandStack.Ptr != -1 {
			t.Fatalf("iteration %d: operand stack leaked, Ptr=%d want -1", i, ins.OperandStack.Ptr)
		}
	}
}

// Test_resolveImports_uninstantiated checks importing from a module that was
// never instantiated is a clean link error, not a nil-pointer panic.
func Test_resolveImports_uninstantiated(t *testing.T) {
	importer := &Instance{
		Module: &Module{
			ImportSection: []*segments.ImportSegment{
				{Module: "a", Name: "b", Desc: &segments.ImportDesc{Kind: segments.KindFunction}},
			},
		},
		IndexSpace: &IndexSpace{},
	}
	// the extern module has an ExportSection but was never instantiated
	// (IndexSpace == nil, as NewModule leaves it)
	extern := &Module{
		ExportSection: map[string]*segments.ExportSegment{
			"b": {Name: "b", Desc: &segments.ExportDesc{Kind: segments.KindFunction}},
		},
	}
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panicked instead of returning a link error: %v", r)
			}
		}()
		return importer.resolveImports(map[string]*Module{"a": extern})
	}()
	if err == nil {
		t.Error("expected a link error for the uninstantiated extern module")
	}
}
