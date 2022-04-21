package wasm

import (
	"encoding/binary"
	"errors"

	"github.com/c0mm4nd/wasman/config"
)

// ErrPtrOutOfBounds will be throw when the pointer visiting a pos out of the range of memory
var ErrPtrOutOfBounds = errors.New("pointer is out of bounds")

func memoryBase(ins *Instance) (uint64, error) {
	ins.Active.PC++
	_, err := ins.fetchUint32() // ignore align
	if err != nil {
		return 0, err
	}
	ins.Active.PC++
	v, err := ins.fetchUint32()
	if err != nil {
		return 0, err
	}

	base := uint64(v) + ins.OperandStack.Pop()
	if !(base < uint64(len(ins.Memory.Value))) {
		return 0, ErrPtrOutOfBounds
	}

	return base, nil
}

func i32Load(ins *Instance) error {
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(binary.LittleEndian.Uint32(ins.Memory.Value[base:])))

	return nil
}

func i64Load(ins *Instance) error {
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(binary.LittleEndian.Uint64(ins.Memory.Value[base:]))

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

	ins.OperandStack.Push(uint64(ins.Memory.Value[base]))

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

	ins.OperandStack.Push(uint64(binary.LittleEndian.Uint16(ins.Memory.Value[base:])))

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

	ins.OperandStack.Push(uint64(ins.Memory.Value[base]))

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

	ins.OperandStack.Push(uint64(binary.LittleEndian.Uint16(ins.Memory.Value[base:])))

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

	ins.OperandStack.Push(uint64(binary.LittleEndian.Uint32(ins.Memory.Value[base:])))

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

	binary.LittleEndian.PutUint32(ins.Memory.Value[base:], uint32(val))

	return nil
}

func i64Store(ins *Instance) error {
	val := ins.OperandStack.Pop()
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint64(ins.Memory.Value[base:], val)

	return nil
}

func f32Store(ins *Instance) error {
	val := ins.OperandStack.Pop()
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint32(ins.Memory.Value[base:], uint32(val))

	return nil
}

func f64Store(ins *Instance) error {
	v := ins.OperandStack.Pop()
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint64(ins.Memory.Value[base:], v)

	return nil
}

func i32Store8(ins *Instance) error {
	v := byte(ins.OperandStack.Pop())
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	ins.Memory.Value[base] = v

	return nil
}

func i32Store16(ins *Instance) error {
	v := uint16(ins.OperandStack.Pop())
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint16(ins.Memory.Value[base:], v)

	return nil
}

func i64Store8(ins *Instance) error {
	v := byte(ins.OperandStack.Pop())
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	ins.Memory.Value[base] = v

	return nil
}

func i64Store16(ins *Instance) error {
	v := uint16(ins.OperandStack.Pop())
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint16(ins.Memory.Value[base:], v)

	return nil
}

func i64Store32(ins *Instance) error {
	v := uint32(ins.OperandStack.Pop())
	base, err := memoryBase(ins)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint32(ins.Memory.Value[base:], v)

	return nil
}

func memorySize(ins *Instance) error {
	ins.Active.PC++
	ins.OperandStack.Push(uint64(int32(len(ins.Memory.Value) / config.DefaultMemoryPageSize)))

	return nil
}

func memoryGrow(ins *Instance) error {
	ins.Active.PC++
	n := uint32(ins.OperandStack.Pop())

	if ins.Module.MemorySection[0].Max != nil &&
		uint64(n+uint32(len(ins.Memory.Value)/config.DefaultMemoryPageSize)) > uint64(*(ins.Module.MemorySection[0].Max)) {
		v := int32(-1)
		ins.OperandStack.Push(uint64(v))

		return nil
	}

	ins.OperandStack.Push(uint64(len(ins.Memory.Value)) / config.DefaultMemoryPageSize)
	ins.Memory.Value = append(ins.Memory.Value, make([]byte, n*config.DefaultMemoryPageSize)...)

	return nil
}
