package wasman

import (
	"encoding/binary"
)

func memoryBase(ins *Instance) uint64 {
	ins.Context.PC++
	_ = ins.fetchUint32() // ignore align
	ins.Context.PC++
	return uint64(ins.fetchUint32()) + ins.OperandStack.pop()
}

func i32Load(ins *Instance) {
	base := memoryBase(ins)
	ins.OperandStack.push(uint64(binary.LittleEndian.Uint32(ins.Memory[base:])))
}

func i64Load(ins *Instance) {
	base := memoryBase(ins)
	ins.OperandStack.push(binary.LittleEndian.Uint64(ins.Memory[base:]))
}

func f32Load(ins *Instance) {
	i32Load(ins)
}

func f64Load(ins *Instance) {
	i64Load(ins)
}

func i32Load8s(ins *Instance) {
	base := memoryBase(ins)
	ins.OperandStack.push(uint64(ins.Memory[base]))
}

func i32Load8u(ins *Instance) {
	i32Load8s(ins)
}

func i32Load16s(ins *Instance) {
	base := memoryBase(ins)
	ins.OperandStack.push(uint64(binary.LittleEndian.Uint16(ins.Memory[base:])))
}

func i32Load16u(ins *Instance) {
	i32Load16s(ins)
}

func i64Load8s(ins *Instance) {
	base := memoryBase(ins)
	ins.OperandStack.push(uint64(ins.Memory[base]))
}

func i64Load8u(ins *Instance) {
	i64Load8s(ins)
}

func i64Load16s(ins *Instance) {
	base := memoryBase(ins)
	ins.OperandStack.push(uint64(binary.LittleEndian.Uint16(ins.Memory[base:])))
}

func i64Load16u(ins *Instance) {
	i64Load16s(ins)
}

func i64Load32s(ins *Instance) {
	base := memoryBase(ins)
	ins.OperandStack.push(uint64(binary.LittleEndian.Uint32(ins.Memory[base:])))
}

func i64Load32u(ins *Instance) {
	i64Load32s(ins)
}

func i32Store(ins *Instance) {
	val := ins.OperandStack.pop()
	base := memoryBase(ins)
	binary.LittleEndian.PutUint32(ins.Memory[base:], uint32(val))
}

func i64Store(ins *Instance) {
	val := ins.OperandStack.pop()
	base := memoryBase(ins)
	binary.LittleEndian.PutUint64(ins.Memory[base:], val)
}

func f32Store(ins *Instance) {
	val := ins.OperandStack.pop()
	base := memoryBase(ins)
	binary.LittleEndian.PutUint32(ins.Memory[base:], uint32(val))
}

func f64Store(ins *Instance) {
	v := ins.OperandStack.pop()
	base := memoryBase(ins)
	binary.LittleEndian.PutUint64(ins.Memory[base:], v)
}

func i32Store8(ins *Instance) {
	v := byte(ins.OperandStack.pop())
	base := memoryBase(ins)
	ins.Memory[base] = v
}

func i32Store16(ins *Instance) {
	v := uint16(ins.OperandStack.pop())
	base := memoryBase(ins)
	binary.LittleEndian.PutUint16(ins.Memory[base:], v)
}

func i64Store8(ins *Instance) {
	v := byte(ins.OperandStack.pop())
	base := memoryBase(ins)
	ins.Memory[base] = v
}

func i64Store16(ins *Instance) {
	v := uint16(ins.OperandStack.pop())
	base := memoryBase(ins)
	binary.LittleEndian.PutUint16(ins.Memory[base:], v)
}

func i64Store32(ins *Instance) {
	v := uint32(ins.OperandStack.pop())
	base := memoryBase(ins)
	binary.LittleEndian.PutUint32(ins.Memory[base:], v)
}

func memorySize(ins *Instance) {
	ins.Context.PC++
	ins.OperandStack.push(uint64(int32(len(ins.Memory) / defaultPageSize)))
}

func memoryGrow(ins *Instance) {
	ins.Context.PC++
	n := uint32(ins.OperandStack.pop())

	if ins.Module.MemorySection[0].Max != nil &&
		uint64(n+uint32(len(ins.Memory)/defaultPageSize)) > uint64(*(ins.Module.MemorySection[0].Max)) {
		v := int32(-1)
		ins.OperandStack.push(uint64(v))
		return
	}

	ins.OperandStack.push(uint64(len(ins.Memory)) / defaultPageSize)
	ins.Memory = append(ins.Memory, make([]byte, n*defaultPageSize)...)
}