//go:build (darwin || linux) && (arm64 || amd64)

package jit

import (
	"math"
	"testing"
)

func f64b(v float64) uint64 { return math.Float64bits(v) }
func f32b(v float32) uint64 { return uint64(math.Float32bits(v)) }

// i64u forces a runtime int64->uint64 conversion (avoids constant overflow).
func i64u(v int64) uint64 { return v2u(v) }

func v2u(v int64) uint64 { return uint64(v) }

func TestFloatArith(t *testing.T) {
	cases := []struct {
		op   byte
		a, b uint64
		want uint64
	}{
		{0xa0, f64b(1.5), f64b(2.25), f64b(3.75)},   // f64.add
		{0xa1, f64b(1.5), f64b(2.25), f64b(-0.75)},  // f64.sub
		{0xa2, f64b(3), f64b(-2.5), f64b(-7.5)},     // f64.mul
		{0xa3, f64b(1), f64b(8), f64b(0.125)},       // f64.div
		{0xa3, f64b(1), f64b(0), f64b(math.Inf(1))}, // div by zero -> inf
		{0x92, f32b(1.5), f32b(2.25), f32b(3.75)},   // f32.add
		{0x95, f32b(1), f32b(0), f32b(float32(math.Inf(1)))},
		{0xa4, f64b(math.Copysign(0, -1)), f64b(0.0), f64b(math.Copysign(0, -1))}, // min(-0,+0)
		{0xa4, f64b(1), f64b(math.NaN()), 0x7ff8000000000000},                     // canonical NaN
		{0xa5, f64b(3), f64b(7), f64b(7)},                                         // f64.max
		{0x96, f32b(3), f32b(7), f32b(3)},                                         // f32.min
		{0x97, f32b(1), f32b(float32(math.NaN())), 0x7fc00000},                    // f32 canonical
		{0x98, f32b(3), f32b(-1), f32b(-3)},                                       // f32.copysign
		{0xa6, f64b(-3), f64b(1), f64b(3)},                                        // f64.copysign
	}
	for _, tc := range cases {
		got, st := runBin(t, tc.op, tc.a, tc.b)
		if st != StatusOK || got != tc.want {
			t.Fatalf("op %#x(%#x,%#x): got %#x st %d, want %#x", tc.op, tc.a, tc.b, got, st, tc.want)
		}
	}
}

func TestFloatUnary(t *testing.T) {
	cases := []struct {
		op   byte
		v    uint64
		want uint64
	}{
		{0x99, f64b(-3.5), f64b(3.5)}, // f64.abs
		{0x9a, f64b(3.5), f64b(-3.5)}, // f64.neg
		{0x8b, f32b(-2), f32b(2)},     // f32.abs
		{0x8c, f32b(-2), f32b(2)},     // f32.neg
		{0x9b, f64b(1.2), f64b(2)},    // f64.ceil
		{0x9c, f64b(-1.2), f64b(-2)},  // f64.floor
		{0x9d, f64b(-1.7), f64b(-1)},  // f64.trunc
		{0x9e, f64b(2.5), f64b(2)},    // f64.nearest (ties to even)
		{0x9f, f64b(9), f64b(3)},      // f64.sqrt
		{0x91, f32b(16), f32b(4)},     // f32.sqrt
		{0x8d, f32b(1.1), f32b(2)},    // f32.ceil
	}
	for _, tc := range cases {
		if got := runUn(t, tc.op, tc.v); got != tc.want {
			t.Fatalf("op %#x(%#x): got %#x, want %#x", tc.op, tc.v, got, tc.want)
		}
	}
}

func TestFloatCompare(t *testing.T) {
	nan := f64b(math.NaN())
	cases := []struct {
		op   byte
		a, b uint64
		want uint64
	}{
		{0x61, f64b(2), f64b(2), 1}, {0x61, f64b(2), nan, 0}, // f64.eq
		{0x62, f64b(2), nan, 1}, {0x62, f64b(2), f64b(2), 0}, // f64.ne (NaN -> true)
		{0x63, f64b(1), f64b(2), 1}, {0x63, nan, f64b(2), 0}, // f64.lt
		{0x64, f64b(3), f64b(2), 1},                      // f64.gt
		{0x65, f64b(2), f64b(2), 1}, {0x65, nan, nan, 0}, // f64.le
		{0x66, f64b(2), f64b(3), 0},  // f64.ge
		{0x5b, f32b(5), f32b(5), 1},  // f32.eq
		{0x5d, f32b(-1), f32b(1), 1}, // f32.lt
	}
	for _, tc := range cases {
		got, st := runBin(t, tc.op, tc.a, tc.b)
		if st != StatusOK || got != tc.want {
			t.Fatalf("op %#x(%#x,%#x): got %d, want %d", tc.op, tc.a, tc.b, got, tc.want)
		}
	}
}

// negBillion returns float64(float32(-1e9)) without constant folding.
func negBillion() float64 {
	v := float32(-1e9)
	return float64(v)
}

