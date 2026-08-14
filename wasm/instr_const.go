package wasm

import (
	"math"
)

func i32Const(ins *Instance) error {
	if f := ins.Active.Func; f.imms != nil {
		p := ins.Active.PC
		ins.OperandStack.Push(f.imms[p])
		ins.Active.PC = uint64(f.pcEnd[p])
		return nil
	}
	ins.Active.PC++
	v, err := ins.fetchInt32()
	if err != nil {
		return err
	}
	ins.OperandStack.Push(uint64(v))
	return nil
}

func i64Const(ins *Instance) error {
	if f := ins.Active.Func; f.imms != nil {
		p := ins.Active.PC
		ins.OperandStack.Push(f.imms[p])
		ins.Active.PC = uint64(f.pcEnd[p])
		return nil
	}
	ins.Active.PC++
	v, err := ins.fetchInt64()
	if err != nil {
		return err
	}
	ins.OperandStack.Push(uint64(v))
	return nil
}

func f32Const(ins *Instance) error {
	if f := ins.Active.Func; f.imms != nil {
		p := ins.Active.PC
		ins.OperandStack.Push(f.imms[p])
		ins.Active.PC = uint64(f.pcEnd[p])
		return nil
	}
	ins.Active.PC++
	v, err := ins.fetchFloat32()
	if err != nil {
		return err
	}
	ins.OperandStack.Push(uint64(math.Float32bits(v)))
	return nil
}

func f64Const(ins *Instance) error {
	if f := ins.Active.Func; f.imms != nil {
		p := ins.Active.PC
		ins.OperandStack.Push(f.imms[p])
		ins.Active.PC = uint64(f.pcEnd[p])
		return nil
	}
	ins.Active.PC++
	v, err := ins.fetchFloat64()
	if err != nil {
		return err
	}
	ins.OperandStack.Push(math.Float64bits(v))
	return nil
}
