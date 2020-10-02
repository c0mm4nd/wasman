package wasman

import (
	"errors"
	"math"
	"math/bits"
)

var ErrUndefined = errors.New("undefined")

func i32eqz(ins *Instance) error {
	ins.OperandStack.pushBool(ins.OperandStack.pop() == 0)

	return nil
}

func i32eq(ins *Instance) error {
	ins.OperandStack.pushBool(ins.OperandStack.pop() == ins.OperandStack.pop())

	return nil
}

func i32ne(ins *Instance) error {
	ins.OperandStack.pushBool(ins.OperandStack.pop() != ins.OperandStack.pop())

	return nil
}

func i32lts(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(int32(v1) < int32(v2))

	return nil
}

func i32ltu(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(uint32(v1) < uint32(v2))

	return nil
}

func i32gts(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(int32(v1) > int32(v2))

	return nil
}

func i32gtu(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(uint32(v1) > uint32(v2))

	return nil
}

func i32les(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(int32(v1) <= int32(v2))

	return nil
}

func i32leu(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(uint32(v1) <= uint32(v2))

	return nil
}

func i32ges(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(int32(v1) >= int32(v2))

	return nil
}

func i32geu(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(uint32(v1) >= uint32(v2))

	return nil
}

func i64eqz(ins *Instance) error {
	ins.OperandStack.pushBool(ins.OperandStack.pop() == 0)

	return nil
}

func i64eq(ins *Instance) error {
	ins.OperandStack.pushBool(ins.OperandStack.pop() == ins.OperandStack.pop())

	return nil
}

func i64ne(ins *Instance) error {
	ins.OperandStack.pushBool(ins.OperandStack.pop() != ins.OperandStack.pop())

	return nil
}

func i64lts(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(int64(v1) < int64(v2))

	return nil
}

func i64ltu(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(v1 < v2)

	return nil
}

func i64gts(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(int64(v1) > int64(v2))

	return nil
}

func i64gtu(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(v1 < v2)

	return nil
}

func i64les(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(int64(v1) <= int64(v2))

	return nil
}

func i64leu(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(v1 <= v2)

	return nil
}

func i64ges(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(int64(v1) >= int64(v2))

	return nil
}

func i64geu(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(v1 >= v2)

	return nil
}

func f32eq(ins *Instance) error {
	f2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	f1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.pushBool(f1 == f2)

	return nil
}

func f32ne(ins *Instance) error {
	f2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	f1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.pushBool(f1 != f2)

	return nil
}

func f32lt(ins *Instance) error {
	f2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	f1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.pushBool(f1 < f2)

	return nil
}

func f32gt(ins *Instance) error {
	f1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	f2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.pushBool(f1 > f2)

	return nil
}

func f32le(ins *Instance) error {
	f2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	f1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.pushBool(f1 <= f2)

	return nil
}

func f32ge(ins *Instance) error {
	f2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	f1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.pushBool(f1 >= f2)

	return nil
}

func f64eq(ins *Instance) error {
	f2 := math.Float64frombits(ins.OperandStack.pop())
	f1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.pushBool(f1 == f2)

	return nil
}

func f64ne(ins *Instance) error {
	f2 := math.Float64frombits(ins.OperandStack.pop())
	f1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.pushBool(f1 != f2)

	return nil
}

func f64lt(ins *Instance) error {
	f2 := math.Float64frombits(ins.OperandStack.pop())
	f1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.pushBool(f1 < f2)

	return nil
}

func f64gt(ins *Instance) error {
	f2 := math.Float64frombits(ins.OperandStack.pop())
	f1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.pushBool(f1 > f2)

	return nil
}

func f64le(ins *Instance) error {
	f2 := math.Float64frombits(ins.OperandStack.pop())
	f1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.pushBool(f1 <= f2)

	return nil
}

func f64ge(ins *Instance) error {
	f2 := math.Float64frombits(ins.OperandStack.pop())
	f1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.pushBool(f1 >= f2)

	return nil
}

func i32clz(ins *Instance) error {
	ins.OperandStack.push(uint64(bits.LeadingZeros32(uint32(ins.OperandStack.pop()))))

	return nil
}

