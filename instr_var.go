package wasman

func getLocal(ins *Instance) error {
	ins.Context.PC++
	id, err := ins.fetchUint32()
	if err != nil {
		return err
	}

	ins.OperandStack.push(ins.Context.Locals[id])

	return nil
}

func setLocal(ins *Instance) error {
	ins.Context.PC++
	id, err := ins.fetchUint32()
	if err != nil {
		return err
	}

	v := ins.OperandStack.pop()
	ins.Context.Locals[id] = v

	return nil
}

func teeLocal(ins *Instance) error {
	ins.Context.PC++
	id, err := ins.fetchUint32()
	if err != nil {
		return err
	}

	v := ins.OperandStack.peek()
	ins.Context.Locals[id] = v

	return nil
}

func getGlobal(ins *Instance) error {
	ins.Context.PC++
	id, err := ins.fetchUint32()
	if err != nil {
		return err
	}

	ins.OperandStack.push(ins.Globals[id])

	return nil
}

func setGlobal(ins *Instance) error {
	ins.Context.PC++
	id, err := ins.fetchUint32()
	if err != nil {
		return err
	}

	ins.Globals[id] = ins.OperandStack.pop()

	return nil
}
