package wasm

import (
	"errors"
	"fmt"

	"github.com/c0mm4nd/wasman/stacks"
	"github.com/c0mm4nd/wasman/types"
)

// ErrCallStackExhausted is returned when the configured
// ModuleConfig.CallDepthLimit is exceeded, mirroring a wasm "call stack
// exhausted" trap instead of letting the Go stack overflow fatally.
var ErrCallStackExhausted = errors.New("call stack exhausted")

type wasmFunc struct {
	signature *types.FuncType       // the shape of func (defined by inputs and outputs)
	NumLocal  uint32                // index id in local
	body      []byte                // body
	Blocks    map[uint64]*funcBlock // instr blocks inside the func

	// owner is the instance this function was defined in. A call coming from a
	// different instance (an imported function) must execute against the
	// owner's module state — its functions, globals, memory and tables — not
	// the caller's.
	owner *Instance
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
	// a cross-instance call (imported function) runs in its owner's context,
	// with the argument/result values ferried between the two operand stacks.
	if f.owner != nil && f.owner != ins {
		return f.callCross(ins)
	}

	al := len(f.signature.InputTypes)
	locals := make([]uint64, f.NumLocal+uint32(al))
	for i := 0; i < al; i++ {
		locals[al-1-i] = ins.OperandStack.Pop()
	}

	prevPtr := ins.FrameStack.Ptr
	prev := ins.Active
	// the operand stack height after consuming the arguments; a trap unwinds
	// back to it so values pushed by the failing body do not leak
	baseSp := ins.OperandStack.Ptr

	// enforce the optional call-depth limit so runaway recursion traps as a
	// wasm "call stack exhausted" instead of fatally overflowing the Go stack.
	if limit := ins.Module.ModuleConfig.CallDepthLimit; limit != nil &&
		uint64(prevPtr+1) >= *limit {
		return ErrCallStackExhausted
	}

	if ins.Recover {
		defer func() {
			if v := recover(); v != nil {
				// unwind the frame back to the caller so the instance stays usable
				ins.FrameStack.Ptr = prevPtr
				ins.Active = prev
				ins.OperandStack.Ptr = baseSp
				var ok bool
				err, ok = v.(error)
				if !ok {
					err = fmt.Errorf("runtime error: %v", v)
				}
			}
		}()
	}

	frame := &Frame{
		Func:       f,
		Locals:     locals,
		LabelStack: stacks.NewLabelStack(),
	}
	// push the implicit label for the function body so that a `br` targeting the
	// function scope (or an early `return`-style branch) unwinds correctly: its
	// continuation is the end of the body and its arity is the result count.
	frame.LabelStack.Push(&stacks.Label{
		Arity:          len(f.signature.ReturnTypes),
		Sp:             ins.OperandStack.Ptr,
		ContinuationPC: uint64(len(f.body)),
		EndPC:          uint64(len(f.body)),
	})
	ins.FrameStack.Push(frame)
	ins.Active = frame

	err = ins.execFunc()

	// always pop this frame so the FrameStack does not grow unboundedly across
	// sequential calls (and the caller frame is restored on both success and
	// a returned trap error).
	ins.FrameStack.Ptr = prevPtr
	ins.Active = prev

	// a trap discards whatever the failing body left on the operand stack, so
	// repeated traps on a long-lived instance cannot grow the stack unboundedly
	if err != nil {
		ins.OperandStack.Ptr = baseSp
	}

	return err
}

// callCross executes f inside its owning instance while the call came from
// another instance: arguments are moved from the caller's operand stack to the
// owner's, the function runs entirely in the owner's module state, and the
// results are moved back.
func (f *wasmFunc) callCross(caller *Instance) error {
	owner := f.owner

	n := len(f.signature.InputTypes)
	args := make([]uint64, n)
	for i := n - 1; i >= 0; i-- {
		args[i] = caller.OperandStack.Pop()
	}
	for _, a := range args {
		owner.OperandStack.Push(a)
	}

	if err := f.call(owner); err != nil {
		return err
	}

	m := len(f.signature.ReturnTypes)
	rets := make([]uint64, m)
	for i := m - 1; i >= 0; i-- {
		rets[i] = owner.OperandStack.Pop()
	}
	for _, r := range rets {
		caller.OperandStack.Push(r)
	}

	return nil
}
