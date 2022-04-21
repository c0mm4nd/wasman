package wasm

import (
	"bytes"
	"errors"

	"github.com/c0mm4nd/wasman/leb128decode"
	"github.com/c0mm4nd/wasman/stacks"
	"github.com/c0mm4nd/wasman/types"
)

// errors on control instr
var (
	ErrUnreachable                 = errors.New("unreachable")
	ErrBlockNotInitialized         = errors.New("block not initialized")
	ErrBlockNotFound               = errors.New("block not found")
	ErrFuncSignMismatch            = errors.New("function signature mismatch")
	ErrLabelNotFound               = errors.New("label not found")
	ErrTableIndexOutOfRange        = errors.New("table index out of range")
	ErrTableInstanceNotInitialized = errors.New("table entry not initialized")
)

func unreachable(_ *Instance) error {
	return ErrUnreachable
}

func nop(_ *Instance) error {
	return nil
}

func block(ins *Instance) error {
	ctx := ins.Active
	block, ok := ctx.Func.Blocks[ctx.PC]
	if !ok {
		return ErrBlockNotInitialized
	}

	ctx.PC += block.BlockTypeBytes
	ctx.LabelStack.Push(&stacks.Label{
		Arity:          len(block.BlockType.ReturnTypes),
		ContinuationPC: block.EndAt,
		EndPC:          block.EndAt,
	})

	return nil
}

func loop(ins *Instance) error {
	ctx := ins.Active
	block, ok := ctx.Func.Blocks[ctx.PC]
	if !ok {
		return ErrBlockNotFound
	}
	ctx.PC += block.BlockTypeBytes
	ctx.LabelStack.Push(&stacks.Label{
		Arity:          len(block.BlockType.ReturnTypes),
		ContinuationPC: block.StartAt - 1,
		EndPC:          block.EndAt,
	})

	return nil
}

func ifOp(ins *Instance) error {
	ctx := ins.Active
	block, ok := ctx.Func.Blocks[ins.Active.PC]
	if !ok {
		return ErrBlockNotInitialized
	}
	ctx.PC += block.BlockTypeBytes

	if ins.OperandStack.Pop() == 0 {
		// enter else
		ins.Active.PC = block.ElseAt
	}

	ctx.LabelStack.Push(&stacks.Label{
		Arity:          len(block.BlockType.ReturnTypes),
		ContinuationPC: block.EndAt,
		EndPC:          block.EndAt,
	})

	return nil
}

func elseOp(ins *Instance) error {
	l := ins.Active.LabelStack.Pop()
	ins.Active.PC = l.EndPC

	return nil
}

func end(ins *Instance) error {
	if ins.Active.LabelStack.Ptr > -1 {
		_ = ins.Active.LabelStack.Pop()
	}

	return nil
}

func br(ins *Instance) error {
	ins.Active.PC++
	index, err := ins.fetchUint32()
	if err != nil {
		return err
	}

	return branchAt(ins, index)
}

func branchAt(ins *Instance, index uint32) error {
	var l *stacks.Label

	for i := uint32(0); i < index+1; i++ {
		l = ins.Active.LabelStack.Pop()
	}

	if l == nil {
		return ErrLabelNotFound
	}

	ins.Active.PC = l.ContinuationPC

	return nil
}

func brIf(ins *Instance) error {
	ins.Active.PC++
	index, err := ins.fetchUint32()
	if err != nil {
		return err
	}

	c := ins.OperandStack.Pop()
	if c != 0 {
		return branchAt(ins, index)
	}

	return nil
}

func brTable(ins *Instance) error {
	ins.Active.PC++
	r := bytes.NewReader(ins.Active.Func.body[ins.Active.PC:])
	nl, num, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return err
	}

	lis := make([]uint32, nl)
	for i := range lis {
		li, n, err := leb128decode.DecodeUint32(r)
		if err != nil {
			return err
		}
		num += n
		lis[i] = li
	}

	ln, n, err := leb128decode.DecodeUint32(r)
	if err != nil {
		return err
	}
	ins.Active.PC += n + num

	i := ins.OperandStack.Pop()
	if uint32(i) < nl {
		return branchAt(ins, lis[i])
	}

	return branchAt(ins, ln)
}

func call(ins *Instance) error {
	ins.Active.PC++
	index, err := ins.fetchUint32()
	if err != nil {
		return err
	}

	err = ins.Functions[index].call(ins)
	if err != nil {
		return err
	}

	return nil
}

func callIndirect(ins *Instance) error {
	ins.Active.PC++
	index, err := ins.fetchUint32()
	if err != nil {
		return err
	}

	expType := ins.Module.TypeSection[index]

	tableIndex := ins.OperandStack.Pop()
	// note: mvp limits the size of table index space to 1
	if tableIndex >= uint64(len(ins.Module.IndexSpace.Tables[0].Value)) {
		return ErrTableIndexOutOfRange
	}

	te := ins.Module.IndexSpace.Tables[0].Value[tableIndex]
	if te == nil {
		return ErrTableInstanceNotInitialized
	}

	f := ins.Functions[*te]
	ft := f.getType()
	if !types.HasSameSignature(ft.InputTypes, expType.InputTypes) ||
		!types.HasSameSignature(ft.ReturnTypes, expType.ReturnTypes) {
		return ErrFuncSignMismatch
	}

	err = f.call(ins)
	if err != nil {
		return err
	}

	ins.Active.PC++ // skip 0x00

	return nil
}
