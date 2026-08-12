package wasm

import (
	"math"
	"testing"

	"github.com/c0mm4nd/wasman/stacks"
)

func newNumVM() *Instance {
	return &Instance{OperandStack: stacks.NewOperandStack()}
}

// Test_i64gtu guards against the copy-paste regression where i64.gt_u
// used `<` instead of `>`.
func Test_i64gtu(t *testing.T) {
	tests := []struct {
		v1, v2 uint64
		want   uint64
	}{
		{5, 3, 1},
		{3, 5, 0},
		{5, 5, 0},
		{math.MaxUint64, 0, 1},
	}
	for _, tt := range tests {
		vm := newNumVM()
		vm.OperandStack.Push(tt.v1)
		vm.OperandStack.Push(tt.v2)
		if err := i64gtu(vm); err != nil {
			t.Fatal(err)
		}
		if got := vm.OperandStack.Pop(); got != tt.want {
			t.Errorf("i64gtu(%d,%d)=%d want %d", tt.v1, tt.v2, got, tt.want)
		}
	}
}

// Test_f32gt guards the operand ordering (must compute first > second).
func Test_f32gt(t *testing.T) {
	tests := []struct {
		v1, v2 float32
		want   uint64
	}{
		{2, 1, 1},
		{1, 2, 0},
		{1, 1, 0},
	}
	for _, tt := range tests {
		vm := newNumVM()
		vm.OperandStack.Push(uint64(math.Float32bits(tt.v1)))
		vm.OperandStack.Push(uint64(math.Float32bits(tt.v2)))
		if err := f32gt(vm); err != nil {
			t.Fatal(err)
		}
		if got := vm.OperandStack.Pop(); got != tt.want {
			t.Errorf("f32gt(%v,%v)=%d want %d", tt.v1, tt.v2, got, tt.want)
		}
	}
}

func Test_f32max(t *testing.T) {
	vm := newNumVM()
	vm.OperandStack.Push(uint64(math.Float32bits(1)))
	vm.OperandStack.Push(uint64(math.Float32bits(2)))
	if err := f32max(vm); err != nil {
		t.Fatal(err)
	}
	if got := math.Float32frombits(uint32(vm.OperandStack.Pop())); got != 2 {
		t.Errorf("f32max(1,2)=%v want 2", got)
	}
}

func Test_f64max(t *testing.T) {
	vm := newNumVM()
	vm.OperandStack.Push(math.Float64bits(1))
	vm.OperandStack.Push(math.Float64bits(2))
	if err := f64max(vm); err != nil {
		t.Fatal(err)
	}
	if got := math.Float64frombits(vm.OperandStack.Pop()); got != 2 {
		t.Errorf("f64max(1,2)=%v want 2", got)
	}
}

// Test_f32nearest checks round-to-nearest ties-to-even and f32 encoding.
func Test_f32nearest(t *testing.T) {
	tests := []struct {
		in, want float32
	}{
		{2.5, 2}, // tie -> even
		{3.5, 4}, // tie -> even
		{2.3, 2},
		{2.7, 3},
		{-2.5, -2},
	}
	for _, tt := range tests {
		vm := newNumVM()
		vm.OperandStack.Push(uint64(math.Float32bits(tt.in)))
		if err := f32nearest(vm); err != nil {
			t.Fatal(err)
		}
		raw := vm.OperandStack.Pop()
		if raw>>32 != 0 {
			t.Errorf("f32nearest(%v) leaked high bits: %#x", tt.in, raw)
		}
		if got := math.Float32frombits(uint32(raw)); got != tt.want {
			t.Errorf("f32nearest(%v)=%v want %v", tt.in, got, tt.want)
		}
	}
}

func Test_signExtend(t *testing.T) {
	// i32.extend8_s: 0xFF (byte -1) -> -1
	vm := newNumVM()
	vm.OperandStack.Push(0xFF)
	if err := i32extend8s(vm); err != nil {
		t.Fatal(err)
	}
	if got := uint32(vm.OperandStack.Pop()); got != 0xFFFFFFFF {
		t.Errorf("i32.extend8_s(0xFF)=%#x want 0xffffffff", got)
	}
	// i64.extend32_s: 0x8000_0000 -> sign-extend to 0xFFFFFFFF_80000000
	vm = newNumVM()
	vm.OperandStack.Push(0x80000000)
	if err := i64extend32s(vm); err != nil {
		t.Fatal(err)
	}
	if got := vm.OperandStack.Pop(); got != 0xFFFFFFFF80000000 {
		t.Errorf("i64.extend32_s=%#x want 0xffffffff80000000", got)
	}
	// i32.extend16_s of a positive value stays positive
	vm = newNumVM()
	vm.OperandStack.Push(0x1234)
	if err := i32extend16s(vm); err != nil {
		t.Fatal(err)
	}
	if got := uint32(vm.OperandStack.Pop()); got != 0x1234 {
		t.Errorf("i32.extend16_s(0x1234)=%#x want 0x1234", got)
	}
}

