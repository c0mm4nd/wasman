package wasm

import (
	"encoding/binary"
	"github.com/c0mm4nd/wasman/config"
)

func memoryBase(ins *Instance) (uint64, error) {
	ins.Context.PC++
	_, err := ins.fetchUint32() // ignore align
	if err != nil {
		return 0, err
	}
	ins.Context.PC++
	v, err := ins.fetchUint32()
	if err != nil {
		return 0, err
	}

	return uint64(v) + ins.OperandStack.Pop(), nil
}

func i32Load(ins *Instance) error {
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(binary.LittleEndian.Uint32(ins.Memory[base:])))

	return nil
}

func i64Load(ins *Instance) error {
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(binary.LittleEndian.Uint64(ins.Memory[base:]))

	return nil
}

func f32Load(ins *Instance) error {
	return i32Load(ins)
}

func f64Load(ins *Instance) error {
	return i64Load(ins)
}

func i32Load8s(ins *Instance) error {
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(ins.Memory[base]))

	return nil
}

func i32Load8u(ins *Instance) error {
	return i32Load8s(ins)
}

func i32Load16s(ins *Instance) error {
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(binary.LittleEndian.Uint16(ins.Memory[base:])))

	return nil
}

func i32Load16u(ins *Instance) error {
	return i32Load16s(ins)
}

func i64Load8s(ins *Instance) error {
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(ins.Memory[base]))

	return nil
}

func i64Load8u(ins *Instance) error {
	return i64Load8s(ins)
}

func i64Load16s(ins *Instance) error {
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(binary.LittleEndian.Uint16(ins.Memory[base:])))

	return nil
}

func i64Load16u(ins *Instance) error {
	return i64Load16s(ins)
}

func i64Load32s(ins *Instance) error {
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(binary.LittleEndian.Uint32(ins.Memory[base:])))

	return nil
}

func i64Load32u(ins *Instance) error {
	return i64Load32s(ins)
}

func i32Store(ins *Instance) error {
	val := ins.OperandStack.Pop()
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint32(ins.Memory[base:], uint32(val))

	return nil
}

func i64Store(ins *Instance) error {
	val := ins.OperandStack.Pop()
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint64(ins.Memory[base:], val)

	return nil
}

func f32Store(ins *Instance) error {
	val := ins.OperandStack.Pop()
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint32(ins.Memory[base:], uint32(val))

	return nil
}

func f64Store(ins *Instance) error {
	v := ins.OperandStack.Pop()
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint64(ins.Memory[base:], v)

	return nil
}

func i32Store8(ins *Instance) error {
	v := byte(ins.OperandStack.Pop())
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	ins.Memory[base] = v

	return nil
}

func i32Store16(ins *Instance) error {
	v := uint16(ins.OperandStack.Pop())
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint16(ins.Memory[base:], v)

	return nil
}

func i64Store8(ins *Instance) error {
	v := byte(ins.OperandStack.Pop())
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	ins.Memory[base] = v

	return nil
}

func i64Store16(ins *Instance) error {
	v := uint16(ins.OperandStack.Pop())
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint16(ins.Memory[base:], v)

	return nil
}

func i64Store32(ins *Instance) error {
	v := uint32(ins.OperandStack.Pop())
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint32(ins.Memory[base:], v)

	return nil
}

func memorySize(ins *Instance) error {
	ins.Context.PC++
	ins.OperandStack.Push(uint64(int32(len(ins.Memory) / config.DefaultPageSize)))

	return nil
}

func memoryGrow(ins *Instance) error {
	ins.Context.PC++
	n := uint32(ins.OperandStack.Pop())

	if ins.Module.MemorySection[0].Max != nil &&
		uint64(n+uint32(len(ins.Memory)/config.DefaultPageSize)) > uint64(*(ins.Module.MemorySection[0].Max)) {
		v := int32(-1)
		ins.OperandStack.Push(uint64(v))

		return nil
	}

	ins.OperandStack.Push(uint64(len(ins.Memory)) / config.DefaultPageSize)
	ins.Memory = append(ins.Memory, make([]byte, n*config.DefaultPageSize)...)

	return nil
}
