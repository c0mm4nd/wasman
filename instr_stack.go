package wasman

func drop(ins *Instance) {
	ins.OperandStack.drop()
}

func selectOp(ins *Instance) {
	c := ins.OperandStack.pop()
	v2 := ins.OperandStack.pop()
	if c == 0 {
		_ = ins.OperandStack.pop()
		ins.OperandStack.push(v2)
	}
}
