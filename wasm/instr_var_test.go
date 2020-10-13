package wasm

import (
	"testing"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/stacks"
	"github.com/stretchr/testify/assert"
)

func Test_getLocal(t *testing.T) {
	exp := uint64(100)
	ctx := &wasmContext{
		Func: &wasmFunc{
			body: []byte{byte(expr.OpCodeLocalGet), 0x05},
		},
		Locals: []uint64{0, 0, 0, 0, 0, exp},
	}

	vm := &Instance{Context: ctx, OperandStack: stacks.NewOperandStack()}
	assert.NoError(t, getLocal(vm))
	assert.Equal(t, exp, vm.OperandStack.Pop())
	assert.Equal(t, -1, vm.OperandStack.Ptr)
}

func Test_setLocal(t *testing.T) {
	ctx := &wasmContext{
		Func: &wasmFunc{
			body: []byte{byte(expr.OpCodeLocalSet), 0x05},
		},
		Locals: make([]uint64, 100),
	}

	exp := uint64(100)
	st := stacks.NewOperandStack()
	st.Push(exp)

	vm := &Instance{Context: ctx, OperandStack: st}
	assert.NoError(t, setLocal(vm))
	assert.Equal(t, exp, vm.Context.Locals[5])
	assert.Equal(t, -1, vm.OperandStack.Ptr)
}

func Test_teeLocal(t *testing.T) {
	ctx := &wasmContext{
		Func: &wasmFunc{
			body: []byte{byte(expr.OpCodeLocalTee), 0x05},
		},
		Locals: make([]uint64, 100),
	}

	exp := uint64(100)
	st := stacks.NewOperandStack()
	st.Push(exp)

	vm := &Instance{Context: ctx, OperandStack: st}
	assert.NoError(t, teeLocal(vm))
	assert.Equal(t, exp, vm.Context.Locals[5])
	assert.Equal(t, exp, vm.OperandStack.Pop())
}

func Test_getGlobal(t *testing.T) {
	ctx := &wasmContext{
		Func: &wasmFunc{
			body: []byte{byte(expr.OpCodeGlobalGet), 0x05},
		},
	}

	exp := uint64(1)
	globals := []uint64{0, 0, 0, 0, 0, exp}

	vm := &Instance{
		Context:      ctx,
		OperandStack: stacks.NewOperandStack(),
		Globals:      globals,
	}
	assert.NoError(t, getGlobal(vm))
	assert.Equal(t, exp, vm.OperandStack.Pop())
	assert.Equal(t, -1, vm.OperandStack.Ptr)
}

func Test_setGlobal(t *testing.T) {
	ctx := &wasmContext{
		Func: &wasmFunc{
			body: []byte{byte(expr.OpCodeGlobalSet), 0x05},
		},
	}

	exp := uint64(100)
	st := stacks.NewOperandStack()
	st.Push(exp)

	vm := &Instance{Context: ctx, OperandStack: st, Globals: []uint64{0, 0, 0, 0, 0, 0}}
	assert.NoError(t, setGlobal(vm))
	assert.Equal(t, exp, vm.Globals[5])
	assert.Equal(t, -1, vm.OperandStack.Ptr)
}
