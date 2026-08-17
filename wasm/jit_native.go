package wasm

import (
	"fmt"
	"os"
	"sync/atomic"
	"unsafe"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/tollstation"
	"github.com/c0mm4nd/wasman/types"
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
		sig := jit.FuncSig{In: len(t.InputTypes), Out: len(t.ReturnTypes)}
		if wf, ok := fn.(*wasmFunc); ok {
			sig.Locals = int(wf.NumLocal) + sig.In
		}
		funcSigs[i] = sig
	}
	typeSigs := make([]jit.FuncSig, len(ins.TypeSection))
	for i, t := range ins.TypeSection {
		typeSigs[i] = jit.FuncSig{In: len(t.InputTypes), Out: len(t.ReturnTypes)}
	}
	var depthLimit uint64
	if l := ins.Module.ModuleConfig.CallDepthLimit; l != nil {
		depthLimit = *l
	}
	// a TollStation that exposes its ceiling and prices every opcode
	// uniformly can be metered inline by the baseline tier; anything else
	// keeps the metered interpreter (the JIT stays off for it).
	var tollPrice uint64
	if ts := ins.Module.ModuleConfig.TollStation; ts != nil {
		if mt, ok := ts.(interface{ GetMax() uint64 }); ok {
			if p, uniform := uniformOpPrice(ts); uniform && p > 0 {
				tollPrice = p
				ins.metered = true
				ins.tollMax = mt.GetMax()
			}
		}
		if !ins.metered {
			return // non-meterable toll: interpreter only
		}
	}
	// instance-wide structural signature ids: type-section entries first
	// (compiled code embeds these), table entries may extend the map later
	ins.sigIDs = make(map[string]uint32)
	typeSigIDs := make([]uint32, len(ins.TypeSection))
	for i, t := range ins.TypeSection {
		typeSigIDs[i] = ins.sigID(sigKey(t))
	}
	var wideOps []uint16
	for i, fn := range ins.IndexSpace.Functions {
		if hf, ok := fn.(*HostFunc); ok && hf.wideOp != 0 {
			if wideOps == nil {
				wideOps = make([]uint16, len(ins.IndexSpace.Functions))
			}
			wideOps[i] = hf.wideOp
		}
	}
	type job struct {
		wf *wasmFunc
		fd *jit.FuncDesc
	}
	var jobs []job
	for i, fn := range ins.IndexSpace.Functions {
		if wf, ok := fn.(*wasmFunc); ok && wf.owner == ins && wf.imms != nil && wf.compiled == nil {
			fd := buildFuncDesc(wf, funcSigs, typeSigs)
			fd.SelfIdx = uint32(i)
			fd.DepthLimit = depthLimit
			fd.TypeSigIDs = typeSigIDs
			fd.WideOps = wideOps
			fd.TollPrice = tollPrice
			jobs = append(jobs, job{wf, fd})
		}
	}
	// pass 1: fix the native-call target set before generating any code, so
	// call sites to functions that compile later can still go native
	forceBaseline := jitForceBaseline || tollPrice != 0 // opt tier cannot meter
	var native []bool
	if jit.NativeCallsSupported() && !forceBaseline {
		native = make([]bool, len(ins.IndexSpace.Functions))
		for _, j := range jobs {
			native[j.fd.SelfIdx] = jit.OptEligible(j.fd)
		}
	}
	// pass 2: compile; if a promised-native function still fails (resource
	// errors), restart with native calls disabled — a stale promise would
	// leave call sites targeting a hole in the entry table
	for retry := 0; retry < 2; retry++ {
		ok := true
		for _, j := range jobs {
			j.wf.compiled = nil
			if !forceBaseline {
				j.fd.NativeFuncs = native
				if cd, err := jit.CompileOpt(j.fd); err == nil {
					j.wf.compiled = cd
					continue
				} else if native != nil && native[j.fd.SelfIdx] {
					ok = false
					break
				}
			}
			if cd, err := jit.CompileBaseline(j.fd); err == nil {
				j.wf.compiled = cd
			}
		}
		if ok {
			break
		}
		native = nil
	}
	if native != nil {
		ins.nativeEntries = make([]uintptr, len(ins.IndexSpace.Functions))
		for i, fn := range ins.IndexSpace.Functions {
			if wf, ok := fn.(*wasmFunc); ok && wf.compiled != nil && wf.compiled.NativeABI {
				ins.nativeEntries[i] = uintptr(unsafe.Pointer(&wf.compiled.Code[0]))
			}
		}
	}
}

