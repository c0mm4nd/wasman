//go:build (darwin || linux) && (arm64 || amd64)

package jit

import (
	"os"
	"testing"
	"unsafe"
)

// compileUnderTest routes every semantic test through the tier selected by
// WASMAN_TEST_TIER: "opt" exercises the optimizing compiler (falling back
// to baseline outside its subset, mirroring the engine's tiering), any
// other value pins the baseline tier.
func compileUnderTest(fd *FuncDesc) (*Compiled, error) {
	if os.Getenv("WASMAN_TEST_TIER") == "opt" {
		if cd, err := CompileOpt(fd); err == nil {
			return cd, nil
		}
	}
	return CompileBaseline(fd)
}

// runFD compiles fd with the tier under test and executes it under a
// minimal host: call-exit sites run as no-ops, memory/globals attach when
// provided. Returns the declared results and the final status (results
// are only meaningful for StatusOK).
func runFD(t *testing.T, fd *FuncDesc, locals, mem []uint64, memBytes int, globals []*uint64) ([]uint64, uint32) {
	t.Helper()
	cd, err := compileUnderTest(fd)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Free(cd.Code) })

	var ctx Ctx
	if len(mem) > 0 {
		ctx.Mem = uintptr(unsafe.Pointer(&mem[0]))
		ctx.MemLen = uint64(memBytes)
	}
	if len(globals) > 0 {
		ctx.Globals = uintptr(unsafe.Pointer(&globals[0]))
	}

	if cd.NativeABI {
		need := fd.NumLocals
		buf := make([]uint64, need+cd.FrameSlots+64)
		copy(buf, locals)
		base := uintptr(unsafe.Pointer(&buf[0]))
		ctx.Stack = base + uintptr(need*8)
		ctx.StackLimit = base + uintptr(len(buf)*8)
		entry := 0
		for i := 0; i < 1000; i++ {
			st := CallAt(cd.Code, entry, &ctx)
			switch st {
			case StatusOK:
				fb := int(uintptr(ctx.Sp)-base) / 8
				return buf[fb : fb+fd.NumRets], st
			case StatusCall: // no-op callee; resume at the continuation
				site := cd.CallSites[uint32(ctx.TrapInfo)]
				ctx.Stack = uintptr(ctx.Sp)
				entry = site.Cont
			default:
				return nil, st
			}
		}
		t.Fatal("no return")
		return nil, 0
	}

	stack := make([]uint64, cd.MaxHeight+8)
	ctx.Stack = uintptr(unsafe.Pointer(&stack[0]))
	if len(locals) > 0 {
		ctx.Locals = uintptr(unsafe.Pointer(&locals[0]))
	}
	entry := 0
	for i := 0; i < 1000; i++ {
		st := CallAt(cd.Code, entry, &ctx)
		switch st {
		case StatusOK:
			return stack[:ctx.Sp], st
		case StatusCall:
			entry = cd.CallSites[uint32(ctx.TrapInfo)].Cont
		default:
			return nil, st
		}
	}
	t.Fatal("no return")
	return nil, 0
}

// runOK is runFD asserting a clean return.
func runOK(t *testing.T, fd *FuncDesc, locals []uint64) []uint64 {
	t.Helper()
	got, st := runFD(t, fd, locals, nil, 0, nil)
	if st != StatusOK {
		t.Fatalf("status %d", st)
	}
	return got
}
