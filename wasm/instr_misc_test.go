package wasm

import (
	"math"
	"testing"

	"github.com/c0mm4nd/wasman/stacks"
)

func Test_truncSat(t *testing.T) {
	f32 := func(v float32) uint64 { return uint64(math.Float32bits(v)) }
	f64 := func(v float64) uint64 { return math.Float64bits(v) }

	tests := []struct {
		name string
		fn   func(*Instance) error
		in   uint64
		want uint64 // low bits compared per width below
		w64  bool
	}{
		// i32.trunc_sat_f32_s
		{"i32s nan", i32truncSatF32S, f32(float32(math.NaN())), 0, false},
		{"i32s 3.7", i32truncSatF32S, f32(3.7), 3, false},
		{"i32s -3.7", i32truncSatF32S, f32(-3.7), 0xFFFFFFFD, false}, // -3
		{"i32s +ovf", i32truncSatF32S, f32(3e9), 0x7FFFFFFF, false},  // MaxInt32
		{"i32s -ovf", i32truncSatF32S, f32(-3e9), 0x80000000, false}, // MinInt32
		// i32.trunc_sat_f32_u
		{"i32u neg", i32truncSatF32U, f32(-1), 0, false},
		{"i32u ovf", i32truncSatF32U, f32(5e9), 0xFFFFFFFF, false}, // MaxUint32
		{"i32u 3.7", i32truncSatF32U, f32(3.7), 3, false},
		// i64.trunc_sat_f64_s
		{"i64s nan", i64truncSatF64S, f64(math.NaN()), 0, true},
		{"i64s ovf", i64truncSatF64S, f64(1e19), 0x7FFFFFFFFFFFFFFF, true},   // MaxInt64
		{"i64s -ovf", i64truncSatF64S, f64(-1e19), 0x8000000000000000, true}, // MinInt64
		// i64.trunc_sat_f64_u
		{"i64u neg", i64truncSatF64U, f64(-1), 0, true},
		{"i64u ovf", i64truncSatF64U, f64(2e19), 0xFFFFFFFFFFFFFFFF, true}, // MaxUint64
	}
	for _, tt := range tests {
		vm := &Instance{OperandStack: stacks.NewOperandStack()}
		vm.OperandStack.Push(tt.in)
		if err := tt.fn(vm); err != nil {
			t.Fatalf("%s: unexpected error %v", tt.name, err)
		}
		got := vm.OperandStack.Pop()
		if !tt.w64 {
			got = uint64(uint32(got))
		}
		if got != tt.want {
			t.Errorf("%s: got %#x want %#x", tt.name, got, tt.want)
		}
	}
}
