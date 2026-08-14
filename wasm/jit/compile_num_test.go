//go:build (darwin || linux) && (arm64 || amd64)

package jit

import "testing"

// runBin compiles [local.get 0, local.get 1, op] and returns result + status.
func runBin(t *testing.T, op byte, a, b uint64) (uint64, uint32) {
	t.Helper()
	fd := assemble([]ins{
		{0x20, 0, 1}, {0x20, 1, 1}, {op, 0, 0}, {0x0b, 0, 0},
	}, 2, 2, 1)
	cd, err := compileUnderTest(fd)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Free(cd.Code) })
	stack := make([]uint64, cd.MaxHeight+1)
	locals := []uint64{a, b}
	ctx := &Ctx{Stack: ptrOf(stack), Locals: ptrOf(locals)}
	st := Call(cd.Code, ctx)
	if st != StatusOK {
		return 0, st
	}
	return stack[0], st
}

// runUn compiles [local.get 0, op].
func runUn(t *testing.T, op byte, v uint64) uint64 {
	t.Helper()
	fd := assemble([]ins{
		{0x20, 0, 1}, {op, 0, 0}, {0x0b, 0, 0},
	}, 1, 1, 1)
	cd, err := compileUnderTest(fd)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Free(cd.Code) })
	stack := make([]uint64, cd.MaxHeight+1)
	locals := []uint64{v}
	ctx := &Ctx{Stack: ptrOf(stack), Locals: ptrOf(locals)}
	if st := Call(cd.Code, ctx); st != StatusOK {
		t.Fatalf("op %#x(%#x): status %d", op, v, st)
	}
	return stack[0]
}

func TestDivRem(t *testing.T) {
	neg := func(v int64) uint64 { return uint64(v) }
	cases := []struct {
		op   byte
		a, b uint64
		want uint64
	}{
		{0x6d, 7, 2, 3},                                   // i32.div_s
		{0x6d, neg(-7), 2, neg(-3)},                       // truncated toward zero
		{0x6e, 0xfffffffe, 2, 0x7fffffff},                 // i32.div_u (big u32)
		{0x6f, neg(-7), 2, neg(-1)},                       // i32.rem_s sign follows dividend
		{0x6f, 0x80000000, neg(-1), 0},                    // MinInt32 % -1 == 0, no trap
		{0x70, 0xffffffff, 10, 5},                         // i32.rem_u
		{0x7f, neg(-9), 4, neg(-2)},                       // i64.div_s
		{0x80, 0xfffffffffffffffe, 2, 0x7fffffffffffffff}, // i64.div_u
		{0x81, 0x8000000000000000, neg(-1), 0},            // MinInt64 % -1 == 0
		{0x82, 0xffffffffffffffff, 10, 5},                 // i64.rem_u
	}
	for _, tc := range cases {
		got, st := runBin(t, tc.op, tc.a, tc.b)
		if st != StatusOK || got != tc.want {
			t.Fatalf("op %#x(%#x,%#x): got %#x st %d, want %#x", tc.op, tc.a, tc.b, got, st, tc.want)
		}
	}

	// traps: /0 for all four families, MinInt/-1 for div_s
	trapCases := []struct {
		op   byte
		a, b uint64
		want uint32
	}{
		{0x6d, 1, 0, StatusDivZero},
		{0x6e, 1, 0, StatusDivZero},
		{0x6f, 1, 0, StatusDivZero},
		{0x70, 1, 0, StatusDivZero},
		{0x7f, 1, 0, StatusDivZero},
		{0x82, 1, 0, StatusDivZero},
		{0x6d, 0x80000000, neg(-1), StatusIntOverflow},
		{0x7f, 0x8000000000000000, neg(-1), StatusIntOverflow},
	}
	for _, tc := range trapCases {
		if _, st := runBin(t, tc.op, tc.a, tc.b); st != tc.want {
			t.Fatalf("op %#x(%#x,%#x): status %d, want %d", tc.op, tc.a, tc.b, st, tc.want)
		}
	}
}

func TestBitCounts(t *testing.T) {
	cases := []struct {
		op   byte
		v    uint64
		want uint64
	}{
		{0x67, 1, 31}, {0x67, 0x80000000, 0}, {0x67, 0, 32}, // i32.clz
		{0x68, 0x80000000, 31}, {0x68, 2, 1}, {0x68, 0, 32}, // i32.ctz
		{0x69, 0xf0f0f0f0, 16}, {0x69, 0, 0}, // i32.popcnt
		{0x79, 1, 63}, {0x79, 0, 64}, // i64.clz
		{0x7a, 0x8000000000000000, 63}, {0x7a, 0, 64}, // i64.ctz
		{0x7b, 0xffffffffffffffff, 64}, {0x7b, 0x8000000000000001, 2}, // i64.popcnt
	}
	for _, tc := range cases {
		if got := runUn(t, tc.op, tc.v); got != tc.want {
			t.Fatalf("op %#x(%#x): got %d, want %d", tc.op, tc.v, got, tc.want)
		}
	}
}

func TestRotates(t *testing.T) {
	cases := []struct {
		op   byte
		a, b uint64
		want uint64
	}{
		{0x77, 0x80000001, 1, 0x00000003},  // i32.rotl
		{0x78, 0x00000003, 1, 0x80000001},  // i32.rotr
		{0x77, 0xdeadbeef, 32, 0xdeadbeef}, // full rotation
		{0x89, 0x8000000000000001, 1, 3},   // i64.rotl
		{0x8a, 3, 1, 0x8000000000000001},   // i64.rotr
	}
	for _, tc := range cases {
		got, st := runBin(t, tc.op, tc.a, tc.b)
		if st != StatusOK || got != tc.want {
			t.Fatalf("op %#x(%#x,%#x): got %#x, want %#x", tc.op, tc.a, tc.b, got, tc.want)
		}
	}
}

func TestSignExtensionOps(t *testing.T) {
	cases := []struct {
		op   byte
		v    uint64
		want uint64
	}{
		{0xc0, 0x80, 0xffffff80}, // i32.extend8_s (zero to 64)
		{0xc0, 0x7f, 0x7f},
		{0xc1, 0x8000, 0xffff8000},             // i32.extend16_s
		{0xc2, 0x80, 0xffffffffffffff80},       // i64.extend8_s
		{0xc3, 0x8000, 0xffffffffffff8000},     // i64.extend16_s
		{0xc4, 0x80000000, 0xffffffff80000000}, // i64.extend32_s
	}
	for _, tc := range cases {
		if got := runUn(t, tc.op, tc.v); got != tc.want {
			t.Fatalf("op %#x(%#x): got %#x, want %#x", tc.op, tc.v, got, tc.want)
		}
	}
}
