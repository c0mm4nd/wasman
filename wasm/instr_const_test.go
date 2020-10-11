package wasm

import (
	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/stacks"
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
)

func Test_i32Const(t *testing.T) {
	ctx := &wasmContext{
		Func: &wasmFunc{
			body: []byte{byte(expr.OpCodeI32Const), 0x05},
		},
	}

	vm := &Instance{
		Context:      ctx,
		OperandStack: stacks.NewOperandStack(),
	}
	assert.NoError(t, i32Const(vm))
	assert.Equal(t, uint32(0x05), uint32(vm.OperandStack.Pop()))
	assert.Equal(t, -1, vm.OperandStack.Ptr)
}

func Test_i64Const(t *testing.T) {
	ctx := &wasmContext{
		Func: &wasmFunc{
			body: []byte{byte(expr.OpCodeI64Const), 0x05},
		},
	}

	vm := &Instance{
		Context:      ctx,
		OperandStack: stacks.NewOperandStack(),
	}
	assert.NoError(t, i64Const(vm))
	assert.Equal(t, uint32(0x05), uint32(vm.OperandStack.Pop()))
	assert.Equal(t, -1, vm.OperandStack.Ptr)
}

func Test_f32Const(t *testing.T) {

	ctx := &wasmContext{
		Func: &wasmFunc{
			body: []byte{byte(expr.OpCodeF32Const), 0x00, 0x00, 0x80, 0x3f},
		},
	}

	vm := &Instance{
		Context:      ctx,
		OperandStack: stacks.NewOperandStack(),
	}
	assert.NoError(t, f32Const(vm))
	assert.Equal(t, float32(1.0), math.Float32frombits(uint32(vm.OperandStack.Pop())))
	assert.Equal(t, -1, vm.OperandStack.Ptr)
}

func Test_f64Const(t *testing.T) {
	ctx := &wasmContext{
		Func: &wasmFunc{
			body: []byte{byte(expr.OpCodeF64Const), 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0, 0x3f},
		},
	}

	vm := &Instance{
		Context:      ctx,
		OperandStack: stacks.NewOperandStack(),
	}
	assert.NoError(t, f64Const(vm))
	assert.Equal(t, 1.0, math.Float64frombits(vm.OperandStack.Pop()))
	assert.Equal(t, -1, vm.OperandStack.Ptr)
}
