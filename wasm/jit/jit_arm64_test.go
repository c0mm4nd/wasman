//go:build (darwin || linux) && arm64

package jit

import (
	"testing"
	"unsafe"
)

// TestSmoke generates "push 42; push 0xdeadbeefcafe" natively, runs it via
// the trampoline and checks the whole pipeline (encoder, mmap, ABI).
func TestSmoke(t *testing.T) {
	var a Asm
	a.Prologue()
	a.MovImm64(RegT0, 42)
	a.PushReg(RegT0)
	a.MovImm64(RegT0, 0xdeadbeefcafe)
	a.PushReg(RegT0)
	a.Epilogue(StatusOK)

	code, err := AllocExec(a.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	defer Free(code)

	stack := make([]uint64, 16)
	ctx := &Ctx{Stack: uintptr(unsafe.Pointer(&stack[0])), Sp: 0}
	if st := Call(code, ctx); st != StatusOK {
		t.Fatalf("status = %d", st)
	}
	if ctx.Sp != 2 || stack[0] != 42 || stack[1] != 0xdeadbeefcafe {
		t.Fatalf("sp=%d stack=%v", ctx.Sp, stack[:3])
	}
}