func i32ctz(ins *Instance) error {
	ins.OperandStack.push(uint64(bits.TrailingZeros32(uint32(ins.OperandStack.pop()))))

	return nil
}

func i32popcnt(ins *Instance) error {
	ins.OperandStack.push(uint64(bits.OnesCount32(uint32(ins.OperandStack.pop()))))

	return nil
}

func i32add(ins *Instance) error {
	ins.OperandStack.push(uint64(uint32(ins.OperandStack.pop()) + uint32(ins.OperandStack.pop())))

	return nil
}

func i32sub(ins *Instance) error {
	v2 := uint32(ins.OperandStack.pop())
	v1 := uint32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 - v2))

	return nil
}

func i32mul(ins *Instance) error {
	ins.OperandStack.push(uint64(uint32(ins.OperandStack.pop()) * uint32(ins.OperandStack.pop())))

	return nil
}

func i32divs(ins *Instance) error {
	v2 := int32(ins.OperandStack.pop())
	v1 := int32(ins.OperandStack.pop())
	if v2 == 0 || (v1 == math.MinInt32 && v2 == -1) {
		return ErrUndefined
	}
	ins.OperandStack.push(uint64(v1 / v2))

	return nil
}

func i32divu(ins *Instance) error {
	v2 := uint32(ins.OperandStack.pop())
	v1 := uint32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 / v2))

	return nil
}

func i32rems(ins *Instance) error {
	v2 := int32(ins.OperandStack.pop())
	v1 := int32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 % v2))

	return nil
}

func i32remu(ins *Instance) error {
	v2 := uint32(ins.OperandStack.pop())
	v1 := uint32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 % v2))

	return nil
}

func i32and(ins *Instance) error {
	ins.OperandStack.push(uint64(uint32(ins.OperandStack.pop()) & uint32(ins.OperandStack.pop())))

	return nil
}

func i32or(ins *Instance) error {
	ins.OperandStack.push(uint64(uint32(ins.OperandStack.pop()) | uint32(ins.OperandStack.pop())))

	return nil
}

func i32xor(ins *Instance) error {
	ins.OperandStack.push(uint64(uint32(ins.OperandStack.pop()) ^ uint32(ins.OperandStack.pop())))

	return nil
}

func i32shl(ins *Instance) error {
	v2 := uint32(ins.OperandStack.pop())
	v1 := uint32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 << (v2 % 32)))

	return nil
}

func i32shru(ins *Instance) error {
	v2 := uint32(ins.OperandStack.pop())
	v1 := uint32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 >> (v2 % 32)))

	return nil
}

func i32shrs(ins *Instance) error {
	v2 := uint32(ins.OperandStack.pop())
	v1 := int32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 >> (v2 % 32)))

	return nil
}

func i32rotl(ins *Instance) error {
	v2 := int(ins.OperandStack.pop())
	v1 := uint32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(bits.RotateLeft32(v1, v2)))

	return nil
}

func i32rotr(ins *Instance) error {
	v2 := int(ins.OperandStack.pop())
	v1 := uint32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(bits.RotateLeft32(v1, -v2)))

	return nil
}

// i64
func i64clz(ins *Instance) error {
	ins.OperandStack.push(uint64(bits.LeadingZeros64(ins.OperandStack.pop())))

	return nil
}

func i64ctz(ins *Instance) error {
	ins.OperandStack.push(uint64(bits.TrailingZeros64(ins.OperandStack.pop())))

	return nil
}

func i64popcnt(ins *Instance) error {
	ins.OperandStack.push(uint64(bits.OnesCount64(ins.OperandStack.pop())))

	return nil
}

func i64add(ins *Instance) error {
	ins.OperandStack.push(ins.OperandStack.pop() + ins.OperandStack.pop())

	return nil
}

func i64sub(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.push(v1 - v2)

	return nil
}

func i64mul(ins *Instance) error {
	ins.OperandStack.push(ins.OperandStack.pop() * ins.OperandStack.pop())

	return nil
}

