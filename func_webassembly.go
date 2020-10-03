package wasman

import (
	"github.com/c0mm4nd/wasman/stacks"
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

func (f *wasmFunc) getType() *types.FuncType {
	return f.Signature
}

func (f *wasmFunc) call(ins *Instance) error {
	al := len(f.Signature.InputTypes)
	locals := make([]uint64, f.NumLocal+uint32(al))
	for i := 0; i < al; i++ {
		locals[al-1-i] = ins.OperandStack.Pop()
	}

	prev := ins.Context
	ins.Context = &wasmContext{
		Func:       f,
		Locals:     locals,
		LabelStack: stacks.NewLabelStack(),
	}

	err := ins.execFunc()
	if err != nil {
		return err
	}

	ins.Context = prev

	return nil
}
