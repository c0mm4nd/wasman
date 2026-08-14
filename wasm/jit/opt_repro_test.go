//go:build (darwin || linux) && arm64

package jit

import (
	"os"
	"testing"
	"unsafe"
)

// runWithHost drives a compiled function through the host-exit protocol
// (call sites are no-ops in the test harness).
func runWithHost(t *testing.T, fd *FuncDesc, locals []uint64) []uint64 {
	t.Helper()
	got, st := runFD(t, fd, locals, nil, 0, nil)
	if st != StatusOK {
		t.Fatalf("status %d", st)
	}
	return got
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
	got := runWithHost(t, fd, []uint64{0})
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
	got := runWithHost(t, fd, nil)
	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("got %v, want [2]", got)
	}
}

// TestNativeSelfCall compiles a recursive countdown with the native-call
// ABI and drives it with a function table: cnt(n) = n==0 ? 0 : cnt(n-1).
func TestNativeSelfCall(t *testing.T) {
	fd := assemble([]ins{
		{0x20, 0, 1},         // local.get 0
		{0x45, 0, 0},         // i32.eqz
		{0x04, blk(0, 0), 1}, // if
		{0x41, 0, 1},         // i32.const 0
		{0x0f, 0, 0},         // return
		{0x0b, 0, 0},         // end if
		{0x20, 0, 1},         // local.get 0
		{0x41, 1, 1},         // i32.const 1
		{0x6b, 0, 0},         // i32.sub
		{0x10, 0, 1},         // call 0 (self)
		{0x0b, 0, 0},         // end
	}, 1, 1, 1)
	fd.FuncSigs = []FuncSig{{In: 1, Out: 1, Locals: 1}}
	fd.NativeFuncs = []bool{true}
	fd.SelfIdx = 0
	cd, err := CompileOpt(fd)
	if err != nil {
		t.Fatal(err)
	}
	defer Free(cd.Code)
	if !cd.NativeABI {
		t.Skip("native ABI not supported here")
	}
	if os.Getenv("WASMAN_OPT_DEBUG") == "1" {
		f, _ := os.Create("/tmp/native.bin")
		f.Write(cd.Code)
		f.Close()
		t.Logf("code %d bytes dumped", len(cd.Code))
	}

	buf := make([]uint64, 4096)
	entries := []uintptr{uintptr(unsafe.Pointer(&cd.Code[0]))}
	base := uintptr(unsafe.Pointer(&buf[0]))
	var ctx Ctx
	buf[0] = 25 // argument
	ctx.Stack = base + uintptr(fd.NumLocals*8)
	ctx.StackLimit = base + uintptr(len(buf)*8)
	ctx.Funcs = uintptr(unsafe.Pointer(&entries[0]))
	if st := Call(cd.Code, &ctx); st != StatusOK {
		t.Fatalf("status %d", st)
	}
	fb := int(uintptr(ctx.Sp)-base) / 8
	if buf[fb] != 0 {
		t.Fatalf("got %d, want 0", buf[fb])
	}
}
