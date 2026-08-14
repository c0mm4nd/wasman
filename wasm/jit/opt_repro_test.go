//go:build (darwin || linux) && arm64

package jit

import (
	"testing"
	"unsafe"
)

// runWithHost drives a compiled function through the host-exit protocol the
// way the engine does (call sites are no-ops here).
func runWithHost(t *testing.T, cd *Compiled, locals []uint64) []uint64 {
	t.Helper()
	stack := make([]uint64, cd.MaxHeight+8)
	ctx := &Ctx{Stack: uintptr(unsafe.Pointer(&stack[0]))}
	if len(locals) > 0 {
		ctx.Locals = ptrOf(locals)
	}
	entry := 0
	for i := 0; i < 100; i++ {
		st := CallAt(cd.Code, entry, ctx)
		switch st {
		case StatusOK:
			return stack[:ctx.Sp]
		case StatusCall:
			entry = cd.CallSites[ctx.TrapInfo].Cont
		default:
			t.Fatalf("status %d", st)
		}
	}
	t.Fatal("no return")
	return nil
}

// TestSpecAsBlockMid replicates br_if.wast as-block-mid byte for byte:
// void block, dummy call, br_if on a zeroed local, early return in the
// block, fallthrough constant after it.
func TestSpecAsBlockMid(t *testing.T) {
	fd := assemble([]ins{
		{0x02, blk(0, 0), 1}, // block (void)
		{0x10, 0, 1},         // call 0
		{0x20, 0, 1},         // local.get 0 (declared, zero)
		{0x0d, 0, 1},         // br_if 0
		{0x41, 2, 1},         // i32.const 2
		{0x0f, 0, 0},         // return
		{0x0b, 0, 0},         // end block
		{0x41, 3, 1},         // i32.const 3
		{0x0b, 0, 0},         // end func
	}, 1, 0, 1)
	fd.FuncSigs = []FuncSig{{In: 0, Out: 0}}
	cd, err := CompileOpt(fd)
	if err != nil {
		t.Fatal(err)
	}
	defer Free(cd.Code)
	got := runWithHost(t, cd, []uint64{0})
	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("got %v, want [2]", got)
	}
}

// TestBrIfAfterCall mirrors br_if.wast as-block-mid: a call exit followed by
// a constant-true br_if carrying a value.
func TestBrIfAfterCall(t *testing.T) {
	fd := assemble([]ins{
		{0x02, blk(0, 1), 1}, // block (result i32)
		{0x10, 0, 1},         // call 0 (() -> ())
		{0x41, 2, 1},         // i32.const 2
		{0x41, 1, 1},         // i32.const 1
		{0x0d, 0, 1},         // br_if 0
		{0x1a, 0, 0},         // drop
		{0x41, 3, 1},         // i32.const 3
		{0x0b, 0, 0},         // end block
		{0x0b, 0, 0},         // end func
	}, 0, 0, 1)
	fd.FuncSigs = []FuncSig{{In: 0, Out: 0}}
	cd, err := CompileOpt(fd)
	if err != nil {
		t.Fatal(err)
	}
	defer Free(cd.Code)
	got := runWithHost(t, cd, nil)
	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("got %v, want [2]", got)
	}
}
