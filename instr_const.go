package wasman

import (
	"math"
)


func i32Const(ins *Instance) {
	ins.Context.PC++
	ins.OperandStack.push(uint64(ins.fetchInt32()))
}

func i64Const(ins *Instance) {
	ins.Context.PC++
	ins.OperandStack.push(uint64(ins.fetchInt64()))
}

func f32Const(ins *Instance) {
	ins.Context.PC++
	ins.OperandStack.push(uint64(math.Float32bits(ins.fetchFloat32())))
}

func f64Const(ins *Instance) {
	ins.Context.PC++
	ins.OperandStack.push(math.Float64bits(ins.fetchFloat64()))
}