func i64divs(ins *Instance) error {
	v2 := int64(ins.OperandStack.pop())
	v1 := int64(ins.OperandStack.pop())
	if v2 == 0 || (v1 == math.MinInt64 && v2 == -1) {
		return ErrUndefined
	}
	ins.OperandStack.push(uint64(v1 / v2))

	return nil
}

func i64divu(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.push(v1 / v2)

	return nil
}

func i64rems(ins *Instance) error {
	v2 := int64(ins.OperandStack.pop())
	v1 := int64(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 % v2))

	return nil
}

func i64remu(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.push(v1 % v2)

	return nil
}

func i64and(ins *Instance) error {
	ins.OperandStack.push(ins.OperandStack.pop() & ins.OperandStack.pop())

	return nil
}

func i64or(ins *Instance) error {
	ins.OperandStack.push(ins.OperandStack.pop() | ins.OperandStack.pop())

	return nil
}

func i64xor(ins *Instance) error {
	ins.OperandStack.push(ins.OperandStack.pop() ^ ins.OperandStack.pop())

	return nil
}

func i64shl(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.push(v1 << (v2 % 64))

	return nil
}

func i64shru(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.push(v1 >> (v2 % 64))

	return nil
}

func i64shrs(ins *Instance) error {
	v2 := ins.OperandStack.pop()
	v1 := int64(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 >> (v2 % 64)))

	return nil
}

func i64rotl(ins *Instance) error {
	v2 := int(ins.OperandStack.pop())
	v1 := ins.OperandStack.pop()
	ins.OperandStack.push(bits.RotateLeft64(v1, v2))

	return nil
}

func i64rotr(ins *Instance) error {
	v2 := int(ins.OperandStack.pop())
	v1 := ins.OperandStack.pop()
	ins.OperandStack.push(bits.RotateLeft64(v1, -v2))

	return nil
}

func f32abs(ins *Instance) error {
	const mask uint32 = 1 << 31
	v := uint32(ins.OperandStack.pop()) &^ mask
	ins.OperandStack.push(uint64(v))

	return nil
}

func f32neg(ins *Instance) error {
	v := -math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v)))

	return nil
}

func f32ceil(ins *Instance) error {
	v := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(float32(math.Ceil(float64(v))))))

	return nil
}

func f32floor(ins *Instance) error {
	v := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(float32(math.Floor(float64(v))))))

	return nil
}

func f32trunc(ins *Instance) error {
	v := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(float32(math.Trunc(float64(v))))))

	return nil
}

func f32nearest(ins *Instance) error {
	raw := math.Float32frombits(uint32(ins.OperandStack.pop()))
	v := math.Float64bits(float64(int32(raw + float32(math.Copysign(0.5, float64(raw))))))
	ins.OperandStack.push(v)

	return nil
}

func f32sqrt(ins *Instance) error {
	v := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(float32(math.Sqrt(float64(v))))))

	return nil
}

func f32add(ins *Instance) error {
	v := math.Float32frombits(uint32(ins.OperandStack.pop())) + math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v)))

	return nil
}

func f32sub(ins *Instance) error {
	v2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	v1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v1 - v2)))

	return nil
}

func f32mul(ins *Instance) error {
	v := math.Float32frombits(uint32(ins.OperandStack.pop())) * math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v)))

	return nil
}

func f32div(ins *Instance) error {
	v2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	v1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v1 / v2)))

	return nil
}

func f32min(ins *Instance) error {
	v2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	v1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(float32(math.Min(float64(v1), float64(v2))))))

	return nil
}

func f32max(ins *Instance) error {
	v2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	v1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(float32(math.Min(float64(v1), float64(v2))))))

	return nil
}

func f32copysign(ins *Instance) error {
	v2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	v1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(float32(math.Copysign(float64(v1), float64(v2))))))

	return nil
}

func f64abs(ins *Instance) error {
	const mask = 1 << 63
	v := ins.OperandStack.pop() &^ mask
	ins.OperandStack.push(v)

	return nil
}

func f64neg(ins *Instance) error {
	v := -math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(v))

	return nil
}

func f64ceil(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(math.Ceil(v)))

	return nil
}

func f64floor(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(math.Floor(v)))

	return nil
}

