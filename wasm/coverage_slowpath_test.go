package wasm

import (
	"testing"

	"github.com/c0mm4nd/wasman/stacks"
	"github.com/c0mm4nd/wasman/types"
)

// rawFrame builds an instance around a hand-made frame with NO pre-decoded
// immediates, forcing every handler down its byte-decoding slow path.
func rawFrame(body []byte, locals []uint64, globals []*Global) *Instance {
	ins := &Instance{
		OperandStack: stacks.NewOperandStack(),
		FrameStack:   &stacks.Stack[*Frame]{Ptr: -1, Values: make([]*Frame, 4)},
		IndexSpace:   &IndexSpace{Globals: globals},
	}
	ins.Globals = make([]*uint64, len(globals))
	for i, g := range globals {
		ins.Globals[i] = g.ensureCell()
	}
	ins.Active = &Frame{
		Func:       &wasmFunc{body: body},
		Locals:     locals,
		LabelStack: stacks.NewLabelStack(),
	}
	return ins
}

func TestSlowPathConstsAndVars(t *testing.T) {
	g := &Global{GlobalType: &types.GlobalType{ValType: types.ValueTypeI64, Mutable: true}, Val: int64(7)}

	run := func(name string, body []byte, locals []uint64, want uint64) {
		ins := rawFrame(body, locals, []*Global{g})
		if err := instructions[body[0]](ins); err != nil {
			t.Errorf("%s: %v", name, err)
			return
		}
		if got := ins.OperandStack.Peek(); got != want {
			t.Errorf("%s: got %#x want %#x", name, got, want)
		}
	}
	run("i32.const", []byte{0x41, 0x7f}, nil, 0xffffffffffffffff) // -1 sign-extended
	run("i64.const", []byte{0x42, 0x2a}, nil, 42)
	run("f32.const", []byte{0x43, 0x00, 0x00, 0xc0, 0x3f}, nil, 0x3fc00000)
	run("f64.const", []byte{0x44, 0, 0, 0, 0, 0, 0, 0xf8, 0x3f}, nil, 0x3ff8000000000000)
	run("local.get", []byte{0x20, 0x01}, []uint64{5, 99}, 99)
	run("global.get", []byte{0x23, 0x00}, nil, 7)

	// local.set / local.tee / global.set consume a pushed value
	ins := rawFrame([]byte{0x21, 0x00}, []uint64{0}, []*Global{g})
	ins.OperandStack.Push(123)
	if err := instructions[0x21](ins); err != nil || ins.Active.Locals[0] != 123 {
		t.Errorf("local.set: %v locals=%v", err, ins.Active.Locals)
	}
	ins = rawFrame([]byte{0x22, 0x00}, []uint64{0}, []*Global{g})
	ins.OperandStack.Push(9)
	if err := instructions[0x22](ins); err != nil || ins.Active.Locals[0] != 9 || ins.OperandStack.Peek() != 9 {
		t.Errorf("local.tee: %v", err)
	}
	ins = rawFrame([]byte{0x24, 0x00}, nil, []*Global{g})
	ins.OperandStack.Push(1000)
	if err := instructions[0x24](ins); err != nil || *ins.Globals[0] != 1000 {
		t.Errorf("global.set: %v cell=%d", err, *ins.Globals[0])
	}

	// decode errors on every slow path
	for _, bad := range [][]byte{
		{0x41, 0x80}, {0x42, 0x80},
		{0x20, 0x80}, {0x21, 0x80}, {0x22, 0x80}, {0x23, 0x80}, {0x24, 0x80},
	} {
		ins := rawFrame(bad, []uint64{0}, []*Global{g})
		ins.OperandStack.Push(0)
		if err := instructions[bad[0]](ins); err == nil {
			t.Errorf("opcode %#x truncated immediate: expected error", bad[0])
		}
	}
}

// the slow paths of block/loop/br/br_if and memory ops decode structure
// from Blocks maps and raw bytes.
func TestSlowPathControlAndMem(t *testing.T) {
	// block at pc 0 spanning to end, then unreachable skipped by br 0
	body := []byte{0x02, 0x40, 0x0c, 0x00, 0x00, 0x0b} // block; br 0; unreachable; end
	blk := &funcBlock{StartAt: 0, EndAt: 5, BlockType: &types.FuncType{}, BlockTypeBytes: 1}
	ins := rawFrame(body, nil, nil)
	ins.Active.Func.Blocks = map[uint64]*funcBlock{0: blk}
	if err := instructions[0x02](ins); err != nil {
		t.Fatalf("block slow: %v", err)
	}
	if ins.Active.LabelStack.Ptr != 0 {
		t.Fatalf("block did not push a label")
	}
	// loop slow path
	body2 := []byte{0x03, 0x40, 0x0b}
	blk2 := &funcBlock{StartAt: 0, EndAt: 2, BlockType: &types.FuncType{}, BlockTypeBytes: 1}
	ins2 := rawFrame(body2, nil, nil)
	ins2.Active.Func.Blocks = map[uint64]*funcBlock{0: blk2}
	if err := instructions[0x03](ins2); err != nil {
		t.Fatalf("loop slow: %v", err)
	}
	// block/loop with a MISSING Blocks entry must error, not panic
	ins3 := rawFrame(body, nil, nil)
	ins3.Active.Func.Blocks = map[uint64]*funcBlock{}
	if err := instructions[0x02](ins3); err == nil {
		t.Fatal("block without metadata: expected error")
	}
	ins4 := rawFrame(body2, nil, nil)
	ins4.Active.Func.Blocks = map[uint64]*funcBlock{}
	if err := instructions[0x03](ins4); err == nil {
		t.Fatal("loop without metadata: expected error")
	}

	// memoryBase slow path: alignment byte + offset from raw bytes
	ins5 := rawFrame([]byte{0x28, 0x02, 0x04}, nil, nil) // i32.load align=2 offset=4
	ins5.Memory = &Memory{Value: make([]byte, 64)}
	ins5.Memory.Value[8] = 0x2a
	ins5.OperandStack.Push(4) // addr 4 + offset 4 = 8
	if err := instructions[0x28](ins5); err != nil || uint32(ins5.OperandStack.Peek()) != 0x2a {
		t.Fatalf("i32.load slow: %v got %#x", err, ins5.OperandStack.Peek())
	}
	// truncated memarg
	ins6 := rawFrame([]byte{0x28, 0x02}, nil, nil)
	ins6.Memory = &Memory{Value: make([]byte, 64)}
	ins6.OperandStack.Push(0)
	if err := instructions[0x28](ins6); err == nil {
		t.Fatal("truncated memarg: expected error")
	}
}
