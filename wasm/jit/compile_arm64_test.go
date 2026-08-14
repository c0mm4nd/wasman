//go:build (darwin || linux) && arm64

package jit

import (
	"testing"
	"unsafe"
)

// ins is one instruction for the test assembler: opcode, immediate value and
// how many bytes the immediate occupies in the encoded body.
type ins struct {
	op     byte
	imm    uint64
	immLen int
}

// assemble builds a FuncDesc the way the engine's loader would: byte-encoded
// body plus Imms/PcEnd side tables indexed by opcode position.
func assemble(code []ins, numLocals, numParams, numRets int) *FuncDesc {
	var body []byte
	var offs []int
	for _, i := range code {
		offs = append(offs, len(body))
		body = append(body, i.op)
		for k := 0; k < i.immLen; k++ { // placeholder bytes; Imms carries the value
			body = append(body, 0)
		}
	}
	imms := make([]uint64, len(body))
	pcEnd := make([]uint32, len(body))
	for n, i := range code {
		if i.immLen > 0 {
			imms[offs[n]] = i.imm
			pcEnd[offs[n]] = uint32(offs[n] + i.immLen)
		}
	}
	return &FuncDesc{Body: body, Imms: imms, PcEnd: pcEnd,
		NumLocals: numLocals, NumParams: numParams, NumRets: numRets}
}

func runCompiled(t *testing.T, cd *Compiled, locals []uint64) []uint64 {
	t.Helper()
	stack := make([]uint64, cd.MaxHeight+1)
	ctx := &Ctx{Stack: uintptr(unsafe.Pointer(&stack[0]))}
	if len(locals) > 0 {
		ctx.Locals = uintptr(unsafe.Pointer(&locals[0]))
	}
	if st := Call(cd.Code, ctx); st != StatusOK {
		t.Fatalf("status = %d", st)
	}
	return stack[:ctx.Sp]
}

// TestStraightLineI32 compiles ((a+b)*3 ^ b) >>u 1 and checks bit-exact
// parity with the interpreter's stack representation.
func TestStraightLineI32(t *testing.T) {
	fd := assemble([]ins{
		{0x20, 0, 1}, // local.get 0
		{0x20, 1, 1}, // local.get 1
		{0x6a, 0, 0}, // i32.add
		{0x41, 3, 1}, // i32.const 3
		{0x6c, 0, 0}, // i32.mul
		{0x20, 1, 1}, // local.get 1
		{0x73, 0, 0}, // i32.xor
		{0x41, 1, 1}, // i32.const 1
		{0x76, 0, 0}, // i32.shr_u
		{0x0b, 0, 0}, // end
	}, 2, 2, 1)
	cd, err := Compile(fd)
	if err != nil {
		t.Fatal(err)
	}
	defer Free(cd.Code)

	got := runCompiled(t, cd, []uint64{5, 7})
	if len(got) != 1 || got[0] != 17 { // ((5+7)*3 ^ 7) >> 1
		t.Fatalf("got %v", got)
	}
}

// TestSignExtension checks i32 results are sign-extended exactly like the
// interpreter (0 - 1 must sit on the stack as 0xffffffffffffffff).
func TestSignExtension(t *testing.T) {
	fd := assemble([]ins{
		{0x41, 0, 1}, // i32.const 0
		{0x41, 1, 1}, // i32.const 1
		{0x6b, 0, 0}, // i32.sub
		{0x0b, 0, 0},
	}, 0, 0, 1)
	cd, err := Compile(fd)
	if err != nil {
		t.Fatal(err)
	}
	defer Free(cd.Code)
	got := runCompiled(t, cd, nil)
	if got[0] != 0xffffffffffffffff {
		t.Fatalf("got %#x, want sign-extended -1", got[0])
	}

	// and zero-extension for the logical group: -1 & -1 keeps only 32 bits? no —
	// the interpreter zero-extends and/or/xor results: (-1) ^ 0 => 0xffffffff
	fd2 := assemble([]ins{
		{0x41, 0xffffffffffffffff, 1}, // i32.const -1 (pre-decoded bits)
		{0x41, 0, 1},                  // i32.const 0
		{0x73, 0, 0},                  // i32.xor
		{0x0b, 0, 0},
	}, 0, 0, 1)
	cd2, err := Compile(fd2)
	if err != nil {
		t.Fatal(err)
	}
	defer Free(cd2.Code)
	if got := runCompiled(t, cd2, nil); got[0] != 0xffffffff {
		t.Fatalf("got %#x, want zero-extended 0xffffffff", got[0])
	}
}

// TestI64AndLocals covers i64 arithmetic, shifts and local.set/tee.
func TestI64AndLocals(t *testing.T) {
	// l2 = a * b; return (l2 << 8) - a
	fd := assemble([]ins{
		{0x20, 0, 1}, // local.get 0
		{0x20, 1, 1}, // local.get 1
		{0x7e, 0, 0}, // i64.mul
		{0x21, 2, 1}, // local.set 2
		{0x20, 2, 1}, // local.get 2
		{0x42, 8, 1}, // i64.const 8
		{0x86, 0, 0}, // i64.shl
		{0x20, 0, 1}, // local.get 0
		{0x7d, 0, 0}, // i64.sub
		{0x0b, 0, 0},
	}, 3, 2, 1)
	cd, err := Compile(fd)
	if err != nil {
		t.Fatal(err)
	}
	defer Free(cd.Code)
	a, b := uint64(123456789), uint64(987654321)
	want := (a*b)<<8 - a
	if got := runCompiled(t, cd, []uint64{a, b, 0}); got[0] != want {
		t.Fatalf("got %d, want %d", got[0], want)
	}
}

// TestUnsupportedFallsBack ensures unknown opcodes report ErrUnsupported.
func TestUnsupportedFallsBack(t *testing.T) {
	fd := assemble([]ins{{0x10, 0, 1}, {0x0b, 0, 0}}, 0, 0, 0) // call
	if _, err := Compile(fd); err == nil {
		t.Fatal("expected ErrUnsupported")
	}
}