func f64trunc(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(math.Trunc(v)))

	return nil
}

func f64nearest(ins *Instance) error {
	raw := math.Float64frombits(ins.OperandStack.pop())
	v := math.Float64bits(float64(int64(raw + math.Copysign(0.5, raw))))
	ins.OperandStack.push(v)

	return nil
}

func f64sqrt(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(math.Sqrt(v)))

	return nil
}

func f64add(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.pop()) + math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(v))

	return nil
}

func f64sub(ins *Instance) error {
	v2 := math.Float64frombits(ins.OperandStack.pop())
	v1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(v1 - v2))

	return nil
}

func f64mul(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.pop()) * math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(v))

	return nil
}

func f64div(ins *Instance) error {
	v2 := math.Float64frombits(ins.OperandStack.pop())
	v1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(v1 / v2))

	return nil
}

func f64min(ins *Instance) error {
	v2 := math.Float64frombits(ins.OperandStack.pop())
	v1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(math.Min(v1, v2)))

	return nil
}

func f64max(ins *Instance) error {
	v2 := math.Float64frombits(ins.OperandStack.pop())
	v1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(math.Min(v1, v2)))

	return nil
}

func f64copysign(ins *Instance) error {
	v2 := math.Float64frombits(ins.OperandStack.pop())
	v1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(math.Copysign(v1, v2)))

	return nil
}

func i32wrapi64(ins *Instance) error {
	ins.OperandStack.push(uint64(uint32(ins.OperandStack.pop())))

	return nil
}

func i32truncf32s(ins *Instance) error {
	v := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(int32(math.Trunc(float64(v)))))

	return nil
}

func i32truncf32u(ins *Instance) error {
	v := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(uint32(math.Trunc(float64(v)))))

	return nil
}

func i32truncf64s(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(int32(math.Trunc(v))))

	return nil
}

func i32truncf64u(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(uint32(math.Trunc(v))))

	return nil
}

func i64extendi32s(ins *Instance) error {
	v := int64(int32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(v))

	return nil
}

func i64extendi32u(ins *Instance) error {
	v := uint64(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(v)

	return nil
}

func i64truncf32s(ins *Instance) error {
	v := math.Trunc(float64(math.Float32frombits(uint32(ins.OperandStack.pop()))))
	ins.OperandStack.push(uint64(int64(v)))

	return nil
}

func i64truncf32u(ins *Instance) error {
	v := math.Trunc(float64(math.Float32frombits(uint32(ins.OperandStack.pop()))))
	ins.OperandStack.push(uint64(v))

	return nil
}

func i64truncf64s(ins *Instance) error {
	v := math.Trunc(math.Float64frombits(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(int64(v)))

	return nil
}

func i64truncf64u(ins *Instance) error {
	v := math.Trunc(math.Float64frombits(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(v))

	return nil
}

func f32converti32s(ins *Instance) error {
	v := float32(int32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v)))

	return nil
}

func f32converti32u(ins *Instance) error {
	v := float32(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v)))

	return nil
}

func f32converti64s(ins *Instance) error {
	v := float32(int64(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v)))

	return nil
}

func f32converti64u(ins *Instance) error {
	v := float32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(math.Float32bits(v)))

	return nil
}

func f32demotef64(ins *Instance) error {
	v := float32(math.Float64frombits(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v)))

	return nil
}

func f64converti32s(ins *Instance) error {
	v := float64(int32(ins.OperandStack.pop()))
	ins.OperandStack.push(math.Float64bits(v))

	return nil
}

func f64converti32u(ins *Instance) error {
	v := float64(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(math.Float64bits(v))

	return nil
}

func f64converti64s(ins *Instance) error {
	v := float64(int64(ins.OperandStack.pop()))
	ins.OperandStack.push(math.Float64bits(v))

	return nil
}

func f64converti64u(ins *Instance) error {
	v := float64(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(v))

	return nil
}

func f64promotef32(ins *Instance) error {
	v := float64(math.Float32frombits(uint32(ins.OperandStack.pop())))
	ins.OperandStack.push(math.Float64bits(v))

	return nil
}
