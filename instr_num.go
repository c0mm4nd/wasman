package wasman

import (
	"math"
	"math/bits"
)

func i32eqz(ins *Instance) {
	ins.OperandStack.pushBool(ins.OperandStack.pop() == 0)
}

func i32eq(ins *Instance) {
	ins.OperandStack.pushBool(ins.OperandStack.pop() == ins.OperandStack.pop())
}

func i32ne(ins *Instance) {
	ins.OperandStack.pushBool(ins.OperandStack.pop() != ins.OperandStack.pop())
}

func i32lts(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(int32(v1) < int32(v2))
}

func i32ltu(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(uint32(v1) < uint32(v2))
}

func i32gts(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(int32(v1) > int32(v2))
}

func i32gtu(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(uint32(v1) > uint32(v2))
}

func i32les(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(int32(v1) <= int32(v2))
}

func i32leu(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(uint32(v1) <= uint32(v2))
}

func i32ges(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(int32(v1) >= int32(v2))
}

func i32geu(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(uint32(v1) >= uint32(v2))
}

func i64eqz(ins *Instance) {
	ins.OperandStack.pushBool(ins.OperandStack.pop() == 0)
}

func i64eq(ins *Instance) {
	ins.OperandStack.pushBool(ins.OperandStack.pop() == ins.OperandStack.pop())
}

func i64ne(ins *Instance) {
	ins.OperandStack.pushBool(ins.OperandStack.pop() != ins.OperandStack.pop())
}

func i64lts(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(int64(v1) < int64(v2))
}

func i64ltu(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(v1 < v2)
}

func i64gts(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(int64(v1) > int64(v2))
}

func i64gtu(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(v1 < v2)
}

func i64les(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(int64(v1) <= int64(v2))
}

func i64leu(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(v1 <= v2)
}

func i64ges(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(int64(v1) >= int64(v2))
}

func i64geu(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.pushBool(v1 >= v2)
}

func f32eq(ins *Instance) {
	f2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	f1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.pushBool(f1 == f2)
}
func f32ne(ins *Instance) {
	f2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	f1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.pushBool(f1 != f2)
}

func f32lt(ins *Instance) {
	f2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	f1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.pushBool(f1 < f2)
}

func f32gt(ins *Instance) {
	f1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	f2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.pushBool(f1 > f2)
}

func f32le(ins *Instance) {
	f2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	f1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.pushBool(f1 <= f2)
}

func f32ge(ins *Instance) {
	f2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	f1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.pushBool(f1 >= f2)
}

func f64eq(ins *Instance) {
	f2 := math.Float64frombits(ins.OperandStack.pop())
	f1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.pushBool(f1 == f2)
}
func f64ne(ins *Instance) {
	f2 := math.Float64frombits(ins.OperandStack.pop())
	f1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.pushBool(f1 != f2)
}

func f64lt(ins *Instance) {
	f2 := math.Float64frombits(ins.OperandStack.pop())
	f1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.pushBool(f1 < f2)
}

func f64gt(ins *Instance) {
	f2 := math.Float64frombits(ins.OperandStack.pop())
	f1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.pushBool(f1 > f2)
}

func f64le(ins *Instance) {
	f2 := math.Float64frombits(ins.OperandStack.pop())
	f1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.pushBool(f1 <= f2)
}

func f64ge(ins *Instance) {
	f2 := math.Float64frombits(ins.OperandStack.pop())
	f1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.pushBool(f1 >= f2)
}

func i32clz(ins *Instance) {
	ins.OperandStack.push(uint64(bits.LeadingZeros32(uint32(ins.OperandStack.pop()))))
}

func i32ctz(ins *Instance) {
	ins.OperandStack.push(uint64(bits.TrailingZeros32(uint32(ins.OperandStack.pop()))))
}

func i32popcnt(ins *Instance) {
	ins.OperandStack.push(uint64(bits.OnesCount32(uint32(ins.OperandStack.pop()))))
}

func i32add(ins *Instance) {
	ins.OperandStack.push(uint64(uint32(ins.OperandStack.pop()) + uint32(ins.OperandStack.pop())))
}

func i32sub(ins *Instance) {
	v2 := uint32(ins.OperandStack.pop())
	v1 := uint32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 - v2))
}

func i32mul(ins *Instance) {
	ins.OperandStack.push(uint64(uint32(ins.OperandStack.pop()) * uint32(ins.OperandStack.pop())))
}

func i32divs(ins *Instance) {
	v2 := int32(ins.OperandStack.pop())
	v1 := int32(ins.OperandStack.pop())
	if v2 == 0 || (v1 == math.MinInt32 && v2 == -1) {
		panic("undefined")
	}
	ins.OperandStack.push(uint64(v1 / v2))
}

func i32divu(ins *Instance) {
	v2 := uint32(ins.OperandStack.pop())
	v1 := uint32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 / v2))
}

func i32rems(ins *Instance) {
	v2 := int32(ins.OperandStack.pop())
	v1 := int32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 % v2))
}

func i32remu(ins *Instance) {
	v2 := uint32(ins.OperandStack.pop())
	v1 := uint32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 % v2))
}

