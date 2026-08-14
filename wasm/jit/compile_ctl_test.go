//go:build (darwin || linux) && (arm64 || amd64)

package jit

import "testing"

// blk packs block param/result counts the way the engine pre-decodes them.
func blk(params, results int) uint64 { return uint64(params)<<32 | uint64(results) }

func compileRun(t *testing.T, fd *FuncDesc, locals []uint64) []uint64 {
	t.Helper()
	cd, err := CompileBaseline(fd)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Free(cd.Code) })
	return runCompiled(t, cd, locals)
}

// TestSumLoop compiles the canonical block/loop/br_if/br summation.
func TestSumLoop(t *testing.T) {
	fd := assemble([]ins{
		{0x02, blk(0, 0), 1}, // block
		{0x03, blk(0, 0), 1}, // loop
		{0x20, 0, 1},         // local.get 0 (n)
		{0x45, 0, 0},         // i32.eqz
		{0x0d, 1, 1},         // br_if 1 (exit)
		{0x20, 1, 1},         // local.get 1 (acc)
		{0x20, 0, 1},         // local.get 0
		{0x6a, 0, 0},         // i32.add
		{0x21, 1, 1},         // local.set 1
		{0x20, 0, 1},         // local.get 0
		{0x41, 1, 1},         // i32.const 1
		{0x6b, 0, 0},         // i32.sub
		{0x21, 0, 1},         // local.set 0
		{0x0c, 0, 1},         // br 0 (continue)
		{0x0b, 0, 0},         // end loop
		{0x0b, 0, 0},         // end block
		{0x20, 1, 1},         // local.get 1
		{0x0b, 0, 0},         // end func
	}, 2, 1, 1)
	got := compileRun(t, fd, []uint64{100, 0})
	if len(got) != 1 || got[0] != 5050 {
		t.Fatalf("got %v, want [5050]", got)
	}
}

// TestIfElse compiles min(a,b) via if/else with a result value.
func TestIfElse(t *testing.T) {
	fd := assemble([]ins{
		{0x20, 0, 1},         // local.get 0
		{0x20, 1, 1},         // local.get 1
		{0x49, 0, 0},         // i32.lt_u
		{0x04, blk(0, 1), 1}, // if (result i32)
		{0x20, 0, 1},
		{0x05, 0, 0}, // else
		{0x20, 1, 1},
		{0x0b, 0, 0}, // end if
		{0x0b, 0, 0}, // end func
	}, 2, 2, 1)
	if got := compileRun(t, fd, []uint64{3, 9}); got[0] != 3 {
		t.Fatalf("min(3,9) = %v", got)
	}
	if got := compileRun(t, fd, []uint64{9, 3}); got[0] != 3 {
		t.Fatalf("min(9,3) = %v", got)
	}
}

// TestBrOutOfNestedBlock checks result moves and unreachable-code skipping.
func TestBrOutOfNestedBlock(t *testing.T) {
	fd := assemble([]ins{
		{0x02, blk(0, 1), 1}, // block (result i32)
		{0x02, blk(0, 0), 1}, // block
		{0x41, 7, 1},         // i32.const 7
		{0x0c, 1, 1},         // br 1 (carries the 7 out)
		{0x0b, 0, 0},         // end inner
		{0x41, 9, 1},         // i32.const 9 (dead)
		{0x0c, 0, 1},         // br 0 (dead)
		{0x0b, 0, 0},         // end outer
		{0x0b, 0, 0},         // end func
	}, 0, 0, 1)
	if got := compileRun(t, fd, nil); got[0] != 7 {
		t.Fatalf("got %v, want [7]", got)
	}
}

// TestReturnAndSelect covers early return and select.
func TestReturnAndSelect(t *testing.T) {
	// if (local0) { return 11 }; select(20, 30, local1)
	fd := assemble([]ins{
		{0x20, 0, 1},
		{0x04, blk(0, 0), 1}, // if
		{0x41, 11, 1},
		{0x0f, 0, 0}, // return
		{0x0b, 0, 0}, // end if
		{0x41, 20, 1},
		{0x41, 30, 1},
		{0x20, 1, 1},
		{0x1b, 0, 0}, // select
		{0x0b, 0, 0},
	}, 2, 2, 1)
	if got := compileRun(t, fd, []uint64{1, 0}); got[0] != 11 {
		t.Fatalf("return path: %v", got)
	}
	if got := compileRun(t, fd, []uint64{0, 1}); got[0] != 20 {
		t.Fatalf("select true: %v", got)
	}
	if got := compileRun(t, fd, []uint64{0, 0}); got[0] != 30 {
		t.Fatalf("select false: %v", got)
	}
}

// TestComparisons spot-checks signed vs unsigned compares on both widths.
func TestComparisons(t *testing.T) {
	cases := []struct {
		op   byte
		a, b uint64
		want uint64
	}{
		{0x48, 0xffffffffffffffff, 1, 1}, // i32.lt_s: -1 < 1
		{0x49, 0xffffffff, 1, 0},         // i32.lt_u: max > 1
		{0x4e, 5, 5, 1},                  // i32.ge_s
		{0x53, 0xffffffffffffffff, 1, 1}, // i64.lt_s
		{0x54, 0xffffffffffffffff, 1, 0}, // i64.lt_u
		{0x5a, 7, 9, 0},                  // i64.ge_u
	}
	for _, tc := range cases {
		fd := assemble([]ins{
			{0x20, 0, 1}, {0x20, 1, 1}, {tc.op, 0, 0}, {0x0b, 0, 0},
		}, 2, 2, 1)
		if got := compileRun(t, fd, []uint64{tc.a, tc.b}); got[0] != tc.want {
			t.Fatalf("op %#x (%#x,%#x): got %v, want %d", tc.op, tc.a, tc.b, got, tc.want)
		}
	}
}

// TestUnreachableTrap checks the trap status path.
func TestUnreachableTrap(t *testing.T) {
	fd := assemble([]ins{{0x00, 0, 0}, {0x0b, 0, 0}}, 0, 0, 0)
	cd, err := CompileBaseline(fd)
	if err != nil {
		t.Fatal(err)
	}
	defer Free(cd.Code)
	stack := make([]uint64, 4)
	ctx := &Ctx{Stack: ptrOf(stack)}
	if st := Call(cd.Code, ctx); st != StatusUnreachable {
		t.Fatalf("status = %d, want unreachable", st)
	}
}