func buildFuncDesc(f *wasmFunc, funcSigs, typeSigs []jit.FuncSig) *jit.FuncDesc {
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
	return fd
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
	var ts tollstation.TollStation
	if ins.metered {
		ts = ins.Module.ModuleConfig.TollStation
		ctx.Toll = ts.GetToll()
		ctx.TollMax = ins.tollMax
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
			if ts != nil {
				_ = ts.AddToll(ctx.Toll - ts.GetToll())
			}
			os.Ptr = baseSp + int(ctx.Sp)
			return nil
		case jit.StatusCall, jit.StatusCallIndirect:
			if ctx.TrapInfo >= uint64(len(cd.CallSites)) {
				return ErrUndefined
			}
			site := &cd.CallSites[ctx.TrapInfo]
			os.Ptr = baseSp + site.SpBefore
			if ts != nil { // charge what native code consumed before the callee
				_ = ts.AddToll(ctx.Toll - ts.GetToll())
			}
			var err error
			switch site.Kind {
			case jit.SiteCallIndirect:
				err = callIndirectCore(ins, site.TypeIdx, site.TableIdx)
			case jit.SiteMemGrow:
				err = memoryGrowBody(ins)
			case jit.SiteMemFill:
				err = memoryFill(ins)
			case jit.SiteMemCopy:
				err = memoryCopy(ins)
			default:
				fn := ins.IndexSpace.Functions[site.FuncIdx]
				// same-instance native callees skip the generic call
				// ceremony (reflection-free arg copy, no per-level recover)
				if wf, ok := fn.(*wasmFunc); ok && wf.compiled != nil &&
					!wf.compiled.NativeABI && wf.owner == ins {
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
			if ts != nil { // the callee charged the shared station; resume from it
				ctx.Toll = ts.GetToll()
			}
			// a call boundary is also the interruption point for native code
			if atomic.LoadUint32(&ins.interruptFlag) != 0 {
				return ErrInterrupted
			}
			entry = site.Cont
		case jit.StatusToll:
			if ts != nil {
				_ = ts.AddToll(ctx.Toll - ts.GetToll())
			}
			return tollstation.ErrTollOverflow
		case jit.StatusUnreachable:
			if ts != nil {
				_ = ts.AddToll(ctx.Toll - ts.GetToll())
			}
			return ErrUnreachable
		case jit.StatusMemOOB:
			if ts != nil {
				_ = ts.AddToll(ctx.Toll - ts.GetToll())
			}
			return ErrPtrOutOfBounds
		case jit.StatusConvInvalid:
			if ts != nil {
				_ = ts.AddToll(ctx.Toll - ts.GetToll())
			}
			return ErrInvalidConversionToInt
		case jit.StatusConvOverflow:
			if ts != nil {
				_ = ts.AddToll(ctx.Toll - ts.GetToll())
			}
			return ErrIntegerOverflow
		default: // div-by-zero / overflow trap the same way the interpreter does
			return ErrUndefined
		}
	}
}

// nativeStackSlots sizes the dedicated stack backing native call frames
// (64K slots = 512KiB); exceeding it traps as call-stack exhaustion.
const nativeStackSlots = 1 << 16

// execNativeABI runs a function compiled for the native-call ABI: frames
// live on the dedicated native stack (locals below the stack base), calls
// between such functions are direct native calls, and only calls that leave
// the compiled world exit to this loop, which ferries arguments between the
// native stack and the interpreter's operand stack.
func (ins *Instance) execNativeABI(f *wasmFunc) error {
	cd := f.compiled
	osk := ins.OperandStack
	al := len(f.signature.InputTypes)
	need := int(f.NumLocal) + al
	if ins.nativeStack == nil {
		ins.nativeStack = make([]uint64, nativeStackSlots)
	}
	ns := ins.nativeStack
	start := ins.nativeTop // above any chain suspended in a host exit
	if start+cd.FrameSlots > len(ns) {
		return ErrCallStackExhausted
	}
	copy(ns[start:start+al], osk.Values[osk.Ptr-al+1:osk.Ptr+1])
	osk.Ptr -= al

	base := uintptr(unsafe.Pointer(&ns[0]))
	var ctx jit.Ctx
	ctx.Stack = base + uintptr((start+need)*8)
	ctx.StackLimit = base + uintptr(len(ns)*8)
	if len(ins.nativeEntries) > 0 {
		ctx.Funcs = uintptr(unsafe.Pointer(&ins.nativeEntries[0]))
	}
	if len(ins.Globals) > 0 {
		ctx.Globals = uintptr(unsafe.Pointer(&ins.Globals[0]))
	}
	if len(ins.indirectMirror) > 0 {
		ctx.Indirect = uintptr(unsafe.Pointer(&ins.indirectMirror[0]))
	}
	ctx.Depth = uint64(ins.FrameStack.Ptr + 1)

	code, entry := cd.Code, 0
	for {
		if ins.Memory != nil && len(ins.Memory.Value) > 0 {
			ctx.Mem = uintptr(unsafe.Pointer(&ins.Memory.Value[0]))
			ctx.MemLen = uint64(len(ins.Memory.Value))
		} else {
			ctx.Mem, ctx.MemLen = 0, 0
		}
		switch st := jit.CallAt(code, entry, &ctx); st {
		case jit.StatusOK:
			fb := int((uintptr(ctx.Sp) - base) / 8)
			for i := 0; i < len(f.signature.ReturnTypes); i++ {
				osk.Push(ns[fb+i])
			}
			return nil
		case jit.StatusCall, jit.StatusCallIndirect:
			// the exit names the frame's function and site; any compiled
			// function in the chain may be the one exiting
			fidx, sid := uint32(ctx.TrapInfo>>32), uint32(ctx.TrapInfo)
			if int(fidx) >= len(ins.IndexSpace.Functions) {
				return ErrUndefined
			}
			ef, ok := ins.IndexSpace.Functions[fidx].(*wasmFunc)
			if !ok || ef.compiled == nil || int(sid) >= len(ef.compiled.CallSites) {
				return ErrUndefined
			}
			site := &ef.compiled.CallSites[sid]
			fb := int((uintptr(ctx.Sp) - base) / 8)
			var nin, nout int
			switch site.Kind {
			case jit.SiteCallIndirect:
				sig := ins.TypeSection[site.TypeIdx]
				nin, nout = len(sig.InputTypes)+1, len(sig.ReturnTypes)
			case jit.SiteMemGrow:
				nin, nout = 1, 1
			case jit.SiteMemFill, jit.SiteMemCopy:
				nin, nout = 3, 0
			default:
				sig := ins.IndexSpace.Functions[site.FuncIdx].getType()
				nin, nout = len(sig.InputTypes), len(sig.ReturnTypes)
			}
			argBase := fb + site.SpBefore - nin
			for i := 0; i < nin; i++ {
				osk.Push(ns[argBase+i])
			}
			// the callee may re-enter native code; its frames go above ours,
			// and the generic depth accounting must see the suspended native
			// frames (ctx.Depth carries the chain's true depth when a limit
			// is configured)
			prevTop := ins.nativeTop
			ins.nativeTop = fb + ef.compiled.FrameSlots - ef.compiled.LocalSlots
			prevFrames := ins.FrameStack.Ptr
			if d := int(ctx.Depth); d > prevFrames+1 {
				ins.FrameStack.Ptr = d - 1
			}
			var err error
			switch site.Kind {
			case jit.SiteCallIndirect:
				err = callIndirectCore(ins, site.TypeIdx, site.TableIdx)
			case jit.SiteMemGrow:
				err = memoryGrowBody(ins)
			case jit.SiteMemFill:
				err = memoryFill(ins)
			case jit.SiteMemCopy:
				err = memoryCopy(ins)
			default:
				err = ins.IndexSpace.Functions[site.FuncIdx].call(ins)
			}
			ins.nativeTop = prevTop
			ins.FrameStack.Ptr = prevFrames
			if err != nil {
				return err
			}
			for i := nout - 1; i >= 0; i-- {
				ns[argBase+i] = osk.Pop()
			}
			if atomic.LoadUint32(&ins.interruptFlag) != 0 {
				return ErrInterrupted
			}
			ctx.Stack = base + uintptr(fb*8)
			code, entry = ef.compiled.Code, site.Cont
		case jit.StatusUnreachable:
			return ErrUnreachable
		case jit.StatusMemOOB:
			return ErrPtrOutOfBounds
		case jit.StatusConvInvalid:
			return ErrInvalidConversionToInt
		case jit.StatusConvOverflow:
			return ErrIntegerOverflow
		case jit.StatusExhausted:
			return ErrCallStackExhausted
		default:
			return ErrUndefined
		}
	}
}

// sigKey canonicalizes a signature structurally.
func sigKey(t *types.FuncType) string {
	b := make([]byte, 0, len(t.InputTypes)+len(t.ReturnTypes)+1)
	for _, v := range t.InputTypes {
		b = append(b, byte(v))
	}
	b = append(b, 0xff)
	for _, v := range t.ReturnTypes {
		b = append(b, byte(v))
	}
	return string(b)
}

func (ins *Instance) sigID(key string) uint32 {
	if id, ok := ins.sigIDs[key]; ok {
		return id
	}
	id := uint32(len(ins.sigIDs))
	ins.sigIDs[key] = id
	return id
}

// buildNativeIndirect mirrors table 0 for the native call_indirect fast
// path. Only a private table qualifies: an imported or exported table can
// be repopulated by other modules after this instance compiled, and a
// stale mirror would dispatch to the wrong function (a zero entry merely
// falls back to the host, which is always correct).
func (ins *Instance) buildNativeIndirect() {
	if ins.nativeEntries == nil || len(ins.IndexSpace.Tables) == 0 || ins.sigIDs == nil {
		return
	}
	if len(ins.Module.ImportSection) > 0 {
		for _, imp := range ins.Module.ImportSection {
			if imp.Desc != nil && imp.Desc.TableTypePtr != nil {
				return
			}
		}
	}
	for _, exp := range ins.Module.ExportSection {
		if exp.Desc != nil && exp.Desc.Kind == 1 { // table export
			return
		}
	}
	t := ins.IndexSpace.Tables[0]
	m := make([]uint64, 1+2*len(t.Value))
	m[0] = uint64(len(t.Value))
	for i, fn := range t.Value {
		if fn == nil {
			continue
		}
		wf, ok := fn.(*wasmFunc)
		if !ok || wf.compiled == nil || !wf.compiled.NativeABI || wf.owner != ins {
			continue
		}
		m[1+2*i] = uint64(ins.sigID(sigKey(wf.getType())))<<32 |
			uint64(wf.compiled.LocalSlots*8)
		m[1+2*i+1] = uint64(uintptr(unsafe.Pointer(&wf.compiled.Code[0])))
	}
	ins.indirectMirror = m
}

// uniformOpPrice reports the per-opcode price if a toll station charges the
// same for every opcode (the inline meter assumes uniform pricing).
func uniformOpPrice(ts tollstation.TollStation) (uint64, bool) {
	p := ts.GetOpPrice(0x20) // local.get
	for _, op := range []byte{0x00, 0x0b, 0x10, 0x41, 0x6a, 0x28, 0x36, 0x04, 0x0d, 0x0f} {
		if ts.GetOpPrice(expr.OpCode(op)) != p {
			return 0, false
		}
	}
	return p, true
}
