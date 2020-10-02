package wasman

func drop(ins *Instance) error {
	ins.OperandStack.drop()

	return nil
}

func selectOp(ins *Instance) error {
	c := ins.OperandStack.pop()
	v2 := ins.OperandStack.pop()
	if c == 0 {
		_ = ins.OperandStack.pop()
		ins.OperandStack.push(v2)
	}

	return nil
}
