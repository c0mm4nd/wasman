package wasman

import (
	"github.com/c0mm4nd/wasman/types"
)

func call(ins *Instance) {
	ins.Context.PC++
	index := ins.fetchUint32()
	ins.Functions[index].Call(ins)
}

func callIndirect(ins *Instance) {
	ins.Context.PC++
	index := ins.fetchUint32()
	expType := ins.Module.TypesSection[index]

	tableIndex := ins.OperandStack.pop()
	// note: mvp limits the size of table index space to 1
	if tableIndex >= uint64(len(ins.Module.indexSpace.Tables[0])) {
		panic("table index out of range")
	}

	te := ins.Module.indexSpace.Tables[0][tableIndex]
	if te == nil {
		panic("table entry not initialized")
	}

	f := ins.Functions[*te]
	ft := f.FuncType()
	if !types.HasSameSignature(ft.InputTypes, expType.InputTypes) ||
		!types.HasSameSignature(ft.ReturnTypes, expType.ReturnTypes) {
		panic("function signature mismatch")
	}
	f.Call(ins)

	ins.Context.PC++ // skip 0x00
}
