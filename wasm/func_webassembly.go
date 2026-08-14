package wasm

import (
	"errors"
	"fmt"

	"github.com/c0mm4nd/wasman/stacks"
	"github.com/c0mm4nd/wasman/types"
	"github.com/c0mm4nd/wasman/wasm/jit"
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

	// name is the debug name for trap backtraces (from the custom "name"
	// section, or a synthesized func[N]).
	name string

	// pre-decoded immediates, indexed by the PC of the opcode byte: imms holds
	// the (packed) immediate value and pcEnd the PC of the instruction's last
	// byte, so the exec loop never decodes LEB128 at run time. blocksAt is the
	// Blocks map flattened for O(1) lookup; brPlans holds br_table targets.
	imms     []uint64
	pcEnd    []uint32
	blocksAt []*funcBlock
	brPlans  map[uint64]*brPlan
	// brFast, indexed like imms, holds resumePC+1 for br/br_if sites whose
	// target is an enclosing loop: the branch can then jump straight to the
	// loop's first inner instruction, keeping the loop label in place instead
	// of popping it and re-executing the loop opcode every iteration.
	brFast []uint32

	// compiled is the native translation of the body (ModuleConfig.EnableJIT);
	// nil when the JIT is off, unsupported here, or the body uses constructs
	// outside the compiled subset.
	compiled *jit.Compiled
}

// brPlan is a pre-decoded br_table: its targets and default label.
type brPlan struct {
	targets []uint32
	def     uint32
}

// maxTraceFrames bounds a trap backtrace so deep recursion (e.g. a call-stack
// exhaustion with 1000+ frames) stays cheap and readable.
const maxTraceFrames = 16

// trapError decorates a trap with the wasm call frames it unwound through.
type trapError struct {
	err    error
	frames []string
}

func (t *trapError) Error() string {
	s := t.err.Error() + "\nwasm stack:"
	for _, f := range t.frames {
		s += "\n\tat " + f
	}
	return s
}

func (t *trapError) Unwrap() error { return t.err }

// annotateTrap appends the current frame to err's backtrace (in place when
// one exists already, so unwinding N frames stays O(N)).
func annotateTrap(err error, frame string) error {
	if te, ok := err.(*trapError); ok {
		if len(te.frames) < maxTraceFrames {
			te.frames = append(te.frames, frame)
		}
		return te
	}
	return &trapError{err: err, frames: []string{frame}}
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

	// frames (with their label stacks and locals arrays) are pooled per
	// instance: call-heavy code would otherwise allocate three objects per call
	frame := ins.acquireFrame()
	al := len(f.signature.InputTypes)
	need := int(f.NumLocal) + al
	if cap(frame.Locals) >= need {
		frame.Locals = frame.Locals[:need]
		for i := range frame.Locals {
			frame.Locals[i] = 0 // wasm locals start zeroed
		}
	} else {
		frame.Locals = make([]uint64, need)
	}
	locals := frame.Locals
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
		ins.releaseFrame(frame)
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
				err = annotateTrap(err, f.name)
			}
		}()
	}

	// JIT-compiled bodies run natively against the operand stack and locals;
	// tolls cannot be charged per instruction there, so a TollStation keeps
	// the interpreter path.
	if cd := f.compiled; cd != nil && ins.Module.ModuleConfig.TollStation == nil {
		err = ins.execNative(cd, locals, baseSp)
		ins.FrameStack.Ptr = prevPtr
		ins.Active = prev
		ins.releaseFrame(frame)
		if err != nil {
			ins.OperandStack.Ptr = baseSp
			return annotateTrap(err, f.name)
		}
		return nil
	}

	frame.Func = f
	// push the implicit label for the function body so that a `br` targeting the
	// function scope (or an early `return`-style branch) unwinds correctly: its
	// continuation is the end of the body and its arity is the result count.
	frame.LabelStack.Push(stacks.Label{
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
	ins.releaseFrame(frame)

	// a trap discards whatever the failing body left on the operand stack, so
	// repeated traps on a long-lived instance cannot grow the stack unboundedly
	if err != nil {
		ins.OperandStack.Ptr = baseSp
		return annotateTrap(err, f.name)
	}

	// a successful body must leave its declared results; an invalid body run
	// with SkipValidation may not, which would underflow every caller's pops
	// (callCross, internal call sites, CallExportedFunc)
	if ins.OperandStack.Ptr-baseSp < len(f.signature.ReturnTypes) {
		ins.OperandStack.Ptr = baseSp
		return fmt.Errorf("function returned too few values")
	}

	return nil
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

// blockAt resolves the block starting at pc: O(1) through the flattened
// array when compiled, falling back to the map for hand-built fixtures.
func (f *wasmFunc) blockAt(pc uint64) *funcBlock {
	if f.blocksAt != nil {
		if pc < uint64(len(f.blocksAt)) {
			return f.blocksAt[pc]
		}
		return nil
	}
	return f.Blocks[pc]
}