func i32and(ins *Instance) {
	ins.OperandStack.push(uint64(uint32(ins.OperandStack.pop()) & uint32(ins.OperandStack.pop())))
}

func i32or(ins *Instance) {
	ins.OperandStack.push(uint64(uint32(ins.OperandStack.pop()) | uint32(ins.OperandStack.pop())))
}

func i32xor(ins *Instance) {
	ins.OperandStack.push(uint64(uint32(ins.OperandStack.pop()) ^ uint32(ins.OperandStack.pop())))
}

func i32shl(ins *Instance) {
	v2 := uint32(ins.OperandStack.pop())
	v1 := uint32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 << (v2 % 32)))
}

func i32shru(ins *Instance) {
	v2 := uint32(ins.OperandStack.pop())
	v1 := uint32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 >> (v2 % 32)))
}

func i32shrs(ins *Instance) {
	v2 := uint32(ins.OperandStack.pop())
	v1 := int32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 >> (v2 % 32)))
}

func i32rotl(ins *Instance) {
	v2 := int(ins.OperandStack.pop())
	v1 := uint32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(bits.RotateLeft32(v1, v2)))
}

func i32rotr(ins *Instance) {
	v2 := int(ins.OperandStack.pop())
	v1 := uint32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(bits.RotateLeft32(v1, -v2)))
}

// i64
func i64clz(ins *Instance) {
	ins.OperandStack.push(uint64(bits.LeadingZeros64(ins.OperandStack.pop())))
}

func i64ctz(ins *Instance) {
	ins.OperandStack.push(uint64(bits.TrailingZeros64(ins.OperandStack.pop())))
}

func i64popcnt(ins *Instance) {
	ins.OperandStack.push(uint64(bits.OnesCount64(ins.OperandStack.pop())))
}

func i64add(ins *Instance) {
	ins.OperandStack.push(ins.OperandStack.pop() + ins.OperandStack.pop())
}

func i64sub(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.push(v1 - v2)
}

func i64mul(ins *Instance) {
	ins.OperandStack.push(ins.OperandStack.pop() * ins.OperandStack.pop())
}

func i64divs(ins *Instance) {
	v2 := int64(ins.OperandStack.pop())
	v1 := int64(ins.OperandStack.pop())
	if v2 == 0 || (v1 == math.MinInt64 && v2 == -1) {
		panic("undefined")
	}
	ins.OperandStack.push(uint64(v1 / v2))
}

func i64divu(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.push(v1 / v2)
}

func i64rems(ins *Instance) {
	v2 := int64(ins.OperandStack.pop())
	v1 := int64(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 % v2))
}

func i64remu(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.push(v1 % v2)
}

func i64and(ins *Instance) {
	ins.OperandStack.push(ins.OperandStack.pop() & ins.OperandStack.pop())
}

func i64or(ins *Instance) {
	ins.OperandStack.push(ins.OperandStack.pop() | ins.OperandStack.pop())
}

func i64xor(ins *Instance) {
	ins.OperandStack.push(ins.OperandStack.pop() ^ ins.OperandStack.pop())
}

func i64shl(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.push(v1 << (v2 % 64))
}

func i64shru(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := ins.OperandStack.pop()
	ins.OperandStack.push(v1 >> (v2 % 64))
}

func i64shrs(ins *Instance) {
	v2 := ins.OperandStack.pop()
	v1 := int64(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(v1 >> (v2 % 64)))
}

func i64rotl(ins *Instance) {
	v2 := int(ins.OperandStack.pop())
	v1 := ins.OperandStack.pop()
	ins.OperandStack.push(bits.RotateLeft64(v1, v2))
}

func i64rotr(ins *Instance) {
	v2 := int(ins.OperandStack.pop())
	v1 := ins.OperandStack.pop()
	ins.OperandStack.push(bits.RotateLeft64(v1, -v2))
}

func f32abs(ins *Instance) {
	const mask uint32 = 1 << 31
	v := uint32(ins.OperandStack.pop()) &^ mask
	ins.OperandStack.push(uint64(v))
}

func f32neg(ins *Instance) {
	v := -math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v)))
}

func f32ceil(ins *Instance) {
	v := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(float32(math.Ceil(float64(v))))))
}

func f32floor(ins *Instance) {
	v := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(float32(math.Floor(float64(v))))))
}

func f32trunc(ins *Instance) {
	v := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(float32(math.Trunc(float64(v))))))
}

func f32nearest(ins *Instance) {
	raw := math.Float32frombits(uint32(ins.OperandStack.pop()))
	v := math.Float64bits(float64(int32(raw + float32(math.Copysign(0.5, float64(raw))))))
	ins.OperandStack.push(v)
}

func f32sqrt(ins *Instance) {
	v := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(float32(math.Sqrt(float64(v))))))
}

func f32add(ins *Instance) {
	v := math.Float32frombits(uint32(ins.OperandStack.pop())) + math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v)))
}

func f32sub(ins *Instance) {
	v2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	v1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v1 - v2)))
}

func f32mul(ins *Instance) {
	v := math.Float32frombits(uint32(ins.OperandStack.pop())) * math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v)))
}

