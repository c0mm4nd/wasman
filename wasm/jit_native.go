package wasm

import (
	"unsafe"

	"github.com/c0mm4nd/wasman/wasm/jit"
)

// compileNative translates f with the template JIT. Functions using
// constructs outside the compiled subset keep compiled == nil and run in
// the interpreter; on unsupported platforms jit.Compile rejects everything.
func (ins *Instance) compileNative(f *wasmFunc) {
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
func (ins *Instance) execNative(cd *jit.Compiled, locals []uint64, baseSp int) error {
	os := ins.OperandStack
	if need := baseSp + 1 + cd.MaxHeight; len(os.Values) < need {
		os.Values = append(os.Values, make([]uint64, need-len(os.Values))...)
	}
	var ctx jit.Ctx
	if cd.MaxHeight > 0 {
		ctx.Stack = uintptr(unsafe.Pointer(&os.Values[baseSp+1]))
	}
	if len(locals) > 0 {
		ctx.Locals = uintptr(unsafe.Pointer(&locals[0]))
	}
	if ins.Memory != nil && len(ins.Memory.Value) > 0 {
		ctx.Mem = uintptr(unsafe.Pointer(&ins.Memory.Value[0]))
		ctx.MemLen = uint64(len(ins.Memory.Value))
	}
	if len(ins.Globals) > 0 {
		ctx.Globals = uintptr(unsafe.Pointer(&ins.Globals[0]))
	}
	switch jit.Call(cd.Code, &ctx) {
	case jit.StatusOK:
		os.Ptr = baseSp + int(ctx.Sp)
		return nil
	case jit.StatusUnreachable:
		return ErrUnreachable
	case jit.StatusMemOOB:
		return ErrPtrOutOfBounds
	default: // div-by-zero / overflow trap the same way the interpreter does
		return ErrUndefined
	}
}
