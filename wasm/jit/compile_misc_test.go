//go:build (darwin || linux) && (arm64 || amd64)

package jit

import (
	"testing"
	"unsafe"
)

// TestGlobals covers global.get/set through the *uint64 cell indirection.
func TestGlobals(t *testing.T) {
	g0, g1 := uint64(41), uint64(0)
	cells := []*uint64{&g0, &g1}
	// g1 = g0 + 1; return g1
	fd := assemble([]ins{
		{0x23, 0, 1}, // global.get 0
		{0x41, 1, 1}, // i32.const 1
		{0x7c, 0, 0}, // i64.add
		{0x24, 1, 1}, // global.set 1
		{0x23, 1, 1}, // global.get 1
		{0x0b, 0, 0},
	}, 0, 0, 1)
	cd, err := CompileBaseline(fd)
	if err != nil {
		t.Fatal(err)
	}
	defer Free(cd.Code)
	stack := make([]uint64, cd.MaxHeight+1)
	ctx := &Ctx{Stack: ptrOf(stack), Globals: uintptr(unsafe.Pointer(&cells[0]))}
	if st := Call(cd.Code, ctx); st != StatusOK {
		t.Fatalf("status %d", st)
	}
	if stack[0] != 42 || g1 != 42 {
		t.Fatalf("stack %d, g1 %d, want 42", stack[0], g1)
	}
}

// TestBrTable covers indexed dispatch including the default label.
func TestBrTable(t *testing.T) {
	// block(r=1){ block{ block{ br_table [1,0] def=1 (local0) ; }
	//   push 10; br 1 } push 20; br 0 } -> idx 0 => depth1 => 20?  layout below:
	fd := assemble([]ins{
		{0x02, blk(0, 1), 1}, // outer block (result i32)
		{0x02, blk(0, 0), 1}, // mid
		{0x02, blk(0, 0), 1}, // inner
		{0x20, 0, 1},         // local.get 0
		{0x0e, 0, 1},         // br_table [0,1] default 2 (set via BrTables)
		{0x0b, 0, 0},         // end inner   (idx 0 lands here)
		{0x41, 10, 1},        // push 10
		{0x0c, 1, 1},         // br 1 (to outer end)
		{0x0b, 0, 0},         // end mid     (idx 1 lands here)
		{0x41, 20, 1},        // push 20
		{0x0c, 0, 1},         // br 0
		{0x0b, 0, 0},         // end outer   (default: needs a value -> falls
		{0x0b, 0, 0},         //   through mid/outer? default=2 exits with 30 below)
	}, 1, 1, 1)
	// br_table plan at the 0x0e opcode's PC: targets [0,1], default 1
	var pc0e int
	{ // find the byte offset of the 0x0e opcode in the assembled body
		off := 0
		for _, i := range []int{2, 2, 2, 2} { // three blocks + local.get
			off += i
		}
		pc0e = off
	}
	fd.BrTables = map[int]BrTable{pc0e: {Targets: []uint32{0, 1}, Def: 1}}

	run := func(idx uint64) uint64 {
		cd, err := CompileBaseline(fd)
		if err != nil {
			t.Fatal(err)
		}
		defer Free(cd.Code)
		stack := make([]uint64, cd.MaxHeight+1)
		locals := []uint64{idx}
		ctx := &Ctx{Stack: ptrOf(stack), Locals: ptrOf(locals)}
		if st := Call(cd.Code, ctx); st != StatusOK {
			t.Fatalf("idx %d: status %d", idx, st)
		}
		return stack[0]
	}
	if got := run(0); got != 10 { // depth 0: exits inner, runs "push 10"
		t.Fatalf("idx 0: got %d, want 10", got)
	}
	if got := run(1); got != 20 { // depth 1: exits mid, runs "push 20"
		t.Fatalf("idx 1: got %d, want 20", got)
	}
	if got := run(7); got != 20 { // out of range: default depth 1
		t.Fatalf("idx 7: got %d, want 20", got)
	}
}
