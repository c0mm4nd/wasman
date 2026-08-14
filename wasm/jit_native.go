package wasm

import (
	"fmt"
	"os"
	"sync/atomic"
	"unsafe"

	"github.com/c0mm4nd/wasman/wasm/jit"
)

// jitForceBaseline pins compilation to the baseline tier (test/debug knob,
// so the spec suite can exercise both tiers independently).
var jitForceBaseline = os.Getenv("WASMAN_JIT_TIER") == "baseline"

// compileNativeAll translates every eligible locally-defined function with
// the template JIT. Functions using constructs outside the compiled subset
// keep compiled == nil and run in the interpreter; on unsupported platforms
// jit.Compile rejects everything.
func (ins *Instance) compileNativeAll() {
	// native float arithmetic keeps hardware NaN payloads, so deterministic
	// NaN canonicalization stays on the interpreter
	if !jit.Supported() || ins.canonNaN {
		return
	}
	funcSigs := make([]jit.FuncSig, len(ins.IndexSpace.Functions))
	for i, fn := range ins.IndexSpace.Functions {
		t := fn.getType()
		funcSigs[i] = jit.FuncSig{In: len(t.InputTypes), Out: len(t.ReturnTypes)}
	}
	typeSigs := make([]jit.FuncSig, len(ins.TypeSection))
	for i, t := range ins.TypeSection {
		typeSigs[i] = jit.FuncSig{In: len(t.InputTypes), Out: len(t.ReturnTypes)}
	}
	for _, fn := range ins.IndexSpace.Functions {
		if wf, ok := fn.(*wasmFunc); ok && wf.owner == ins {
			ins.compileNative(wf, funcSigs, typeSigs)
		}
	}
}

func (ins *Instance) compileNative(f *wasmFunc, funcSigs, typeSigs []jit.FuncSig) {
	if f.imms == nil || f.compiled != nil {
		return
	}
	fd := &jit.FuncDesc{
		Body:      f.body,
		Imms:      append([]uint64(nil), f.imms...),
		PcEnd:     append([]uint32(nil), f.pcEnd...),
		NumLocals: int(f.NumLocal) + len(f.signature.InputTypes),
		NumParams: len(f.signature.InputTypes),
		NumRets:   len(f.signature.ReturnTypes),
		FuncSigs:  funcSigs,
		TypeSigs:  typeSigs,
	}
	if len(f.brPlans) > 0 {
		fd.BrTables = make(map[int]jit.BrTable, len(f.brPlans))
		for pc, plan := range f.brPlans {
			fd.BrTables[int(pc)] = jit.BrTable{Targets: plan.targets, Def: plan.def}
		}
	}
	// pack each block's param/result counts where the compiler expects them
	for pc, blk := range f.Blocks {
		fd.Imms[pc] = uint64(len(blk.BlockType.InputTypes))<<32 |
			uint64(len(blk.BlockType.ReturnTypes))
		fd.PcEnd[pc] = uint32(blk.StartAt + blk.BlockTypeBytes)
	}
	// tiered: the optimizing compiler first, the baseline template
	// compiler for anything outside its subset, the interpreter last
	if !jitForceBaseline {
		if cd, err := jit.CompileOpt(fd); err == nil {
			f.compiled = cd
			return
		}
	}
	if cd, err := jit.CompileBaseline(fd); err == nil {
		f.compiled = cd
	}
}

// callNative invokes a same-instance JIT-compiled callee from native code:
// the moral equivalent of wasmFunc.call without the interpreter frame setup —
// arguments are block-copied into the pooled frame's locals and the depth
// accounting matches the generic path exactly. Panics (with Recover on)
// unwind to the exported call's recover, which restores the outer state.
func (ins *Instance) callNative(f *wasmFunc) error {
	os := ins.OperandStack
	al := len(f.signature.InputTypes)
	baseSp := os.Ptr - al

	prevPtr := ins.FrameStack.Ptr
	if limit := ins.Module.ModuleConfig.CallDepthLimit; limit != nil &&
		uint64(prevPtr+1) >= *limit {
		return ErrCallStackExhausted
	}

	// the arguments already sit contiguously on the operand stack in local
	// order, so the callee's locals live there in place: only the declared
	// locals (the tail) need zeroing, and the callee's own stack area
	// starts right above them. No locals array, no copying.
	need := int(f.NumLocal) + al
	cd := f.compiled
	if want := baseSp + 1 + need + cd.MaxHeight; len(os.Values) < want {
		os.Values = append(os.Values, make([]uint64, want-len(os.Values))...)
	}
	for i := baseSp + 1 + al; i < baseSp+1+need; i++ {
		os.Values[i] = 0
	}
	os.Ptr = baseSp

	frame := ins.acquireFrame()
	prev := ins.Active
	frame.Func = f
	ins.FrameStack.Push(frame)
	ins.Active = frame

	err := ins.execNative(cd, nil, baseSp+1, baseSp+need)

	ins.FrameStack.Ptr = prevPtr
	ins.Active = prev
	ins.releaseFrame(frame)

	if err != nil {
		ins.OperandStack.Ptr = baseSp
		return annotateTrap(err, f.name)
	}
	// results land above the callee frame; move them to the caller's top
	copy(os.Values[baseSp+1:], os.Values[baseSp+need+1 : baseSp+need+1+f.compiled.MaxHeight][:len(f.signature.ReturnTypes)])
	os.Ptr = baseSp + len(f.signature.ReturnTypes)
	return nil
}

