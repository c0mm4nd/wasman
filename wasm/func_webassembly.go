package wasm

import (
	"fmt"

	"github.com/c0mm4nd/wasman/stacks"
	"github.com/c0mm4nd/wasman/types"
)

type wasmFunc struct {
	signature *types.FuncType       // the shape of func (defined by inputs and outputs)
	NumLocal  uint32                // index id in local
	body      []byte                // body
	Blocks    map[uint64]*funcBlock // instr blocks inside the func
}

type funcBlock struct {
	StartAt uint64
	ElseAt  uint64
	EndAt   uint64

	BlockType      *types.FuncType
	BlockTypeBytes uint64
}

func (f *wasmFunc) getType() *types.FuncType {
	return f.signature
}

func (f *wasmFunc) call(ins *Instance) (err error) {
	al := len(f.signature.InputTypes)
	locals := make([]uint64, f.NumLocal+uint32(al))
	for i := 0; i < al; i++ {
		locals[al-1-i] = ins.OperandStack.Pop()
	}

	prevPtr := ins.FrameStack.Ptr
	if ins.Recover {
		defer func() {
			if v := recover(); v != nil {
				ins.FrameStack.Ptr = prevPtr
				var ok bool
				err, ok = v.(error)
				if !ok {
					err = fmt.Errorf("runtime error: %v", v)
				}
			}
		}()
	}

	prev := ins.Active
	frame := &Frame{
		Func:       f,
		Locals:     locals,
		LabelStack: stacks.NewLabelStack(),
	}
	ins.FrameStack.Push(frame)
	ins.Active = frame

	err = ins.execFunc()
	if err != nil {
		return err
	}

	ins.Active = prev

	return nil
}
