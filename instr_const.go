package wasman

import (
	"math"
)

func i32Const(ins *Instance) error {
	ins.Context.PC++

	v, err := ins.fetchInt32()
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(v))

	return nil
}

func i64Const(ins *Instance) error {
	ins.Context.PC++

	v, err := ins.fetchInt64()
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(v))

	return nil
}

func f32Const(ins *Instance) error {
	ins.Context.PC++

	v, err := ins.fetchFloat32()
	if err != nil {
		return err
	}

	ins.OperandStack.Push(uint64(math.Float32bits(v)))

	return nil
}

func f64Const(ins *Instance) error {
	ins.Context.PC++

	v, err := ins.fetchFloat64()
	if err != nil {
		return err
	}

	ins.OperandStack.Push(math.Float64bits(v))

	return nil
}
