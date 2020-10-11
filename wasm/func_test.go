package wasm

import (
	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/stacks"
	"github.com/c0mm4nd/wasman/types"
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
)

func TestHostFunction_Call(t *testing.T) {
	var cnt int64
	f := func(in int64) (int32, int64, float32, float64) {
		cnt += in
		return 1, 2, 3, 4
	}
	hf := &HostFunc{
		function: f,
		Signature: &types.FuncType{
			InputTypes:  []types.ValueType{types.ValueTypeI64},
			ReturnTypes: []types.ValueType{types.ValueTypeI32, types.ValueTypeI64, types.ValueTypeF32, types.ValueTypeF64},
		},
	}

	vm := &Instance{OperandStack: stacks.NewOperandStack()}
	vm.OperandStack.Push(10)

	assert.NoError(t, hf.call(vm))
	assert.Equal(t, 3, vm.OperandStack.Ptr)
	assert.Equal(t, int64(10), cnt)

	// f64
	assert.Equal(t, 4.0, math.Float64frombits(vm.OperandStack.Pop()))
	assert.Equal(t, float32(3.0), float32(math.Float64frombits(vm.OperandStack.Pop())))
	assert.Equal(t, int64(2), int64(vm.OperandStack.Pop()))
	assert.Equal(t, int32(1), int32(vm.OperandStack.Pop()))
}

func TestNativeFunction_Call(t *testing.T) {
	n := &wasmFunc{
		signature: &types.FuncType{},
		body: []byte{
			byte(expr.OpCodeI64Const), 0x05, byte(expr.OpCodeReturn),
		},
	}
	vm := &Instance{
		OperandStack: stacks.NewOperandStack(),
		Context: &wasmContext{
			PC: 1000,
		},
	}

	assert.NoError(t, n.call(vm))
	assert.Equal(t, uint64(0x05), vm.OperandStack.Pop())
	assert.Equal(t, uint64(1000), vm.Context.PC)
}

func TestVirtualMachine_execNativeFunction(t *testing.T) {
	n := &wasmFunc{
		signature: &types.FuncType{},
		body: []byte{
			byte(expr.OpCodeI64Const), 0x05,
			byte(expr.OpCodeI64Const), 0x01,
			byte(expr.OpCodeReturn),
		},
	}
	vm := &Instance{
		OperandStack: stacks.NewOperandStack(),
		Context: &wasmContext{
			Func: n,
		},
	}

	assert.NoError(t, vm.execFunc())
	assert.Equal(t, uint64(4), vm.Context.PC)
	assert.Equal(t, uint64(0x01), vm.OperandStack.Pop())
	assert.Equal(t, uint64(0x05), vm.OperandStack.Pop())
}
