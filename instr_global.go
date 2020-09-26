package wasman

func getGlobal(ins *Instance) {
	ins.Context.PC++
	id := ins.fetchUint32()
	ins.OperandStack.push(ins.Globals[id])
}

func setGlobal(ins *Instance) {
	ins.Context.PC++
	id := ins.fetchUint32()
	ins.Globals[id] = ins.OperandStack.pop()
}
