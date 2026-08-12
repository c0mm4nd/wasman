package wasm

import (
	"errors"
	"math"
	"math/bits"
)

// ErrUndefined is a panic error
var ErrUndefined = errors.New("undefined")

// trapping float-to-int conversion errors
var (
	ErrInvalidConversionToInt = errors.New("invalid conversion to integer")
	ErrIntegerOverflow        = errors.New("integer overflow")
)

// truncCheck truncates v and verifies the result lies in [lo, hi), trapping on
// NaN, ±Inf or out-of-range values as required by the wasm iNN.trunc_fMM ops.
func truncCheck(v float64, lo, hi float64) (float64, error) {
	if math.IsNaN(v) {
		return 0, ErrInvalidConversionToInt
	}
	t := math.Trunc(v)
	if math.IsInf(t, 0) || t < lo || t >= hi {
		return 0, ErrIntegerOverflow
	}
	return t, nil
}

func i32eqz(ins *Instance) error {
	if ins.OperandStack.Pop() == 0 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i32eq(ins *Instance) error {
	v1 := ins.OperandStack.Pop()
	v2 := ins.OperandStack.Pop()
	if v1 == v2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i32ne(ins *Instance) error {
	v1 := ins.OperandStack.Pop()
	v2 := ins.OperandStack.Pop()
	if v1 != v2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i32lts(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	if int32(v1) < int32(v2) {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i32ltu(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	if uint32(v1) < uint32(v2) {

		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i32gts(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	if int32(v1) > int32(v2) {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i32gtu(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	if uint32(v1) > uint32(v2) {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i32les(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	if int32(v1) <= int32(v2) {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i32leu(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	if uint32(v1) <= uint32(v2) {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i32ges(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	if int32(v1) >= int32(v2) {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i32geu(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	if uint32(v1) >= uint32(v2) {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i64eqz(ins *Instance) error {
	if ins.OperandStack.Pop() == 0 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i64eq(ins *Instance) error {
	v1 := ins.OperandStack.Pop()
	v2 := ins.OperandStack.Pop()
	if v1 == v2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i64ne(ins *Instance) error {
	v1 := ins.OperandStack.Pop()
	v2 := ins.OperandStack.Pop()
	if v1 != v2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i64lts(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	if int64(v1) < int64(v2) {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i64ltu(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	if v1 < v2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i64gts(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	if int64(v1) > int64(v2) {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i64gtu(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	if v1 > v2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i64les(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	if int64(v1) <= int64(v2) {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i64leu(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	if v1 <= v2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i64ges(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	if int64(v1) >= int64(v2) {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i64geu(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	if v1 >= v2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}
	return nil
}

func f32eq(ins *Instance) error {
	f2 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	f1 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	if f1 == f2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func f32ne(ins *Instance) error {
	f2 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	f1 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	if f1 != f2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func f32lt(ins *Instance) error {
	f2 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	f1 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	if f1 < f2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func f32gt(ins *Instance) error {
	f2 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	f1 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	if f1 > f2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func f32le(ins *Instance) error {
	f2 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	f1 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	if f1 <= f2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func f32ge(ins *Instance) error {
	f2 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	f1 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	if f1 >= f2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func f64eq(ins *Instance) error {
	f2 := math.Float64frombits(ins.OperandStack.Pop())
	f1 := math.Float64frombits(ins.OperandStack.Pop())
	if f1 == f2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func f64ne(ins *Instance) error {
	f2 := math.Float64frombits(ins.OperandStack.Pop())
	f1 := math.Float64frombits(ins.OperandStack.Pop())
	if f1 != f2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func f64lt(ins *Instance) error {
	f2 := math.Float64frombits(ins.OperandStack.Pop())
	f1 := math.Float64frombits(ins.OperandStack.Pop())
	if f1 < f2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func f64gt(ins *Instance) error {
	f2 := math.Float64frombits(ins.OperandStack.Pop())
	f1 := math.Float64frombits(ins.OperandStack.Pop())
	if f1 > f2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func f64le(ins *Instance) error {
	f2 := math.Float64frombits(ins.OperandStack.Pop())
	f1 := math.Float64frombits(ins.OperandStack.Pop())
	if f1 <= f2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func f64ge(ins *Instance) error {
	f2 := math.Float64frombits(ins.OperandStack.Pop())
	f1 := math.Float64frombits(ins.OperandStack.Pop())
	if f1 >= f2 {
		ins.OperandStack.Push(1)
	} else {
		ins.OperandStack.Push(0)
	}

	return nil
}

func i32clz(ins *Instance) error {
	ins.OperandStack.Push(uint64(bits.LeadingZeros32(uint32(ins.OperandStack.Pop()))))

	return nil
}

func i32ctz(ins *Instance) error {
	ins.OperandStack.Push(uint64(bits.TrailingZeros32(uint32(ins.OperandStack.Pop()))))

	return nil
}

func i32popcnt(ins *Instance) error {
	ins.OperandStack.Push(uint64(bits.OnesCount32(uint32(ins.OperandStack.Pop()))))

	return nil
}

func i32add(ins *Instance) error {
	ins.OperandStack.Push(uint64(int32(ins.OperandStack.Pop()) + int32(ins.OperandStack.Pop())))

	return nil
}

func i32sub(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	ins.OperandStack.Push(uint64(int32(v1) - int32(v2)))

	return nil
}

func i32mul(ins *Instance) error {
	ins.OperandStack.Push(uint64(int32(ins.OperandStack.Pop()) * int32(ins.OperandStack.Pop())))

	return nil
}

func i32divs(ins *Instance) error {
	v2 := int32(ins.OperandStack.Pop())
	v1 := int32(ins.OperandStack.Pop())
	if v2 == 0 || (v1 == math.MinInt32 && v2 == -1) {
		return ErrUndefined
	}
	ins.OperandStack.Push(uint64(v1 / v2))

	return nil
}

func i32divu(ins *Instance) error {
	v2 := uint32(ins.OperandStack.Pop())
	v1 := uint32(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64(v1 / v2))

	return nil
}

func i32rems(ins *Instance) error {
	v2 := int32(ins.OperandStack.Pop())
	v1 := int32(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64(v1 % v2))

	return nil
}

func i32remu(ins *Instance) error {
	v2 := uint32(ins.OperandStack.Pop())
	v1 := uint32(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64(v1 % v2))

	return nil
}

func i32and(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	ins.OperandStack.Push(uint64(uint32(v1) & uint32(v2)))

	return nil
}

func i32or(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	ins.OperandStack.Push(uint64(uint32(v1) | uint32(v2)))

	return nil
}

func i32xor(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	ins.OperandStack.Push(uint64(uint32(v1) ^ uint32(v2)))

	return nil
}

func i32shl(ins *Instance) error {
	v2 := uint32(ins.OperandStack.Pop())
	v1 := uint32(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64(v1 << (v2 % 32)))

	return nil
}

func i32shru(ins *Instance) error {
	v2 := uint32(ins.OperandStack.Pop())
	v1 := uint32(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64(v1 >> (v2 % 32)))

	return nil
}

func i32shrs(ins *Instance) error {
	v2 := uint32(ins.OperandStack.Pop())
	v1 := int32(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64(v1 >> (v2 % 32)))

	return nil
}

func i32rotl(ins *Instance) error {
	v2 := int(ins.OperandStack.Pop())
	v1 := uint32(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64(bits.RotateLeft32(v1, v2)))

	return nil
}

func i32rotr(ins *Instance) error {
	v2 := int(ins.OperandStack.Pop())
	v1 := uint32(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64(bits.RotateLeft32(v1, -v2)))

	return nil
}

// i64
func i64clz(ins *Instance) error {
	ins.OperandStack.Push(uint64(bits.LeadingZeros64(ins.OperandStack.Pop())))

	return nil
}

func i64ctz(ins *Instance) error {
	ins.OperandStack.Push(uint64(bits.TrailingZeros64(ins.OperandStack.Pop())))

	return nil
}

func i64popcnt(ins *Instance) error {
	ins.OperandStack.Push(uint64(bits.OnesCount64(ins.OperandStack.Pop())))

	return nil
}

func i64add(ins *Instance) error {
	ins.OperandStack.Push(ins.OperandStack.Pop() + ins.OperandStack.Pop())

	return nil
}

func i64sub(ins *Instance) error {
	v2 := int64(ins.OperandStack.Pop())
	v1 := int64(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64(v1 - v2))

	return nil
}

func i64mul(ins *Instance) error {
	ins.OperandStack.Push(ins.OperandStack.Pop() * ins.OperandStack.Pop())

	return nil
}

func i64divs(ins *Instance) error {
	v2 := int64(ins.OperandStack.Pop())
	v1 := int64(ins.OperandStack.Pop())
	if v2 == 0 || (v1 == math.MinInt64 && v2 == -1) {
		return ErrUndefined
	}
	ins.OperandStack.Push(uint64(v1 / v2))

	return nil
}

func i64divu(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	ins.OperandStack.Push(v1 / v2)

	return nil
}

func i64rems(ins *Instance) error {
	v2 := int64(ins.OperandStack.Pop())
	v1 := int64(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64(v1 % v2))

	return nil
}

func i64remu(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	ins.OperandStack.Push(v1 % v2)

	return nil
}

func i64and(ins *Instance) error {
	v2 := int64(ins.OperandStack.Pop())
	v1 := int64(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64(v1 & v2))

	return nil
}

func i64or(ins *Instance) error {
	v1 := ins.OperandStack.Pop()
	v2 := ins.OperandStack.Pop()
	ins.OperandStack.Push(v1 | v2)

	return nil
}

func i64xor(ins *Instance) error {
	v1 := ins.OperandStack.Pop()
	v2 := ins.OperandStack.Pop()
	ins.OperandStack.Push(v1 ^ v2)

	return nil
}

func i64shl(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	ins.OperandStack.Push(v1 << (v2 % 64))

	return nil
}

func i64shru(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	ins.OperandStack.Push(v1 >> (v2 % 64))

	return nil
}

func i64shrs(ins *Instance) error {
	v2 := ins.OperandStack.Pop()
	v1 := int64(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64(v1 >> (v2 % 64)))

	return nil
}

func i64rotl(ins *Instance) error {
	v2 := int(ins.OperandStack.Pop())
	v1 := ins.OperandStack.Pop()
	ins.OperandStack.Push(bits.RotateLeft64(v1, v2))

	return nil
}

func i64rotr(ins *Instance) error {
	v2 := int(ins.OperandStack.Pop())
	v1 := ins.OperandStack.Pop()
	ins.OperandStack.Push(bits.RotateLeft64(v1, -v2))

	return nil
}

func f32abs(ins *Instance) error {
	const mask uint32 = 1 << 31
	v := uint32(ins.OperandStack.Pop()) &^ mask
	ins.OperandStack.Push(uint64(v))

	return nil
}

func f32neg(ins *Instance) error {
	v := -math.Float32frombits(uint32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(uint64(math.Float32bits(v)))

	return nil
}

func f32ceil(ins *Instance) error {
	v := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(uint64(math.Float32bits(float32(math.Ceil(float64(v))))))

	return nil
}

func f32floor(ins *Instance) error {
	v := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(uint64(math.Float32bits(float32(math.Floor(float64(v))))))

	return nil
}

func f32trunc(ins *Instance) error {
	v := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(uint64(math.Float32bits(float32(math.Trunc(float64(v))))))

	return nil
}

func f32nearest(ins *Instance) error {
	raw := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	// wasm f32.nearest rounds to the nearest integer, ties to even
	v := float32(math.RoundToEven(float64(raw)))
	ins.OperandStack.Push(uint64(math.Float32bits(v)))

	return nil
}

func f32sqrt(ins *Instance) error {
	v := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(uint64(math.Float32bits(float32(math.Sqrt(float64(v))))))

	return nil
}

func f32add(ins *Instance) error {
	v := math.Float32frombits(uint32(ins.OperandStack.Pop())) + math.Float32frombits(uint32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(uint64(math.Float32bits(v)))

	return nil
}

func f32sub(ins *Instance) error {
	v2 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	v1 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(uint64(math.Float32bits(v1 - v2)))

	return nil
}

func f32mul(ins *Instance) error {
	v := math.Float32frombits(uint32(ins.OperandStack.Pop())) * math.Float32frombits(uint32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(uint64(math.Float32bits(v)))

	return nil
}

func f32div(ins *Instance) error {
	v2 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	v1 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(uint64(math.Float32bits(v1 / v2)))

	return nil
}

func f32min(ins *Instance) error {
	v2 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	v1 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(uint64(math.Float32bits(float32(math.Min(float64(v1), float64(v2))))))

	return nil
}

func f32max(ins *Instance) error {
	v2 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	v1 := math.Float32frombits(uint32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(uint64(math.Float32bits(float32(math.Max(float64(v1), float64(v2))))))

	return nil
}

func f32copysign(ins *Instance) error {
	// pure bit operation: magnitude of v1 with the sign bit of v2. Going
	// through float64 would canonicalize (mangle) NaN payloads.
	const signMask uint32 = 1 << 31
	v2 := uint32(ins.OperandStack.Pop())
	v1 := uint32(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64((v1 &^ signMask) | (v2 & signMask)))

	return nil
}

func f64abs(ins *Instance) error {
	const mask = 1 << 63
	v := ins.OperandStack.Pop() &^ mask
	ins.OperandStack.Push(v)

	return nil
}

func f64neg(ins *Instance) error {
	v := -math.Float64frombits(ins.OperandStack.Pop())
	ins.OperandStack.Push(math.Float64bits(v))

	return nil
}

func f64ceil(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.Pop())
	ins.OperandStack.Push(math.Float64bits(math.Ceil(v)))

	return nil
}

func f64floor(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.Pop())
	ins.OperandStack.Push(math.Float64bits(math.Floor(v)))

	return nil
}

func f64trunc(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.Pop())
	ins.OperandStack.Push(math.Float64bits(math.Trunc(v)))

	return nil
}

func f64nearest(ins *Instance) error {
	raw := math.Float64frombits(ins.OperandStack.Pop())
	// wasm f64.nearest rounds to the nearest integer, ties to even
	ins.OperandStack.Push(math.Float64bits(math.RoundToEven(raw)))

	return nil
}

func f64sqrt(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.Pop())
	ins.OperandStack.Push(math.Float64bits(math.Sqrt(v)))

	return nil
}

func f64add(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.Pop()) + math.Float64frombits(ins.OperandStack.Pop())
	ins.OperandStack.Push(math.Float64bits(v))

	return nil
}

func f64sub(ins *Instance) error {
	v2 := math.Float64frombits(ins.OperandStack.Pop())
	v1 := math.Float64frombits(ins.OperandStack.Pop())
	ins.OperandStack.Push(math.Float64bits(v1 - v2))

	return nil
}

func f64mul(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.Pop()) * math.Float64frombits(ins.OperandStack.Pop())
	ins.OperandStack.Push(math.Float64bits(v))

	return nil
}

func f64div(ins *Instance) error {
	v2 := math.Float64frombits(ins.OperandStack.Pop())
	v1 := math.Float64frombits(ins.OperandStack.Pop())
	ins.OperandStack.Push(math.Float64bits(v1 / v2))

	return nil
}

func f64min(ins *Instance) error {
	v2 := math.Float64frombits(ins.OperandStack.Pop())
	v1 := math.Float64frombits(ins.OperandStack.Pop())
	ins.OperandStack.Push(math.Float64bits(math.Min(v1, v2)))

	return nil
}

func f64max(ins *Instance) error {
	v2 := math.Float64frombits(ins.OperandStack.Pop())
	v1 := math.Float64frombits(ins.OperandStack.Pop())
	ins.OperandStack.Push(math.Float64bits(math.Max(v1, v2)))

	return nil
}

func f64copysign(ins *Instance) error {
	// pure bit operation: magnitude of v1 with the sign bit of v2.
	const signMask uint64 = 1 << 63
	v2 := ins.OperandStack.Pop()
	v1 := ins.OperandStack.Pop()
	ins.OperandStack.Push((v1 &^ signMask) | (v2 & signMask))

	return nil
}

func i32wrapi64(ins *Instance) error {
	ins.OperandStack.Push(uint64(uint32(ins.OperandStack.Pop())))

	return nil
}

func i32truncf32s(ins *Instance) error {
	v := float64(math.Float32frombits(uint32(ins.OperandStack.Pop())))
	t, err := truncCheck(v, -2147483648.0, 2147483648.0) // [-2^31, 2^31)
	if err != nil {
		return err
	}
	ins.OperandStack.Push(uint64(uint32(int32(t))))

	return nil
}

func i32truncf32u(ins *Instance) error {
	v := float64(math.Float32frombits(uint32(ins.OperandStack.Pop())))
	t, err := truncCheck(v, 0.0, 4294967296.0) // [0, 2^32)
	if err != nil {
		return err
	}
	ins.OperandStack.Push(uint64(uint32(t)))

	return nil
}

func i32truncf64s(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.Pop())
	t, err := truncCheck(v, -2147483648.0, 2147483648.0) // [-2^31, 2^31)
	if err != nil {
		return err
	}
	ins.OperandStack.Push(uint64(uint32(int32(t))))

	return nil
}

func i32truncf64u(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.Pop())
	t, err := truncCheck(v, 0.0, 4294967296.0) // [0, 2^32)
	if err != nil {
		return err
	}
	ins.OperandStack.Push(uint64(uint32(t)))

	return nil
}

func i64extendi32s(ins *Instance) error {
	v := int64(int32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(uint64(v))

	return nil
}

func i64extendi32u(ins *Instance) error {
	v := uint64(uint32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(v)

	return nil
}

func i64truncf32s(ins *Instance) error {
	v := float64(math.Float32frombits(uint32(ins.OperandStack.Pop())))
	t, err := truncCheck(v, -9223372036854775808.0, 9223372036854775808.0) // [-2^63, 2^63)
	if err != nil {
		return err
	}
	ins.OperandStack.Push(uint64(int64(t)))

	return nil
}

func i64truncf32u(ins *Instance) error {
	v := float64(math.Float32frombits(uint32(ins.OperandStack.Pop())))
	t, err := truncCheck(v, 0.0, 18446744073709551616.0) // [0, 2^64)
	if err != nil {
		return err
	}
	ins.OperandStack.Push(uint64(t))

	return nil
}

func i64truncf64s(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.Pop())
	t, err := truncCheck(v, -9223372036854775808.0, 9223372036854775808.0) // [-2^63, 2^63)
	if err != nil {
		return err
	}
	ins.OperandStack.Push(uint64(int64(t)))

	return nil
}

func i64truncf64u(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.Pop())
	t, err := truncCheck(v, 0.0, 18446744073709551616.0) // [0, 2^64)
	if err != nil {
		return err
	}
	ins.OperandStack.Push(uint64(t))

	return nil
}

func f32converti32s(ins *Instance) error {
	v := float32(int32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(uint64(math.Float32bits(v)))

	return nil
}

func f32converti32u(ins *Instance) error {
	v := float32(uint32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(uint64(math.Float32bits(v)))

	return nil
}

func f32converti64s(ins *Instance) error {
	v := float32(int64(ins.OperandStack.Pop()))
	ins.OperandStack.Push(uint64(math.Float32bits(v)))

	return nil
}

func f32converti64u(ins *Instance) error {
	v := float32(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64(math.Float32bits(v)))

	return nil
}

func f32demotef64(ins *Instance) error {
	v := float32(math.Float64frombits(ins.OperandStack.Pop()))
	ins.OperandStack.Push(uint64(math.Float32bits(v)))

	return nil
}

func f64converti32s(ins *Instance) error {
	v := float64(int32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(math.Float64bits(v))

	return nil
}

func f64converti32u(ins *Instance) error {
	v := float64(uint32(ins.OperandStack.Pop()))
	ins.OperandStack.Push(math.Float64bits(v))

	return nil
}

func f64converti64s(ins *Instance) error {
	v := float64(int64(ins.OperandStack.Pop()))
	ins.OperandStack.Push(math.Float64bits(v))

	return nil
}

func f64converti64u(ins *Instance) error {
	v := float64(ins.OperandStack.Pop())
	ins.OperandStack.Push(math.Float64bits(v))

	return nil
}

func f64promotef32(ins *Instance) error {
	v := float64(math.Float32frombits(uint32(ins.OperandStack.Pop())))
	ins.OperandStack.Push(math.Float64bits(v))

	return nil
}

// sign-extension operators (wasm 2.0, opcodes 0xc0-0xc4)

func i32extend8s(ins *Instance) error {
	v := int32(int8(uint8(ins.OperandStack.Pop())))
	ins.OperandStack.Push(uint64(uint32(v)))

	return nil
}

func i32extend16s(ins *Instance) error {
	v := int32(int16(uint16(ins.OperandStack.Pop())))
	ins.OperandStack.Push(uint64(uint32(v)))

	return nil
}

func i64extend8s(ins *Instance) error {
	v := int64(int8(uint8(ins.OperandStack.Pop())))
	ins.OperandStack.Push(uint64(v))

	return nil
}

func i64extend16s(ins *Instance) error {
	v := int64(int16(uint16(ins.OperandStack.Pop())))
	ins.OperandStack.Push(uint64(v))

	return nil
}

func i64extend32s(ins *Instance) error {
	v := int64(int32(uint32(ins.OperandStack.Pop())))
	ins.OperandStack.Push(uint64(v))

	return nil
}
