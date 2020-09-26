package wasman

import (
	"github.com/c0mm4nd/wasman/types"
)

type wasmFunc struct {
	Signature *types.FuncType
	NumLocal  uint32
	Body      []byte
	Blocks    map[uint64]*funcBlock
}

type funcBlock struct {
	StartAt        uint64
	ElseAt         uint64
	EndAt          uint64
	BlockType      *types.FuncType
	BlockTypeBytes uint64
}

func (f *wasmFunc) FuncType() *types.FuncType {
	return f.Signature
}

func (f *wasmFunc) Call(ins *Instance) {
	al := len(f.Signature.InputTypes)
	locals := make([]uint64, f.NumLocal+uint32(al))
	for i := 0; i < al; i++ {
		locals[al-1-i] = ins.OperandStack.pop()
	}

	prev := ins.Context
	ins.Context = &wasmContext{
		Func:       f,
		Locals:     locals,
		LabelStack: newLabelStack(),
	}

	ins.execFunc()
	ins.Context = prev
}