// execNative runs a JIT-compiled body: the native code works directly on the
// instance's operand stack slots above baseSp and on the frame's locals.
// Calls exit to this loop, which invokes the callee through the ordinary
// call machinery and re-enters the native code at the site's continuation;
// every re-entry refreshes the stack and memory base pointers, since a
// callee may have regrown either.
func (ins *Instance) execNative(cd *jit.Compiled, locals []uint64, localsOff, baseSp int) error {
	// localsOff >= 0 places the locals inside the operand stack itself (the
	// in-place native frame); the pointer then refreshes with the slice on
	// every re-entry, since a nested call may regrow it
	var ctx jit.Ctx
	if localsOff < 0 && len(locals) > 0 {
		ctx.Locals = uintptr(unsafe.Pointer(&locals[0]))
	}
	if len(ins.Globals) > 0 {
		ctx.Globals = uintptr(unsafe.Pointer(&ins.Globals[0]))
	}
	entry := 0
	for {
		os := ins.OperandStack
		if need := baseSp + 1 + cd.MaxHeight; len(os.Values) < need {
			os.Values = append(os.Values, make([]uint64, need-len(os.Values))...)
		}
		if localsOff >= 0 {
			ctx.Locals = uintptr(unsafe.Pointer(&os.Values[localsOff]))
		}
		if cd.MaxHeight > 0 {
			ctx.Stack = uintptr(unsafe.Pointer(&os.Values[baseSp+1]))
		}
		if ins.Memory != nil && len(ins.Memory.Value) > 0 {
			ctx.Mem = uintptr(unsafe.Pointer(&ins.Memory.Value[0]))
			ctx.MemLen = uint64(len(ins.Memory.Value))
		}
		switch st := jit.CallAt(cd.Code, entry, &ctx); st {
		case jit.StatusOK:
			os.Ptr = baseSp + int(ctx.Sp)
			return nil
		case jit.StatusCall, jit.StatusCallIndirect:
			if ctx.TrapInfo >= uint64(len(cd.CallSites)) {
				return ErrUndefined
			}
			site := &cd.CallSites[ctx.TrapInfo]
			os.Ptr = baseSp + site.SpBefore
			var err error
			switch site.Kind {
			case jit.SiteCallIndirect:
				err = callIndirectCore(ins, site.TypeIdx, site.TableIdx)
			case jit.SiteMemGrow:
				err = memoryGrowBody(ins)
			default:
				fn := ins.IndexSpace.Functions[site.FuncIdx]
				// same-instance native callees skip the generic call
				// ceremony (reflection-free arg copy, no per-level recover)
				if wf, ok := fn.(*wasmFunc); ok && wf.compiled != nil && wf.owner == ins {
					err = ins.callNative(wf)
				} else {
					err = fn.call(ins)
				}
			}
			if err != nil {
				return err
			}
			if os.Ptr != baseSp+site.SpAfter {
				return fmt.Errorf("function returned too few values")
			}
			// a call boundary is also the interruption point for native code
			if atomic.LoadUint32(&ins.interruptFlag) != 0 {
				return ErrInterrupted
			}
			entry = site.Cont
		case jit.StatusUnreachable:
			return ErrUnreachable
		case jit.StatusMemOOB:
			return ErrPtrOutOfBounds
		case jit.StatusConvInvalid:
			return ErrInvalidConversionToInt
		case jit.StatusConvOverflow:
			return ErrIntegerOverflow
		default: // div-by-zero / overflow trap the same way the interpreter does
			return ErrUndefined
		}
	}
}