// Test_truncTrap checks that out-of-range / NaN float->int conversions trap
// instead of producing an undefined result.
func Test_truncTrap(t *testing.T) {
	// in-range value truncates normally
	vm := newNumVM()
	vm.OperandStack.Push(uint64(math.Float32bits(3.7)))
	if err := i32truncf32s(vm); err != nil {
		t.Fatalf("unexpected trap: %v", err)
	}
	if got := int32(uint32(vm.OperandStack.Pop())); got != 3 {
		t.Errorf("i32.trunc_f32_s(3.7)=%d want 3", got)
	}

	traps := []struct {
		name string
		fn   func(*Instance) error
		bits uint64
	}{
		{"nan", i32truncf32s, uint64(math.Float32bits(float32(math.NaN())))},
		{"+inf", i32truncf32s, uint64(math.Float32bits(float32(math.Inf(1))))},
		{"overflow", i32truncf32s, uint64(math.Float32bits(3e9))},        // > 2^31
		{"neg-overflow-u", i32truncf32u, uint64(math.Float32bits(-1.5))}, // < 0
		{"i64-overflow", i64truncf64s, math.Float64bits(1e19)},           // > 2^63
	}
	for _, tt := range traps {
		vm := newNumVM()
		vm.OperandStack.Push(tt.bits)
		if err := tt.fn(vm); err == nil {
			t.Errorf("%s: expected a trap, got none", tt.name)
		}
	}
}

// Test_f32copysign_nan checks copysign only moves the sign bit and preserves
// the NaN payload (no float64 round-trip canonicalization).
func Test_f32copysign_nan(t *testing.T) {
	// magnitude NaN 0x7f80f1e2, sign from a negative number -> 0xff80f1e2
	vm := newNumVM()
	vm.OperandStack.Push(uint64(uint32(0x7f80f1e2)))     // v1 (magnitude, a NaN)
	vm.OperandStack.Push(uint64(math.Float32bits(-1.0))) // v2 (sign source)
	if err := f32copysign(vm); err != nil {
		t.Fatal(err)
	}
	if got := uint32(vm.OperandStack.Pop()); got != 0xff80f1e2 {
		t.Errorf("f32.copysign NaN payload=%#x want 0xff80f1e2", got)
	}
}

// Test_i64shrs_negative guards against a Go "negative shift amount" panic when
// the shift operand has its sign bit set (the count must be unsigned mod 64).
func Test_i64shrs_negative(t *testing.T) {
	vm := newNumVM()
	vm.OperandStack.Push(uint64(0xFFFFFFFFFFFFFFFF)) // v1 = -1
	vm.OperandStack.Push(uint64(0xFFFFFFFFFFFFFFFF)) // v2 = -1 -> count = 63
	if err := i64shrs(vm); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := vm.OperandStack.Pop(); got != 0xFFFFFFFFFFFFFFFF {
		t.Errorf("i64.shr_s(-1, -1)=%#x want 0xffffffffffffffff", got)
	}
}

func Test_f64nearest(t *testing.T) {
	tests := []struct {
		in, want float64
	}{
		{2.5, 2},
		{3.5, 4},
		{2.3, 2},
		{2.7, 3},
		{-2.5, -2},
	}
	for _, tt := range tests {
		vm := newNumVM()
		vm.OperandStack.Push(math.Float64bits(tt.in))
		if err := f64nearest(vm); err != nil {
			t.Fatal(err)
		}
		if got := math.Float64frombits(vm.OperandStack.Pop()); got != tt.want {
			t.Errorf("f64nearest(%v)=%v want %v", tt.in, got, tt.want)
		}
	}
}

// Test_i32eq_mixedRepresentation guards against comparing the full 64-bit
// stack slots: the interpreter's i32 representation is not normalized (some
// ops sign-extend, others zero-extend), so i32.eq/ne/eqz must compare only
// the low 32 bits.
func Test_i32eq_mixedRepresentation(t *testing.T) {
	minusOne := int32(-1)
	signExt := uint64(minusOne) // 0xFFFFFFFF_FFFFFFFF (e.g. from i32.const -1)
	zeroExt := uint64(uint32(0xFFFFFFFF))

	vm := newNumVM()
	vm.OperandStack.Push(signExt)
	vm.OperandStack.Push(zeroExt)
	if err := i32eq(vm); err != nil {
		t.Fatal(err)
	}
	if got := vm.OperandStack.Pop(); got != 1 {
		t.Errorf("i32.eq(sign-ext -1, zero-ext -1)=%d want 1", got)
	}

	vm = newNumVM()
	vm.OperandStack.Push(signExt)
	vm.OperandStack.Push(zeroExt)
	if err := i32ne(vm); err != nil {
		t.Fatal(err)
	}
	if got := vm.OperandStack.Pop(); got != 0 {
		t.Errorf("i32.ne(sign-ext -1, zero-ext -1)=%d want 0", got)
	}

	// eqz must also ignore garbage above bit 31
	vm = newNumVM()
	vm.OperandStack.Push(uint64(0xFFFFFFFF_00000000))
	if err := i32eqz(vm); err != nil {
		t.Fatal(err)
	}
	if got := vm.OperandStack.Pop(); got != 1 {
		t.Errorf("i32.eqz(high-bits-only)=%d want 1", got)
	}
}

// Test_divRemByZero checks div/rem by zero return a trap error instead of
// panicking the host (which is fatal when Recover is disabled).
func Test_divRemByZero(t *testing.T) {
	for _, tt := range []struct {
		name string
		fn   func(*Instance) error
	}{
		{"i32.div_u", i32divu},
		{"i64.div_u", i64divu},
		{"i32.rem_s", i32rems},
		{"i32.rem_u", i32remu},
		{"i64.rem_s", i64rems},
		{"i64.rem_u", i64remu},
	} {
		vm := newNumVM()
		vm.OperandStack.Push(1) // dividend
		vm.OperandStack.Push(0) // divisor
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: panicked on divide by zero: %v", tt.name, r)
				}
			}()
			if err := tt.fn(vm); err == nil {
				t.Errorf("%s: expected a trap error on divide by zero", tt.name)
			}
		}()
	}
}
