package wasm

func getLocal(ins *Instance) error {
	ins.Active.PC++
	id, err := ins.fetchUint32()
	if err != nil {
		return err
	}

	ins.OperandStack.Push(ins.Active.Locals[id])

	return nil
}

func setLocal(ins *Instance) error {
	ins.Active.PC++
	id, err := ins.fetchUint32()
	if err != nil {
		return err
	}

	v := ins.OperandStack.Pop()
	ins.Active.Locals[id] = v

	return nil
}

func teeLocal(ins *Instance) error {
	ins.Active.PC++
	id, err := ins.fetchUint32()
	if err != nil {
		return err
	}

	v := ins.OperandStack.Peek()
	ins.Active.Locals[id] = v

	return nil
}

func getGlobal(ins *Instance) error {
	ins.Active.PC++
	id, err := ins.fetchUint32()
	if err != nil {
		return err
	}

	ins.OperandStack.Push(ins.Globals[id])

	return nil
}

func setGlobal(ins *Instance) error {
	ins.Active.PC++
	id, err := ins.fetchUint32()
	if err != nil {
		return err
	}

	ins.Globals[id] = ins.OperandStack.Pop()

	return nil
}
