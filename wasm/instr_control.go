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
	block := ctx.Func.blockAt(ctx.PC)
	if block == nil {
		return ErrBlockNotInitialized
	}

	ctx.PC += block.BlockTypeBytes
	ctx.LabelStack.Push(stacks.Label{
		Arity: len(block.BlockType.ReturnTypes),
		// Sp is captured below the block's parameters so a forward branch keeps
		// the result values (multi-value: a block may take parameters).
		Sp:             ins.OperandStack.Ptr - len(block.BlockType.InputTypes),
		ContinuationPC: block.EndAt,
		EndPC:          block.EndAt,
	})

	return nil
}

func loop(ins *Instance) error {
	ctx := ins.Active
	block := ctx.Func.blockAt(ctx.PC)
	if block == nil {
		return ErrBlockNotFound
	}
	ctx.PC += block.BlockTypeBytes
	ctx.LabelStack.Push(stacks.Label{
		// branching to a loop targets its start, carrying back the loop's
		// parameters (0 in the MVP, non-zero with multi-value loops).
		Arity:          len(block.BlockType.InputTypes),
		Sp:             ins.OperandStack.Ptr - len(block.BlockType.InputTypes),
		ContinuationPC: block.StartAt - 1,
		EndPC:          block.EndAt,
	})

	return nil
}

func ifOp(ins *Instance) error {
	ctx := ins.Active
	block := ctx.Func.blockAt(ins.Active.PC)
	if block == nil {
		return ErrBlockNotInitialized
	}
	ctx.PC += block.BlockTypeBytes

	if ins.OperandStack.Pop() == 0 { // condition is false
		if block.ElseAt > block.StartAt {
			// enter the else branch (a label is still needed so its matching
			// end can unwind the operand stack)
			ins.Active.PC = block.ElseAt
		} else {
			// no else branch: skip the whole if, including its end. Nothing was
			// pushed, so there is no label to pop.
			ins.Active.PC = block.EndAt
			return nil
		}
	}

	ctx.LabelStack.Push(stacks.Label{
		Arity:          len(block.BlockType.ReturnTypes),
		Sp:             ins.OperandStack.Ptr - len(block.BlockType.InputTypes),
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

// brLoopFast takes a branch to an enclosing loop: drop the inner labels,
// KEEP the loop label, carry its params and jump straight into the body.
func brLoopFast(ins *Instance, depth uint32, resume uint32) {
	frame := ins.Active
	ls := frame.LabelStack
	ls.Ptr -= int(depth)
	l := &ls.Values[ls.Ptr]
	os := ins.OperandStack
	if l.Arity > 0 {
		src := os.Ptr - l.Arity + 1
		dst := l.Sp + 1
		if dst != src {
			copy(os.Values[dst:dst+l.Arity], os.Values[src:src+l.Arity])
		}
	}
	os.Ptr = l.Sp + l.Arity
	frame.PC = uint64(resume - 1)
}

func br(ins *Instance) error {
	if f := ins.Active.Func; f.imms != nil {
		p := ins.Active.PC
		if r := f.brFast[p]; r != 0 {
			brLoopFast(ins, uint32(f.imms[p]), r)
			return nil
		}
		return branchAt(ins, uint32(f.imms[ins.Active.PC]))
	}
	ins.Active.PC++
	index, err := ins.fetchUint32()
	if err != nil {
		return err
	}
	return branchAt(ins, index)
}

func branchAt(ins *Instance, index uint32) error {
	ls := ins.Active.LabelStack
	if ls.Ptr < int(index) {
		return ErrLabelNotFound
	}

	var l stacks.Label
	for i := uint32(0); i < index+1; i++ {
		l = ls.Pop()
	}

	// Unwind the operand stack to the target label's height, preserving the top
	// Arity result values. This discards everything the branched-out blocks
	// pushed, which is what makes br/br_if/br_table carry the right values.
	os := ins.OperandStack
	if l.Arity > 0 {
		src := os.Ptr - l.Arity + 1
		dst := l.Sp + 1
		if dst != src {
			copy(os.Values[dst:dst+l.Arity], os.Values[src:src+l.Arity])
		}
	}
	os.Ptr = l.Sp + l.Arity

	ins.Active.PC = l.ContinuationPC

	return nil
}

func brIf(ins *Instance) error {
	if f := ins.Active.Func; f.imms != nil {
		p := ins.Active.PC
		if ins.OperandStack.Pop() != 0 {
			if r := f.brFast[p]; r != 0 {
				brLoopFast(ins, uint32(f.imms[p]), r)
				return nil
			}
			return branchAt(ins, uint32(f.imms[p]))
		}
		ins.Active.PC = uint64(f.pcEnd[p])
		return nil
	}
	ins.Active.PC++
	index, err := ins.fetchUint32()
	if err != nil {
		return err
	}
	if ins.OperandStack.Pop() != 0 {
		return branchAt(ins, index)
	}
	return nil
}

func brTable(ins *Instance) error {
	if f := ins.Active.Func; f.brPlans != nil {
		if plan := f.brPlans[ins.Active.PC]; plan != nil {
			i := ins.OperandStack.Pop()
			if i < uint64(len(plan.targets)) {
				return branchAt(ins, plan.targets[i])
			}
			return branchAt(ins, plan.def)
		}
	}
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
	if f := ins.Active.Func; f.imms != nil {
		p := ins.Active.PC
		ins.Active.PC = uint64(f.pcEnd[p])
		return ins.Functions[f.imms[p]].call(ins)
	}
	ins.Active.PC++
	index, err := ins.fetchUint32()
	if err != nil {
		return err
	}
	return ins.Functions[index].call(ins)
}

func callIndirect(ins *Instance) error {
	var typeIndex, tableIndex uint32
	if f := ins.Active.Func; f.imms != nil {
		p := ins.Active.PC
		packed := f.imms[p]
		typeIndex, tableIndex = uint32(packed>>32), uint32(packed)
		ins.Active.PC = uint64(f.pcEnd[p])
	} else {
		ins.Active.PC++
		ti, err := ins.fetchUint32()
		if err != nil {
			return err
		}
		typeIndex = ti
		// the table index immediate (0 in the MVP, non-zero with multi-table)
		ins.Active.PC++
		tbl, err := ins.fetchUint32()
		if err != nil {
			return err
		}
		tableIndex = tbl
	}

	expType := ins.Module.TypeSection[typeIndex]

	if tableIndex >= uint32(len(ins.IndexSpace.Tables)) {
		return ErrTableIndexOutOfRange
	}
	table := ins.IndexSpace.Tables[tableIndex]

	elemIndex := ins.OperandStack.Pop()
	if elemIndex >= uint64(len(table.Value)) {
		return ErrTableIndexOutOfRange
	}

	// table slots hold resolved function references (possibly from another
	// module sharing this table); nil means uninitialized/null.
	f := table.Value[elemIndex]
	if f == nil {
		return ErrTableInstanceNotInitialized
	}

	ft := f.getType()
	if !types.HasSameSignature(ft.InputTypes, expType.InputTypes) ||
		!types.HasSameSignature(ft.ReturnTypes, expType.ReturnTypes) {
		return ErrFuncSignMismatch
	}

	return f.call(ins)
}
