package wasman


func getLocal(ins *Instance) {
	ins.Context.PC++
	id := ins.fetchUint32()
	ins.OperandStack.push(ins.Context.Locals[id])
}

func setLocal(ins *Instance) {
	ins.Context.PC++
	id := ins.fetchUint32()
	v := ins.OperandStack.pop()
	ins.Context.Locals[id] = v
}

func teeLocal(ins *Instance) {
	ins.Context.PC++
	id := ins.fetchUint32()
	v := ins.OperandStack.peek()
	ins.Context.Locals[id] = v
}

