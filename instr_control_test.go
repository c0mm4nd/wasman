package wasman

import (
	"github.com/c0mm4nd/wasman/instr"
	"github.com/c0mm4nd/wasman/stacks"
	"github.com/c0mm4nd/wasman/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_block(t *testing.T) {
	ctx := &wasmContext{
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
	assert.NoError(t, block(&Instance{Context: ctx}))
	assert.Equal(t, &stacks.Label{
		Arity:          1,
		ContinuationPC: 100,
		EndPC:          100,
	}, ctx.LabelStack.Labels[ctx.LabelStack.Ptr])
	assert.Equal(t, uint64(4), ctx.PC)
}

func Test_loop(t *testing.T) {
	ctx := &wasmContext{
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
	assert.NoError(t, loop(&Instance{Context: ctx}))
	assert.Equal(t, &stacks.Label{
		Arity:          1,
		ContinuationPC: 0,
		EndPC:          100,
	}, ctx.LabelStack.Labels[ctx.LabelStack.Ptr])
	assert.Equal(t, uint64(4), ctx.PC)
}

func Test_ifOp(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		ctx := &wasmContext{
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
		vm := &Instance{Context: ctx, OperandStack: stacks.NewOperandStack()}
		vm.OperandStack.Push(1)
		assert.NoError(t, ifOp(vm))
		assert.Equal(t, &stacks.Label{
			Arity:          1,
			ContinuationPC: 100,
			EndPC:          100,
		}, ctx.LabelStack.Labels[ctx.LabelStack.Ptr])
		assert.Equal(t, uint64(4), ctx.PC)
	})
	t.Run("false", func(t *testing.T) {
		ctx := &wasmContext{
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
		vm := &Instance{Context: ctx, OperandStack: stacks.NewOperandStack()}
		vm.OperandStack.Push(0)
		assert.NoError(t, ifOp(vm))
		assert.Equal(t, &stacks.Label{
			Arity:          1,
			ContinuationPC: 100,
			EndPC:          100,
		}, ctx.LabelStack.Labels[ctx.LabelStack.Ptr])
		assert.Equal(t, uint64(50), ctx.PC)
	})
}

func Test_elseOp(t *testing.T) {
	ctx := &wasmContext{
		LabelStack: stacks.NewLabelStack(),
	}

	ctx.LabelStack.Push(&stacks.Label{EndPC: 100000})
	assert.NoError(t, elseOp(&Instance{Context: ctx}))
	assert.Equal(t, uint64(100000), ctx.PC)
}

func Test_end(t *testing.T) {
	ctx := &wasmContext{LabelStack: stacks.NewLabelStack()}
	ctx.LabelStack.Push(&stacks.Label{EndPC: 100000})
	assert.NoError(t, end(&Instance{Context: ctx}))
	assert.Equal(t, -1, ctx.LabelStack.Ptr)
}

func Test_br(t *testing.T) {
	ctx := &wasmContext{
		LabelStack: stacks.NewLabelStack(),
		Func:       &wasmFunc{body: []byte{0x00, 0x01}},
	}
	vm := &Instance{
		Context:      ctx,
		OperandStack: stacks.NewOperandStack()}
	ctx.LabelStack.Push(&stacks.Label{ContinuationPC: 5})
	ctx.LabelStack.Push(&stacks.Label{})
	assert.NoError(t, br(vm))
	assert.Equal(t, uint64(5), ctx.PC)
}

func Test_brIf(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		ctx := &wasmContext{
			LabelStack: stacks.NewLabelStack(),
			Func:       &wasmFunc{body: []byte{0x00, 0x01}},
		}

		vm := &Instance{Context: ctx, OperandStack: stacks.NewOperandStack()}
		vm.OperandStack.Push(1)
		ctx.LabelStack.Push(&stacks.Label{ContinuationPC: 5})
		ctx.LabelStack.Push(&stacks.Label{})
		assert.NoError(t, brIf(vm))
		assert.Equal(t, uint64(5), ctx.PC)
	})

	t.Run("false", func(t *testing.T) {
		ctx := &wasmContext{
			LabelStack: stacks.NewLabelStack(),
			Func:       &wasmFunc{body: []byte{0x00, 0x01}},
		}

		vm := &Instance{Context: ctx, OperandStack: stacks.NewOperandStack()}
		vm.OperandStack.Push(0)
		assert.NoError(t, brIf(vm))
		assert.Equal(t, uint64(1), ctx.PC)
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
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeCall), 0x01},
			},
		},
		Functions: []fn{nil, df},
	}

	assert.NoError(t, call(ins))
	assert.Equal(t, 1, df.cnt)
}

func Test_callIndirect(t *testing.T) {
	df := &dummyFunc{}
	ins := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeCall), 0x01, 0x00},
			},
		},
		Functions: []fn{nil, df},
		Module: &Module{
			TypesSection: []*types.FuncType{nil, {}},
			indexSpace: &indexSpace{
				Tables: [][]*uint32{{nil, uint32Ptr(1)}},
			},
		},
		OperandStack: stacks.NewOperandStack(),
	}
	ins.OperandStack.Push(1)

	assert.NoError(t, callIndirect(ins))
	assert.Equal(t, 1, df.cnt)
}
