package wasm

import (
	"testing"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/stacks"
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
	err := getLocal(vm)
	if err != nil {
		t.Fail()
	}
	if vm.OperandStack.Pop() != exp {
		t.Fail()
	}
	if vm.OperandStack.Ptr != -1 {
		t.Fail()
	}
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
	err := setLocal(vm)
	if err != nil {
		t.Fail()
	}
	if vm.Context.Locals[5] != exp {
		t.Fail()
	}
	if vm.OperandStack.Ptr != -1 {
		t.Fail()
	}
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
	err := teeLocal(vm)
	if err != nil {
		t.Fail()
	}
	if vm.Context.Locals[5] != exp {
		t.Fail()
	}
	if vm.OperandStack.Pop() != exp {
		t.Fail()
	}
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
	err := getGlobal(vm)
	if err != nil {
		t.Fail()
	}
	if vm.OperandStack.Pop() != exp {
		t.Fail()
	}
	if vm.OperandStack.Ptr != -1 {
		t.Fail()
	}
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
	err := setGlobal(vm)
	if err != nil {
		t.Fail()
	}
	if vm.Globals[5] != exp {
		t.Fail()
	}
	if vm.OperandStack.Ptr != -1 {
		t.Fail()
	}
}
