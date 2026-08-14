package wasm

// Handlers read pre-decoded immediates (f.imms/f.pcEnd, filled at load time);
// the fetch fallback only serves hand-built test fixtures without tables.

func getLocal(ins *Instance) error {
	if f := ins.Active.Func; f.imms != nil {
		p := ins.Active.PC
		ins.OperandStack.Push(ins.Active.Locals[f.imms[p]])
		ins.Active.PC = uint64(f.pcEnd[p])
		return nil
	}
	ins.Active.PC++
	id, err := ins.fetchUint32()
	if err != nil {
		return err
	}
	ins.OperandStack.Push(ins.Active.Locals[id])
	return nil
}

func setLocal(ins *Instance) error {
	if f := ins.Active.Func; f.imms != nil {
		p := ins.Active.PC
		ins.Active.Locals[f.imms[p]] = ins.OperandStack.Pop()
		ins.Active.PC = uint64(f.pcEnd[p])
		return nil
	}
	ins.Active.PC++
	id, err := ins.fetchUint32()
	if err != nil {
		return err
	}
	ins.Active.Locals[id] = ins.OperandStack.Pop()
	return nil
}

func teeLocal(ins *Instance) error {
	if f := ins.Active.Func; f.imms != nil {
		p := ins.Active.PC
		ins.Active.Locals[f.imms[p]] = ins.OperandStack.Peek()
		ins.Active.PC = uint64(f.pcEnd[p])
		return nil
	}
	ins.Active.PC++
	id, err := ins.fetchUint32()
	if err != nil {
		return err
	}
	ins.Active.Locals[id] = ins.OperandStack.Peek()
	return nil
}

func getGlobal(ins *Instance) error {
	if f := ins.Active.Func; f.imms != nil {
		p := ins.Active.PC
		ins.OperandStack.Push(*ins.Globals[f.imms[p]])
		ins.Active.PC = uint64(f.pcEnd[p])
		return nil
	}
	ins.Active.PC++
	id, err := ins.fetchUint32()
	if err != nil {
		return err
	}
	ins.OperandStack.Push(*ins.Globals[id])
	return nil
}

func setGlobal(ins *Instance) error {
	if f := ins.Active.Func; f.imms != nil {
		p := ins.Active.PC
		*ins.Globals[f.imms[p]] = ins.OperandStack.Pop()
		ins.Active.PC = uint64(f.pcEnd[p])
		return nil
	}
	ins.Active.PC++
	id, err := ins.fetchUint32()
	if err != nil {
		return err
	}
	*ins.Globals[id] = ins.OperandStack.Pop()
	return nil
}
