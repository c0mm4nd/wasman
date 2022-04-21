package wasm

import (
	"math"
	"testing"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/stacks"
)

func Test_i32Const(t *testing.T) {
	ctx := &Frame{
		Func: &wasmFunc{
			body: []byte{byte(expr.OpCodeI32Const), 0x05},
		},
	}

	vm := &Instance{
		Active:      ctx,
		OperandStack: stacks.NewOperandStack(),
	}
	err := i32Const(vm)
	if err != nil {
		t.Fail()
	}
	if uint32(vm.OperandStack.Pop()) != 0x05 {
		t.Fail()
	}
	if vm.OperandStack.Ptr != -1 {
		t.Fail()
	}
}

func Test_i64Const(t *testing.T) {
	ctx := &Frame{
		Func: &wasmFunc{
			body: []byte{byte(expr.OpCodeI64Const), 0x05},
		},
	}

	vm := &Instance{
		Active:      ctx,
		OperandStack: stacks.NewOperandStack(),
	}
	err := i64Const(vm)
	if err != nil {
		t.Fail()
	}
	if vm.OperandStack.Pop() != 0x05 {
		t.Fail()
	}
	if vm.OperandStack.Ptr != -1 {
		t.Fail()
	}
}

func Test_f32Const(t *testing.T) {

	ctx := &Frame{
		Func: &wasmFunc{
			body: []byte{byte(expr.OpCodeF32Const), 0x00, 0x00, 0x80, 0x3f},
		},
	}

	vm := &Instance{
		Active:      ctx,
		OperandStack: stacks.NewOperandStack(),
	}
	err := f32Const(vm)
	if err != nil {
		t.Fail()
	}
	if math.Float32frombits(uint32(vm.OperandStack.Pop())) != 1.0 {
		t.Fail()
	}
	if vm.OperandStack.Ptr != -1 {
		t.Fail()
	}
}

func Test_f64Const(t *testing.T) {
	ctx := &Frame{
		Func: &wasmFunc{
			body: []byte{byte(expr.OpCodeF64Const), 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0, 0x3f},
		},
	}

	vm := &Instance{
		Active:      ctx,
		OperandStack: stacks.NewOperandStack(),
	}
	err := f64Const(vm)
	if err != nil {
		t.Fail()
	}
	if math.Float64frombits(vm.OperandStack.Pop()) != 1.0 {
		t.Fail()
	}
	if vm.OperandStack.Ptr != -1 {
		t.Fail()
	}
}