func TestFloatConvert(t *testing.T) {
	cases := []struct {
		op   byte
		v    uint64
		want uint64
	}{
		{0xa8, f32b(-3.7), 0xfffffffd},                                  // i32.trunc_f32_s -> zero-extended -3
		{0xa9, f32b(3.9), 3},                                            // i32.trunc_f32_u
		{0xaa, f64b(-2147483648.0), 0x80000000},                         // i32.trunc_f64_s at the low edge
		{0xab, f64b(4294967295.0), 0xffffffff},                          // i32.trunc_f64_u at the high edge
		{0xae, f32b(-1e9), i64u(int64(negBillion()))},                   // i64.trunc_f32_s
		{0xb1, f64b(1.8446744073709550e19), 0xfffffffffffff800},         // i64.trunc_f64_u big
		{0xb2, 7, f32b(7)},                                              // f32.convert_i32_s
		{0xb3, 0xffffffff, f32b(4294967295.0)},                          // f32.convert_i32_u
		{0xb7, i64u(-5), f64b(-5)},                                      // f64.convert_i32_s
		{0xb8, 0xfffffffe, f64b(4294967294.0)},                          // f64.convert_i32_u
		{0xb9, i64u(-5), f64b(-5)},                                      // f64.convert_i64_s
		{0xba, 0xffffffffffffffff, f64b(1.8446744073709552e19)},         // f64.convert_i64_u
		{0xb4, 1 << 40, f32b(float32(1 << 40))},                         // f32.convert_i64_s
		{0xb5, 0x8000000000000001, f32b(float32(9.223372036854776e18))}, // f32.convert_i64_u rounds
		{0xb6, f64b(1.5), f32b(1.5)},                                    // f32.demote_f64
		{0xbb, f32b(1.5), f64b(1.5)},                                    // f64.promote_f32
		{0xbc, f32b(1.5), f32b(1.5)},                                    // reinterpret: bits unchanged
		{0xbf, f64b(2.5), f64b(2.5)},
	}
	for _, tc := range cases {
		if got := runUn(t, tc.op, tc.v); got != tc.want {
			t.Fatalf("op %#x(%#x): got %#x, want %#x", tc.op, tc.v, got, tc.want)
		}
	}
}

func TestTruncTraps(t *testing.T) {
	nan32, nan64 := f32b(float32(math.NaN())), f64b(math.NaN())
	cases := []struct {
		op   byte
		v    uint64
		want uint32
	}{
		{0xa8, nan32, StatusConvInvalid},
		{0xaa, nan64, StatusConvInvalid},
		{0xb0, nan64, StatusConvInvalid},
		{0xa8, f32b(2147483648.0), StatusConvOverflow},  // 2^31 out of i32_s
		{0xaa, f64b(-2147483649.0), StatusConvOverflow}, // below the window
		{0xa9, f32b(-1.0), StatusConvOverflow},          // negative into _u
		{0xab, f64b(4294967296.0), StatusConvOverflow},
		{0xae, f32b(1e19), StatusConvOverflow},
		{0xaf, f32b(float32(math.Inf(1))), StatusConvOverflow},
		{0xb1, f64b(1.9e19), StatusConvOverflow},
	}
	for _, tc := range cases {
		fd := assemble([]ins{
			{0x20, 0, 1}, {tc.op, 0, 0}, {0x1a, 0, 0}, {0x0b, 0, 0},
		}, 1, 1, 0)
		cd, err := Compile(fd)
		if err != nil {
			t.Fatal(err)
		}
		stack := make([]uint64, cd.MaxHeight+1)
		locals := []uint64{tc.v}
		ctx := &Ctx{Stack: ptrOf(stack), Locals: ptrOf(locals)}
		if st := Call(cd.Code, ctx); st != tc.want {
			t.Fatalf("op %#x(%#x): status %d, want %d", tc.op, tc.v, st, tc.want)
		}
		Free(cd.Code)
	}

	// small negative into _u that truncates to -0: passes (trunc(-0.9) == -0)
	got := runUn(t, 0xa9, f32b(-0.9))
	if got != 0 {
		t.Fatalf("trunc_u(-0.9): got %d, want 0", got)
	}
}

// truncSat assembles [local.get 0, 0xfc sub, end] with the misc immediate.
func truncSat(t *testing.T, sub byte, v uint64) uint64 {
	t.Helper()
	fd := assemble([]ins{
		{0x20, 0, 1}, {0xfc, uint64(sub), 1}, {0x0b, 0, 0},
	}, 1, 1, 1)
	cd, err := Compile(fd)
	if err != nil {
		t.Fatal(err)
	}
	defer Free(cd.Code)
	stack := make([]uint64, cd.MaxHeight+1)
	locals := []uint64{v}
	ctx := &Ctx{Stack: ptrOf(stack), Locals: ptrOf(locals)}
	if st := Call(cd.Code, ctx); st != StatusOK {
		t.Fatalf("sub %d(%#x): status %d", sub, v, st)
	}
	return stack[0]
}

func TestTruncSat(t *testing.T) {
	nan32, nan64 := f32b(float32(math.NaN())), f64b(math.NaN())
	cases := []struct {
		sub  byte
		v    uint64
		want uint64
	}{
		{0, nan32, 0},                // i32.trunc_sat_f32_s NaN
		{0, f32b(-3.7), 0xfffffffd},  // in range, zero-extended
		{0, f32b(1e10), 0x7fffffff},  // clamp max
		{0, f32b(-1e10), 0x80000000}, // clamp min
		{1, f32b(-5), 0},             // _u clamps negatives to 0
		{1, f32b(1e10), 0xffffffff},
		{2, f64b(2147483648.0), 0x7fffffff}, // f64_s clamp at 2^31
		{3, f64b(4294967295.0), 0xffffffff},
		{4, f32b(float32(math.Inf(-1))), 0x8000000000000000}, // i64 -inf
		{5, f32b(1e30), 0xffffffffffffffff},
		{6, nan64, 0},
		{6, f64b(-7.9), i64u(-7)},
		{7, f64b(1.9e19), 0xffffffffffffffff},
		{7, f64b(1.5e19), 15000000000000000000},
	}
	for _, tc := range cases {
		if got := truncSat(t, tc.sub, tc.v); got != tc.want {
			t.Fatalf("sub %d(%#x): got %#x, want %#x", tc.sub, tc.v, got, tc.want)
		}
	}
}
