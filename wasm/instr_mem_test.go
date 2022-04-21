package wasm

import (
	"bytes"
	"math"
	"testing"

	"github.com/c0mm4nd/wasman/utils"

	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/stacks"
	"github.com/c0mm4nd/wasman/types"
)

func Test_i32Load(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0x01, 0x00, 0x00, 0x00},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	if i32Load(vm) != nil {
		t.Fail()
	}
	if vm.OperandStack.Pop() != 1 {
		t.Fail()
	}
}

func Test_i64Load(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI64Load), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	if i64Load(vm) != nil {
		t.Fail()
	}
	if vm.OperandStack.Pop() != 1 {
		t.Fail()
	}
}

func Test_f32Load(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeF32Load), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			MemoryType: types.MemoryType{},
			Value:      []byte{0x00, 0x01, 0x00, 0x00, 0x00},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	if f32Load(vm) != nil {
		t.Fail()
	}
	if math.Float32frombits(uint32(vm.OperandStack.Pop())) != math.Float32frombits(0x01) {
		t.Fail()
	}
}

func Test_f64Load(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeF64Load), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	if f64Load(vm) != nil {
		t.Fail()
	}
	if math.Float64frombits(vm.OperandStack.Pop()) != math.Float64frombits(0x01) {
		t.Fail()
	}
}

func Test_i32Load8s(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0xff},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	if i32Load8s(vm) != nil {
		t.Fail()
	}
	if int8(vm.OperandStack.Pop()) != int8(-1) {
		t.Fail()
	}
}

func Test_i32Load8u(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0xff},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	if i32Load8u(vm) != nil {
		t.Fail()
	}
	if byte(vm.OperandStack.Pop()) != byte(255) {
		t.Fail()
	}
}

func Test_i32Load16s(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0xff, 0x01},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	if i32Load16s(vm) != nil {
		t.Fail()
	}
	if int16(vm.OperandStack.Pop()) != int16(0x01ff) {
		t.Fail()
	}
}

func Test_i32Load16u(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0x00, 0xff},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	if i32Load16u(vm) != nil {
		t.Fail()
	}
	if uint16(vm.OperandStack.Pop()) != uint16(0xff00) {
		t.Fail()
	}
}

func Test_i64Load8s(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0xff},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	if i64Load8s(vm) != nil {
		t.Fail()
	}
	if int8(vm.OperandStack.Pop()) != int8(-1) {
		t.Fail()
	}
}

func Test_i64Load8u(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0xff},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	if i64Load8u(vm) != nil {
		t.Fail()
	}
	if byte(vm.OperandStack.Pop()) != byte(255) {
		t.Fail()
	}
}

func Test_i64Load16s(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0xff, 0x01},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	if i64Load16s(vm) != nil {
		t.Fail()
	}
	if int16(vm.OperandStack.Pop()) != int16(0x01ff) {
		t.Fail()
	}
}

func Test_i64Load16u(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0x00, 0xff},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	if i64Load16u(vm) != nil {
		t.Fail()
	}
	if uint16(vm.OperandStack.Pop()) != uint16(0xff00) {
		t.Fail()
	}
}

func Test_i64Load32s(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0xff, 0x01, 0x00, 0x01},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	if i64Load32s(vm) != nil {
		t.Fail()
	}
	if int32(vm.OperandStack.Pop()) != int32(0x010001ff) {
		t.Fail()
	}
}

func Test_i64Load32u(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0x00, 0xff, 0x00, 0xff},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	if i64Load32u(vm) != nil {
		t.Fail()
	}
	if uint32(vm.OperandStack.Pop()) != uint32(0xff00ff00) {
		t.Fail()
	}
}

func Test_i32Store(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(uint64(0xffffff11))
	if i32Store(vm) != nil {
		t.Fail()
	}
	if !bytes.Equal(vm.Memory.Value[2:], []byte{0x11, 0xff, 0xff, 0xff}) {
		t.Fail()
	}
}

func Test_i64Store(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(uint64(0xffffff11_22222222))
	if i64Store(vm) != nil {
		t.Fail()
	}
	if !bytes.Equal([]byte{
		0x22, 0x22, 0x22, 0x22,
		0x11, 0xff, 0xff, 0xff,
	},
		vm.Memory.Value[2:],
	) {
		t.Fail()
	}
}

