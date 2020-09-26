package wasman

import (
	"bytes"

	"github.com/c0mm4nd/wasman/leb128"
)

func block(ins *Instance) {
	ctx := ins.Context
	block, ok := ctx.Func.Blocks[ctx.PC]
	if !ok {
		panic("block not initialized")
	}

	ctx.PC += block.BlockTypeBytes
	ctx.LabelStack.push(&label{
		Arity:          len(block.BlockType.ReturnTypes),
		ContinuationPC: block.EndAt,
		EndPC:          block.EndAt,
	})
}

func loop(ins *Instance) {
	ctx := ins.Context
	block, ok := ctx.Func.Blocks[ctx.PC]
	if !ok {
		panic("block not found")
	}
	ctx.PC += block.BlockTypeBytes
	ctx.LabelStack.push(&label{
		Arity:          len(block.BlockType.ReturnTypes),
		ContinuationPC: block.StartAt - 1,
		EndPC:          block.EndAt,
	})
}

func ifOp(ins *Instance) {
	ctx := ins.Context
	block, ok := ctx.Func.Blocks[ins.Context.PC]
	if !ok {
		panic("block not initialized")
	}
	ctx.PC += block.BlockTypeBytes

	if ins.OperandStack.pop() == 0 {
		// enter else
		ins.Context.PC = block.ElseAt
	}

	ctx.LabelStack.push(&label{
		Arity:          len(block.BlockType.ReturnTypes),
		ContinuationPC: block.EndAt,
		EndPC:          block.EndAt,
	})
}

func elseOp(ins *Instance) {
	l := ins.Context.LabelStack.pop()
	ins.Context.PC = l.EndPC
}

func end(ins *Instance) {
	if ins.Context.LabelStack.SP > -1 {
		_ = ins.Context.LabelStack.pop()
	}
}

func br(ins *Instance) {
	ins.Context.PC++
	index := ins.fetchUint32()
	brAt(ins, index)
}

func brIf(ins *Instance) {
	ins.Context.PC++
	index := ins.fetchUint32()
	c := ins.OperandStack.pop()
	if c != 0 {
		brAt(ins, index)
	}
}

func brAt(ins *Instance, index uint32) {
	var l *label

	for i := uint32(0); i < index+1; i++ {
		l = ins.Context.LabelStack.pop()
	}

	ins.Context.PC = l.ContinuationPC
}

func brTable(ins *Instance) {
	ins.Context.PC++
	r := bytes.NewBuffer(ins.Context.Func.Body[ins.Context.PC:])
	nl, num, err := leb128.DecodeUint32(r)
	if err != nil {
		panic(err)
	}

	lis := make([]uint32, nl)
	for i := range lis {
		li, n, err := leb128.DecodeUint32(r)
		if err != nil {
			panic(err)
		}
		num += n
		lis[i] = li
	}

	ln, n, err := leb128.DecodeUint32(r)
	if err != nil {
		panic(err)
	}
	ins.Context.PC += n + num

	i := ins.OperandStack.pop()
	if uint32(i) < nl {
		brAt(ins, lis[i])
	} else {
		brAt(ins, ln)
	}
}
