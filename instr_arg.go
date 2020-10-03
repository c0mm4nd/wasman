package wasman

func drop(ins *Instance) error {
	ins.OperandStack.Drop()

	return nil
}

func selectOp(ins *Instance) error {
	c := ins.OperandStack.Pop()
	v2 := ins.OperandStack.Pop()
	if c == 0 {
		_ = ins.OperandStack.Pop()
		ins.OperandStack.Push(v2)
	}

	return nil
}