func f32div(ins *Instance) {
	v2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	v1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v1 / v2)))
}

func f32min(ins *Instance) {
	v2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	v1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(float32(math.Min(float64(v1), float64(v2))))))
}

func f32max(ins *Instance) {
	v2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	v1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(float32(math.Min(float64(v1), float64(v2))))))
}

func f32copysign(ins *Instance) {
	v2 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	v1 := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(float32(math.Copysign(float64(v1), float64(v2))))))
}

func f64abs(ins *Instance) {
	const mask = 1 << 63
	v := ins.OperandStack.pop() &^ mask
	ins.OperandStack.push(v)
}

func f64neg(ins *Instance) {
	v := -math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(v))
}

func f64ceil(ins *Instance) {
	v := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(math.Ceil(v)))
}

func f64floor(ins *Instance) {
	v := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(math.Floor(v)))
}

func f64trunc(ins *Instance) {
	v := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(math.Trunc(v)))
}

func f64nearest(ins *Instance) {
	raw := math.Float64frombits(ins.OperandStack.pop())
	v := math.Float64bits(float64(int64(raw + math.Copysign(0.5, raw))))
	ins.OperandStack.push(v)
}

func f64sqrt(ins *Instance) {
	v := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(math.Sqrt(v)))
}

func f64add(ins *Instance) {
	v := math.Float64frombits(ins.OperandStack.pop()) + math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(v))
}

func f64sub(ins *Instance) {
	v2 := math.Float64frombits(ins.OperandStack.pop())
	v1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(v1 - v2))
}

func f64mul(ins *Instance) {
	v := math.Float64frombits(ins.OperandStack.pop()) * math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(v))
}

func f64div(ins *Instance) {
	v2 := math.Float64frombits(ins.OperandStack.pop())
	v1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(v1 / v2))
}

func f64min(ins *Instance) {
	v2 := math.Float64frombits(ins.OperandStack.pop())
	v1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(math.Min(v1, v2)))
}

func f64max(ins *Instance) {
	v2 := math.Float64frombits(ins.OperandStack.pop())
	v1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(math.Min(v1, v2)))
}

func f64copysign(ins *Instance) {
	v2 := math.Float64frombits(ins.OperandStack.pop())
	v1 := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(math.Copysign(v1, v2)))
}

func i32wrapi64(ins *Instance) {
	ins.OperandStack.push(uint64(uint32(ins.OperandStack.pop())))
}

func i32truncf32s(ins *Instance) {
	v := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(int32(math.Trunc(float64(v)))))
}

func i32truncf32u(ins *Instance) {
	v := math.Float32frombits(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(uint32(math.Trunc(float64(v)))))
}

func i32truncf64s(ins *Instance) {
	v := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(int32(math.Trunc(v))))
}

func i32truncf64u(ins *Instance) {
	v := math.Float64frombits(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(uint32(math.Trunc(v))))
}

func i64extendi32s(ins *Instance) {
	v := int64(int32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(v))
}

func i64extendi32u(ins *Instance) {
	v := uint64(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(v)
}

func i64truncf32s(ins *Instance) {
	v := math.Trunc(float64(math.Float32frombits(uint32(ins.OperandStack.pop()))))
	ins.OperandStack.push(uint64(int64(v)))
}

func i64truncf32u(ins *Instance) {
	v := math.Trunc(float64(math.Float32frombits(uint32(ins.OperandStack.pop()))))
	ins.OperandStack.push(uint64(v))
}

func i64truncf64s(ins *Instance) {
	v := math.Trunc(math.Float64frombits(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(int64(v)))
}

func i64truncf64u(ins *Instance) {
	v := math.Trunc(math.Float64frombits(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(v))
}

func f32converti32s(ins *Instance) {
	v := float32(int32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v)))
}

func f32converti32u(ins *Instance) {
	v := float32(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v)))
}

func f32converti64s(ins *Instance) {
	v := float32(int64(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v)))
}

func f32converti64u(ins *Instance) {
	v := float32(ins.OperandStack.pop())
	ins.OperandStack.push(uint64(math.Float32bits(v)))
}

func f32demotef64(ins *Instance) {
	v := float32(math.Float64frombits(ins.OperandStack.pop()))
	ins.OperandStack.push(uint64(math.Float32bits(v)))
}

func f64converti32s(ins *Instance) {
	v := float64(int32(ins.OperandStack.pop()))
	ins.OperandStack.push(math.Float64bits(v))
}

func f64converti32u(ins *Instance) {
	v := float64(uint32(ins.OperandStack.pop()))
	ins.OperandStack.push(math.Float64bits(v))
}

func f64converti64s(ins *Instance) {
	v := float64(int64(ins.OperandStack.pop()))
	ins.OperandStack.push(math.Float64bits(v))
}

func f64converti64u(ins *Instance) {
	v := float64(ins.OperandStack.pop())
	ins.OperandStack.push(math.Float64bits(v))
}

func f64promotef32(ins *Instance) {
	v := float64(math.Float32frombits(uint32(ins.OperandStack.pop())))
	ins.OperandStack.push(math.Float64bits(v))
}