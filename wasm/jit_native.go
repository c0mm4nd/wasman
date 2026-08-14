package wasm

import (
	"fmt"
	"sync/atomic"
	"unsafe"

	"github.com/c0mm4nd/wasman/wasm/jit"
)

// compileNativeAll translates every eligible locally-defined function with
// the template JIT. Functions using constructs outside the compiled subset
// keep compiled == nil and run in the interpreter; on unsupported platforms
// jit.Compile rejects everything.
func (ins *Instance) compileNativeAll() {
	if !jit.Supported() {
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
	if cd, err := jit.Compile(fd); err == nil {
		f.compiled = cd
	}
}

// execNative runs a JIT-compiled body: the native code works directly on the
// instance's operand stack slots above baseSp and on the frame's locals.
// Calls exit to this loop, which invokes the callee through the ordinary
// call machinery and re-enters the native code at the site's continuation;
// every re-entry refreshes the stack and memory base pointers, since a
// callee may have regrown either.
func (ins *Instance) execNative(cd *jit.Compiled, locals []uint64, baseSp int) error {
	var ctx jit.Ctx
	if len(locals) > 0 {
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
			if site.Indirect {
				err = callIndirectCore(ins, site.TypeIdx, site.TableIdx)
			} else {
				err = ins.IndexSpace.Functions[site.FuncIdx].call(ins)
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
		default: // div-by-zero / overflow trap the same way the interpreter does
			return ErrUndefined
		}
	}
}