func Test_f32Store(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(uint64(math.Float32bits(math.Float32frombits(0xffff_1111))))
	if f32Store(vm) != nil {
		t.Fail()
	}
	if !bytes.Equal([]byte{0x11, 0x11, 0xff, 0xff}, vm.Memory.Value[2:]) {
		t.Fail()
	}
}

func Test_f64Store(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(math.Float64bits(math.Float64frombits(0xffff_1111_0000_1111)))
	if f64Store(vm) != nil {
		t.Fail()
	}
	if !bytes.Equal([]byte{0x11, 0x11, 0x00, 0x00, 0x11, 0x11, 0xff, 0xff}, vm.Memory.Value[2:]) {
		t.Fail()
	}
}

func Test_i32store8(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0x00, 0x00},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(uint64(byte(111)))
	if i32Store8(vm) != nil {
		t.Fail()
	}
	if vm.Memory.Value[2] != byte(111) {
		t.Fail()
	}
}

func Test_i32store16(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0x00, 0x00, 0x00},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(uint64(uint16(0x11ff)))
	if i32Store16(vm) != nil {
		t.Fail()
	}
	if !bytes.Equal([]byte{0xff, 0x11}, vm.Memory.Value[2:]) {
		t.Fail()
	}
}

func Test_i64store8(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0x00, 0x00},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(uint64(byte(111)))
	if i64Store8(vm) != nil {
		t.Fail()
	}
	if vm.Memory.Value[2] != byte(111) {
		t.Fail()
	}
}

func Test_i64store16(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0x00, 0x00, 0x00},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(uint64(uint16(0x11ff)))
	if i64Store16(vm) != nil {
		t.Fail()
	}
	if !bytes.Equal([]byte{0xff, 0x11}, vm.Memory.Value[2:]) {
		t.Fail()
	}
}

func Test_i64store32(t *testing.T) {
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{
				body: []byte{byte(expr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory: &Memory{
			Value: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(uint64(uint32(0x11ff_22ee)))
	if i64Store32(vm) != nil {
		t.Fail()
	}
	if !bytes.Equal([]byte{0xee, 0x22, 0xff, 0x11}, vm.Memory.Value[2:]) {
		t.Fail()
	}
}

func Test_memorySize(t *testing.T) {
	vm := &Instance{
		Active: &Frame{},
		Memory: &Memory{
			Value: make([]byte, config.DefaultMemoryPageSize*2),
		},
		OperandStack: stacks.NewOperandStack(),
	}

	if memorySize(vm) != nil {
		t.Fail()
	}
	if vm.OperandStack.Pop() != uint64(0x2) {
		t.Fail()
	}
}

func Test_memoryGrow(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		vm := &Instance{
			Active: &Frame{},
			Memory: &Memory{
				Value: make([]byte, config.DefaultMemoryPageSize*2),
			},
			OperandStack: stacks.NewOperandStack(),
			Module: &Module{
				MemorySection: []*types.MemoryType{{}},
			},
		}

		vm.OperandStack.Push(5)
		if memoryGrow(vm) != nil {
			t.Fail()
		}
		if vm.OperandStack.Pop() != uint64(0x2) {
			t.Fail()
		}
		if len(vm.Memory.Value)/config.DefaultMemoryPageSize != 7 {
			t.Fail()
		}
	})

	t.Run("oom", func(t *testing.T) {
		vm := &Instance{
			Active: &Frame{},
			Memory: &Memory{
				Value: make([]byte, config.DefaultMemoryPageSize*2),
			},
			OperandStack: stacks.NewOperandStack(),
			Module: &Module{
				MemorySection: []*types.MemoryType{{Max: utils.Uint32Ptr(0)}},
			},
		}

		exp := int32(-1)
		vm.OperandStack.Push(5)
		err := memoryGrow(vm)
		if err != nil {
			t.Fail()
		}
		if vm.OperandStack.Pop() != uint64(exp) {
			t.Fail()
		}
	})

}
