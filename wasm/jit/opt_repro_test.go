//go:build (darwin || linux) && (arm64 || amd64)

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

// TestWideMulSpilledDst reproduces the reviewed defect: when the
// destination-pointer local is memory-homed (register pressure), the
// multiply intrinsics must not reload it through the borrowed locals base.
func TestWideMulSpilledDst(t *testing.T) {
	// 16 params; params 3..15 stay live across the mul and are summed
	// afterwards together with param 0 (the destination pointer), forcing
	// long ranges that exceed both register pools.
	var code []ins
	code = append(code,
		ins{0x20, 0, 1}, // dst ptr
		ins{0x20, 1, 1}, // a ptr
		ins{0x20, 2, 1}, // b ptr
		ins{0x10, 0, 1}, // call 0 = u128.mul (tagged)
	)
	for i := uint64(1); i < 16; i++ {
		code = append(code, ins{0x20, i, 1})
		if i > 1 {
			code = append(code, ins{0x7c, 0, 0}) // i64.add
		}
	}
	// the destination pointer's local is used LAST: its interval reaches
	// farthest, so linear scan evicts it to its memory home under pressure
	code = append(code, ins{0x20, 0, 1}, ins{0x7c, 0, 0}, ins{0x0b, 0, 0})
	fd := assemble(code, 16, 16, 1)
	fd.FuncSigs = []FuncSig{{In: 3, Out: 0, Locals: 3}}
	fd.WideOps = []uint16{WideOpID(WideMul, false)}
	cd, err := CompileOpt(fd)
	if err != nil {
		t.Fatal(err)
	}
	defer Free(cd.Code)

	mem := make([]uint64, 32) // 256 bytes of linear memory
	U := func(lo, hi uint64, at int) {
		mem[at/8] = lo
		mem[at/8+1] = hi
	}
	U(3, 5, 16)  // a at byte 16
	U(7, 11, 32) // b at byte 32

	locals := make([]uint64, 16)
	locals[0], locals[1], locals[2] = 0, 16, 32 // dst at byte 0
	for i := 3; i < 16; i++ {
		locals[i] = uint64(i)
	}
	buf := make([]uint64, 16+cd.FrameSlots+64)
	copy(buf, locals)
	base := uintptr(unsafe.Pointer(&buf[0]))
	var ctx Ctx
	ctx.Stack = base + uintptr(16*8)
	ctx.StackLimit = base + uintptr(len(buf)*8)
	ctx.Mem = uintptr(unsafe.Pointer(&mem[0]))
	ctx.MemLen = 256
	if st := Call(cd.Code, &ctx); st != StatusOK {
		t.Fatalf("status %d", st)
	}
	// 3*7 = 21 low; hi = umulh(3,7)=0 + 3*11 + 5*7 = 68
	if mem[0] != 21 || mem[1] != 68 {
		t.Fatalf("mul result {%d %d}, want {21 68}", mem[0], mem[1])
	}
}

// TestWideMul256SpilledDst is the 256-bit variant of the same defect.
func TestWideMul256SpilledDst(t *testing.T) {
	var code []ins
	code = append(code,
		ins{0x20, 0, 1}, ins{0x20, 1, 1}, ins{0x20, 2, 1}, ins{0x10, 0, 1})
	for i := uint64(1); i < 16; i++ {
		code = append(code, ins{0x20, i, 1})
		if i > 1 {
			code = append(code, ins{0x7c, 0, 0})
		}
	}
	code = append(code, ins{0x20, 0, 1}, ins{0x7c, 0, 0}, ins{0x0b, 0, 0})
	fd := assemble(code, 16, 16, 1)
	fd.FuncSigs = []FuncSig{{In: 3, Out: 0, Locals: 3}}
	fd.WideOps = []uint16{WideOpID(WideMul, true)}
	cd, err := CompileOpt(fd)
	if err != nil {
		t.Fatal(err)
	}
	defer Free(cd.Code)

	mem := make([]uint64, 32)
	mem[8], mem[9], mem[10], mem[11] = 3, 5, 0, 0    // a at byte 64
	mem[16], mem[17], mem[18], mem[19] = 7, 11, 0, 0 // b at byte 128
	locals := make([]uint64, 16)
	locals[0], locals[1], locals[2] = 0, 64, 128
	for i := 3; i < 16; i++ {
		locals[i] = uint64(i)
	}
	buf := make([]uint64, 16+cd.FrameSlots+64)
	copy(buf, locals)
	base := uintptr(unsafe.Pointer(&buf[0]))
	var ctx Ctx
	ctx.Stack = base + uintptr(16*8)
	ctx.StackLimit = base + uintptr(len(buf)*8)
	ctx.Mem = uintptr(unsafe.Pointer(&mem[0]))
	ctx.MemLen = 256
	if st := Call(cd.Code, &ctx); st != StatusOK {
		t.Fatalf("status %d", st)
	}
	// (3 + 5*2^64) * (7 + 11*2^64) = 21, 33+35=68, 55, 0
	if mem[0] != 21 || mem[1] != 68 || mem[2] != 55 || mem[3] != 0 {
		t.Fatalf("mul256 result %v, want [21 68 55 0]", mem[0:4])
	}
}
