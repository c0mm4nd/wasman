package wasm

import (
	"encoding/binary"
	"errors"

	"github.com/c0mm4nd/wasman/config"
)

// ErrPtrOutOfBounds will be throw when the pointer visiting a pos out of the range of memory
var ErrPtrOutOfBounds = errors.New("pointer is out of bounds")

// memoryBase decodes a load/store's align+offset immediates and returns the
// effective address, bounds-checked for an access of the given width so that
// partially out-of-range accesses trap instead of panicking.
func memoryBase(ins *Instance, width uint64) (uint64, error) {
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

	// the address operand is an i32 interpreted as unsigned; masking to 32 bits
	// also makes the base+width comparison below overflow-free.
	base := uint64(v) + uint64(uint32(ins.OperandStack.Pop()))
	if base+width > uint64(len(ins.Memory.Value)) {
		return 0, ErrPtrOutOfBounds
	}

	return base, nil
}

func i32Load(ins *Instance) error {
	base, err := memoryBase(ins, 4)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(binary.LittleEndian.Uint32(ins.Memory.Value[base:])))

	return nil
}

func i64Load(ins *Instance) error {
	base, err := memoryBase(ins, 8)
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
	base, err := memoryBase(ins, 1)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(uint32(int32(int8(ins.Memory.Value[base])))))

	return nil
}

func i32Load8u(ins *Instance) error {
	base, err := memoryBase(ins, 1)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(ins.Memory.Value[base]))

	return nil
}

func i32Load16s(ins *Instance) error {
	base, err := memoryBase(ins, 2)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(uint32(int32(int16(binary.LittleEndian.Uint16(ins.Memory.Value[base:]))))))

	return nil
}

func i32Load16u(ins *Instance) error {
	base, err := memoryBase(ins, 2)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(binary.LittleEndian.Uint16(ins.Memory.Value[base:])))

	return nil
}

func i64Load8s(ins *Instance) error {
	base, err := memoryBase(ins, 1)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(int64(int8(ins.Memory.Value[base]))))

	return nil
}

func i64Load8u(ins *Instance) error {
	base, err := memoryBase(ins, 1)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(ins.Memory.Value[base]))

	return nil
}

func i64Load16s(ins *Instance) error {
	base, err := memoryBase(ins, 2)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(int64(int16(binary.LittleEndian.Uint16(ins.Memory.Value[base:])))))

	return nil
}

func i64Load16u(ins *Instance) error {
	base, err := memoryBase(ins, 2)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(binary.LittleEndian.Uint16(ins.Memory.Value[base:])))

	return nil
}

func i64Load32s(ins *Instance) error {
	base, err := memoryBase(ins, 4)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(int64(int32(binary.LittleEndian.Uint32(ins.Memory.Value[base:])))))

	return nil
}

func i64Load32u(ins *Instance) error {
	base, err := memoryBase(ins, 4)
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(binary.LittleEndian.Uint32(ins.Memory.Value[base:])))

	return nil
}

func i32Store(ins *Instance) error {
	val := ins.OperandStack.Pop()
	base, err := memoryBase(ins, 4)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint32(ins.Memory.Value[base:], uint32(val))

	return nil
}

func i64Store(ins *Instance) error {
	val := ins.OperandStack.Pop()
	base, err := memoryBase(ins, 8)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint64(ins.Memory.Value[base:], val)

	return nil
}

func f32Store(ins *Instance) error {
	val := ins.OperandStack.Pop()
	base, err := memoryBase(ins, 4)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint32(ins.Memory.Value[base:], uint32(val))

	return nil
}

func f64Store(ins *Instance) error {
	v := ins.OperandStack.Pop()
	base, err := memoryBase(ins, 8)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint64(ins.Memory.Value[base:], v)

	return nil
}

func i32Store8(ins *Instance) error {
	v := byte(ins.OperandStack.Pop())
	base, err := memoryBase(ins, 1)
	if err != nil {
		return err
	}

	ins.Memory.Value[base] = v

	return nil
}

func i32Store16(ins *Instance) error {
	v := uint16(ins.OperandStack.Pop())
	base, err := memoryBase(ins, 2)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint16(ins.Memory.Value[base:], v)

	return nil
}

func i64Store8(ins *Instance) error {
	v := byte(ins.OperandStack.Pop())
	base, err := memoryBase(ins, 1)
	if err != nil {
		return err
	}

	ins.Memory.Value[base] = v

	return nil
}

func i64Store16(ins *Instance) error {
	v := uint16(ins.OperandStack.Pop())
	base, err := memoryBase(ins, 2)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint16(ins.Memory.Value[base:], v)

	return nil
}

func i64Store32(ins *Instance) error {
	v := uint32(ins.OperandStack.Pop())
	base, err := memoryBase(ins, 4)
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
	n := uint64(uint32(ins.OperandStack.Pop()))

	current := uint64(len(ins.Memory.Value)) / config.DefaultMemoryPageSize

	// the ceiling is the declared max, otherwise the architectural limit of
	// 65536 pages (a wasm32 linear memory can be at most 4 GiB).
	max := uint64(config.DefaultMemoryMaxPages)
	if ins.Memory.Max != nil && uint64(*ins.Memory.Max) < max {
		max = uint64(*ins.Memory.Max)
	}

	// uint64 math avoids the uint32 overflow that would let a huge n wrap
	// under the limit; a failed grow returns -1 without allocating.
	if current+n > max {
		failed := int32(-1)
		ins.OperandStack.Push(uint64(failed))

		return nil
	}

	ins.OperandStack.Push(current)
	ins.Memory.Value = append(ins.Memory.Value, make([]byte, n*config.DefaultMemoryPageSize)...)

	return nil
}
