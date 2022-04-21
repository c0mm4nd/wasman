package wasm

import (
	"reflect"
	"testing"

	"github.com/c0mm4nd/wasman/utils"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/stacks"
	"github.com/c0mm4nd/wasman/types"
)

func Test_block(t *testing.T) {
	ctx := &Frame{
		PC: 1,
		Func: &wasmFunc{
			Blocks: map[uint64]*funcBlock{
				1: {
					StartAt:        1,
					EndAt:          100,
					BlockTypeBytes: 3,
					BlockType:      &types.FuncType{ReturnTypes: []types.ValueType{types.ValueTypeI32}},
				},
			},
		},
		LabelStack: stacks.NewLabelStack(),
	}
	if block(&Instance{Active: ctx}) != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(&stacks.Label{
		Arity:          1,
		ContinuationPC: 100,
		EndPC:          100,
	}, ctx.LabelStack.Values[ctx.LabelStack.Ptr]) {
		t.Fail()
	}
	if ctx.PC != 4 {
		t.Fail()
	}
}

func Test_loop(t *testing.T) {
	ctx := &Frame{
		PC: 1,
		Func: &wasmFunc{
			Blocks: map[uint64]*funcBlock{
				1: {
					StartAt:        1,
					EndAt:          100,
					BlockTypeBytes: 3,
					BlockType:      &types.FuncType{ReturnTypes: []types.ValueType{types.ValueTypeI32}},
				},
			},
		},
		LabelStack: stacks.NewLabelStack(),
	}
	if loop(&Instance{Active: ctx}) != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(&stacks.Label{
		Arity:          1,
		ContinuationPC: 0,
		EndPC:          100,
	}, ctx.LabelStack.Values[ctx.LabelStack.Ptr]) {
		t.Fail()
	}
	if ctx.PC != 4 {
		t.Fail()
	}
}

func Test_ifOp(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		ctx := &Frame{
			PC: 1,
			Func: &wasmFunc{
				Blocks: map[uint64]*funcBlock{
					1: {
						StartAt:        1,
						EndAt:          100,
						BlockTypeBytes: 3,
						BlockType:      &types.FuncType{ReturnTypes: []types.ValueType{types.ValueTypeI32}},
					},
				},
			},
			LabelStack: stacks.NewLabelStack(),
		}
		vm := &Instance{Active: ctx, OperandStack: stacks.NewOperandStack()}
		vm.OperandStack.Push(1)
		if ifOp(vm) != nil {
			t.Fail()
		}
		if !reflect.DeepEqual(&stacks.Label{
			Arity:          1,
			ContinuationPC: 100,
			EndPC:          100,
		}, ctx.LabelStack.Values[ctx.LabelStack.Ptr]) {
			t.Fail()
		}
		if ctx.PC != 4 {
			t.Fail()
		}
	})
	t.Run("false", func(t *testing.T) {
		ctx := &Frame{
			PC: 1,
			Func: &wasmFunc{
				Blocks: map[uint64]*funcBlock{
					1: {
						StartAt:        1,
						ElseAt:         50,
						EndAt:          100,
						BlockTypeBytes: 3,
						BlockType:      &types.FuncType{ReturnTypes: []types.ValueType{types.ValueTypeI32}},
					},
				},
			},
			LabelStack: stacks.NewLabelStack(),
		}
		vm := &Instance{Active: ctx, OperandStack: stacks.NewOperandStack()}
		vm.OperandStack.Push(0)
		if ifOp(vm) != nil {
			t.Fail()
		}
		if !reflect.DeepEqual(&stacks.Label{
			Arity:          1,
			ContinuationPC: 100,
			EndPC:          100,
		}, ctx.LabelStack.Values[ctx.LabelStack.Ptr]) {
			t.Fail()
		}
		if ctx.PC != 50 {
			t.Fail()
		}
	})
}

func Test_elseOp(t *testing.T) {
	ctx := &Frame{
		LabelStack: stacks.NewLabelStack(),
	}

	ctx.LabelStack.Push(&stacks.Label{EndPC: 100000})
	if elseOp(&Instance{Active: ctx}) != nil {
		t.Fail()
	}
	if ctx.PC != 100000 {
		t.Fail()
	}
}

func Test_end(t *testing.T) {
	ctx := &Frame{LabelStack: stacks.NewLabelStack()}
	ctx.LabelStack.Push(&stacks.Label{EndPC: 100000})
	if end(&Instance{Active: ctx}) != nil {
		t.Fail()
	}
	if ctx.LabelStack.Ptr != -1 {
		t.Fail()
	}
}

func Test_br(t *testing.T) {
	ctx := &Frame{
		LabelStack: stacks.NewLabelStack(),
		Func:       &wasmFunc{body: []byte{0x00, 0x01}},
	}
	vm := &Instance{
		Active:       ctx,
		OperandStack: stacks.NewOperandStack()}
	ctx.LabelStack.Push(&stacks.Label{ContinuationPC: 5})
	ctx.LabelStack.Push(&stacks.Label{})
	if br(vm) != nil {
		t.Fail()
	}
	if ctx.PC != 5 {
		t.Fail()
	}
}

func Test_brIf(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		ctx := &Frame{
			LabelStack: stacks.NewLabelStack(),
			Func:       &wasmFunc{body: []byte{0x00, 0x01}},
		}

		vm := &Instance{Active: ctx, OperandStack: stacks.NewOperandStack()}
		vm.OperandStack.Push(1)
		ctx.LabelStack.Push(&stacks.Label{ContinuationPC: 5})
		ctx.LabelStack.Push(&stacks.Label{})
		if brIf(vm) != nil {
			t.Fail()
		}
		if ctx.PC != 5 {
			t.Fail()
		}
	})

	t.Run("false", func(t *testing.T) {
		ctx := &Frame{
			LabelStack: stacks.NewLabelStack(),
			Func:       &wasmFunc{body: []byte{0x00, 0x01}},
		}

		vm := &Instance{Active: ctx, OperandStack: stacks.NewOperandStack()}
		vm.OperandStack.Push(0)
		if brIf(vm) != nil {
			t.Fail()
		}
		if ctx.PC != 1 {
			t.Fail()
		}
	})
}

func Test_brAt(t *testing.T) {
	// fixme:
}

func Test_brTable(t *testing.T) {
	// fixme:
}

type dummyFunc struct {
	cnt int
}

func (d *dummyFunc) call(_ *Instance) error {
	d.cnt++
	return nil
}

func (d *dummyFunc) getType() *types.FuncType { return &types.FuncType{} }

func Test_call(t *testing.T) {
	df := &dummyFunc{}
	ins := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeCall), 0x01},
			},
		},
		Functions: []fn{nil, df},
	}

	if call(ins) != nil {
		t.Fail()
	}
	if df.cnt != 1 {
		t.Fail()
	}
}

func Test_callIndirect(t *testing.T) {
	df := &dummyFunc{}
	ins := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeCall), 0x01, 0x00},
			},
		},
		Functions: []fn{nil, df},
		Module: &Module{
			TypeSection: []*types.FuncType{nil, {}},
			IndexSpace: &IndexSpace{
				Tables: []*Table{
					{Value: []*uint32{nil, utils.Uint32Ptr(1)}},
				},
			},
		},
		OperandStack: stacks.NewOperandStack(),
	}
	ins.OperandStack.Push(1)

	if callIndirect(ins) != nil {
		t.Fail()
	}
	if df.cnt != 1 {
		t.Fail()
	}
}
