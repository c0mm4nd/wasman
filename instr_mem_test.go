package wasman

import (
	"github.com/c0mm4nd/wasman/instr"
	"github.com/c0mm4nd/wasman/stacks"
	"github.com/c0mm4nd/wasman/types"
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
)

func Test_i32Load(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0x01, 0x00, 0x00, 0x00},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	assert.NoError(t, i32Load(vm))
	assert.Equal(t, uint32(1), uint32(vm.OperandStack.Pop()))
}

func Test_i64Load(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI64Load), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	assert.NoError(t, i64Load(vm))
	assert.Equal(t, uint64(1), vm.OperandStack.Pop())
}

func Test_f32Load(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0x01, 0x00, 0x00, 0x00},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	assert.NoError(t, f32Load(vm))
	assert.Equal(t, math.Float32frombits(0x01),
		math.Float32frombits(uint32(vm.OperandStack.Pop())))
}

func Test_f64Load(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0x00, 0x01, 0x00, 0x00, 0x00},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	assert.NoError(t, f32Load(vm))
	assert.Equal(t, math.Float64frombits(0x01),
		math.Float64frombits(vm.OperandStack.Pop()))
}

func Test_i32Load8s(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0xff},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	assert.NoError(t, i32Load8s(vm))
	assert.Equal(t, int8(-1), int8(vm.OperandStack.Pop()))
}

func Test_i32Load8u(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0xff},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	assert.NoError(t, i32Load8u(vm))
	assert.Equal(t, byte(255), byte(vm.OperandStack.Pop()))
}

func Test_i32Load16s(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0xff, 0x01},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	assert.NoError(t, i32Load16s(vm))
	assert.Equal(t, int16(0x01ff), int16(vm.OperandStack.Pop()))
}

func Test_i32Load16u(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0x00, 0xff},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	assert.NoError(t, i32Load16u(vm))
	assert.Equal(t, uint16(0xff00), uint16(vm.OperandStack.Pop()))
}

func Test_i64Load8s(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0xff},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	assert.NoError(t, i64Load8s(vm))
	assert.Equal(t, int8(-1), int8(vm.OperandStack.Pop()))
}

func Test_i64Load8u(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0xff},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	assert.NoError(t, i64Load8u(vm))
	assert.Equal(t, byte(255), byte(vm.OperandStack.Pop()))
}

func Test_i64Load16s(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0xff, 0x01},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	assert.NoError(t, i64Load16s(vm))
	assert.Equal(t, int16(0x01ff), int16(vm.OperandStack.Pop()))
}

func Test_i64Load16u(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0x00, 0xff},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	assert.NoError(t, i64Load16u(vm))
	assert.Equal(t, uint16(0xff00), uint16(vm.OperandStack.Pop()))
}

func Test_i64Load32s(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0xff, 0x01, 0x00, 0x01},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	assert.NoError(t, i64Load32s(vm))
	assert.Equal(t, int32(0x010001ff), int32(vm.OperandStack.Pop()))
}

func Test_i64Load32u(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Load), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0x00, 0xff, 0x00, 0xff},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(0))
	assert.NoError(t, i64Load32u(vm))
	assert.Equal(t, uint32(0xff00ff00), uint32(vm.OperandStack.Pop()))
}

func Test_i32Store(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(uint64(0xffffff11))
	assert.NoError(t, i32Store(vm))
	assert.Equal(t, []byte{0x11, 0xff, 0xff, 0xff}, vm.Memory[2:])
}

func Test_i64Store(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(uint64(0xffffff11_22222222))
	assert.NoError(t, i64Store(vm))
	assert.Equal(t,
		[]byte{
			0x22, 0x22, 0x22, 0x22,
			0x11, 0xff, 0xff, 0xff,
		},
		vm.Memory[2:],
	)
}

func Test_f32Store(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(uint64(math.Float32bits(math.Float32frombits(0xffff_1111))))
	assert.NoError(t, f32Store(vm))
	assert.Equal(t, []byte{0x11, 0x11, 0xff, 0xff}, vm.Memory[2:])
}

func Test_f64Store(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(math.Float64bits(math.Float64frombits(0xffff_1111_0000_1111)))
	assert.NoError(t, f64Store(vm))
	assert.Equal(t, []byte{0x11, 0x11, 0x00, 0x00, 0x11, 0x11, 0xff, 0xff}, vm.Memory[2:])
}

func Test_i32store8(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0x00, 0x00},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(uint64(byte(111)))
	assert.NoError(t, i32Store8(vm))
	assert.Equal(t, byte(111), vm.Memory[2])
}

func Test_i32store16(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0x00, 0x00, 0x00},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(uint64(uint16(0x11ff)))
	assert.NoError(t, i32Store16(vm))
	assert.Equal(t, []byte{0xff, 0x11}, vm.Memory[2:])
}

func Test_i64store8(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0x00, 0x00},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(uint64(byte(111)))
	assert.NoError(t, i64Store8(vm))
	assert.Equal(t, byte(111), vm.Memory[2])
}

func Test_i64store16(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0x00, 0x00, 0x00},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(uint64(uint16(0x11ff)))
	assert.NoError(t, i64Store16(vm))
	assert.Equal(t, []byte{0xff, 0x11}, vm.Memory[2:])
}

func Test_i64store32(t *testing.T) {
	vm := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				body: []byte{byte(instr.OpCodeI32Store), 0x00, 0x01},
			},
		},
		Memory:       []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		OperandStack: stacks.NewOperandStack(),
	}

	vm.OperandStack.Push(uint64(1))
	vm.OperandStack.Push(uint64(uint32(0x11ff_22ee)))
	assert.NoError(t, i64Store32(vm))
	assert.Equal(t, []byte{0xee, 0x22, 0xff, 0x11}, vm.Memory[2:])
}

func Test_memorySize(t *testing.T) {
	vm := &Instance{
		Context:      &wasmContext{},
		Memory:       make([]byte, defaultPageSize*2),
		OperandStack: stacks.NewOperandStack(),
	}

	assert.NoError(t, memorySize(vm))
	assert.Equal(t, uint64(0x2), vm.OperandStack.Pop())
}

func Test_memoryGrow(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		vm := &Instance{
			Context:      &wasmContext{},
			Memory:       make([]byte, defaultPageSize*2),
			OperandStack: stacks.NewOperandStack(),
			Module: &Module{
				MemorySection: []*types.MemoryType{{}},
			},
		}

		vm.OperandStack.Push(5)
		assert.NoError(t, memoryGrow(vm))
		assert.Equal(t, uint64(0x2), vm.OperandStack.Pop())
		assert.Equal(t, 7, len(vm.Memory)/defaultPageSize)
	})

	t.Run("oom", func(t *testing.T) {
		vm := &Instance{
			Context:      &wasmContext{},
			Memory:       make([]byte, defaultPageSize*2),
			OperandStack: stacks.NewOperandStack(),
			Module: &Module{
				MemorySection: []*types.MemoryType{{Max: uint32Ptr(0)}},
			},
		}

		exp := int32(-1)
		vm.OperandStack.Push(5)
		assert.NoError(t, memoryGrow(vm))
		assert.Equal(t, uint64(exp), vm.OperandStack.Pop())
	})

}
