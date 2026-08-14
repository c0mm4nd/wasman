//go:build (darwin || linux) && amd64

package jit

import (
	"fmt"
	"math"
)

const (
	ccP  = 0xA // parity: unordered after UCOMIS
	ccNP = 0xB
	ccS  = 0x8 // sign
)

const (
	xmm0 = 0
	xmm1 = 1
	xmm2 = 2
)

// emitFloat handles the float opcode ranges; the operand stack keeps f32 as
// zero-extended bit patterns and f64 as raw bits, exactly like the
// interpreter, so moves through the GP registers are bit-preserving.
func (c *compiler) emitFloat(op byte) error {
	a := &c.a
	switch {
	// comparisons: UCOMIS + SETcc, with parity fixups for eq/ne and operand
	// swaps so unordered comes out false for the ordered predicates
	case op >= 0x5b && op <= 0x66:
		d := op >= 0x61
		rel := op - 0x5b
		if d {
			rel = op - 0x61
		}
		a.LdSlot(rCX, c.pop()) // v2
		a.LdSlot(rAX, c.pop()) // v1
		mov := a.MovdXR
		if d {
			mov = a.MovqXR
		}
		mov(xmm0, rAX)
		mov(xmm1, rCX)
		switch rel {
		case 0: // eq: ordered and equal
			a.Ucomis(d, xmm0, xmm1)
			a.Setcc(ccE, rAX)
			a.Setcc(ccNP, rCX)
			a.AndByteAL()
		case 1: // ne: unequal or unordered
			a.Ucomis(d, xmm0, xmm1)
			a.Setcc(ccNE, rAX)
			a.Setcc(ccP, rCX)
			a.OrByteAL()
		case 2: // lt: b > a, ordered
			a.Ucomis(d, xmm1, xmm0)
			a.Setcc(ccA, rAX)
		case 3: // gt
			a.Ucomis(d, xmm0, xmm1)
			a.Setcc(ccA, rAX)
		case 4: // le
			a.Ucomis(d, xmm1, xmm0)
			a.Setcc(ccAE, rAX)
		case 5: // ge
			a.Ucomis(d, xmm0, xmm1)
			a.Setcc(ccAE, rAX)
		}
		a.MovzxB(rAX)
		a.StSlot(rAX, c.push())

	case op == 0x8b || op == 0x99: // abs: clear the sign bit (GP domain)
		c.fBitMask(op == 0x99, 0x7fffffff, 0x7fffffffffffffff, 0x21)
	case op == 0x8c || op == 0x9a: // neg: flip the sign bit
		c.fBitMask(op == 0x9a, 0x80000000, 0x8000000000000000, 0x31)

	case op == 0x98 || op == 0xa6: // copysign: v1 magnitude, v2 sign
		d := op == 0xa6
		mag, sign := uint64(0x7fffffff), uint64(0x80000000)
		if d {
			mag, sign = 0x7fffffffffffffff, 0x8000000000000000
		}
		a.LdSlot(rCX, c.pop())
		a.LdSlot(rAX, c.pop())
		a.MovImm64(rDX, mag)
		a.BinRR(true, 0x21, rAX, rDX) // AND
		a.MovImm64(rDX, sign)
		a.BinRR(true, 0x21, rCX, rDX)
		a.BinRR(true, 0x09, rAX, rCX) // OR
		a.StSlot(rAX, c.push())

	// unary: ceil/floor/trunc/nearest (ROUNDS modes 2/1/3/0) and sqrt
	case op >= 0x8d && op <= 0x91 || op >= 0x9b && op <= 0x9f:
		d := op >= 0x9b
		rel := op - 0x8d
		if d {
			rel = op - 0x9b
		}
		c.fpLoad1(d)
		if rel == 4 {
			pre := byte(0xF3)
			if d {
				pre = 0xF2
			}
			a.sse(pre, 0x51, xmm0, xmm0) // SQRT
		} else {
			modes := [4]byte{2, 1, 3, 0}
			a.Rounds(d, xmm0, xmm0, modes[rel])
		}
		c.fpStore1(d)

	// binary arithmetic: add/sub/mul/div
	case op >= 0x92 && op <= 0x95 || op >= 0xa0 && op <= 0xa3:
		d := op >= 0xa0
		rel := op - 0x92
		if d {
			rel = op - 0xa0
		}
		pre := byte(0xF3)
		if d {
			pre = 0xF2
		}
		ops := [4]byte{0x58, 0x5C, 0x59, 0x5E}
		c.fpLoad2(d)
		a.sse(pre, ops[rel], xmm0, xmm1)
		c.fpPushResult(d)

	// min/max: NaN -> canonical literal; equal operands merge sign bits so
	// (-0,+0) resolves correctly; otherwise MINS/MAXS is exact
	case op == 0x96 || op == 0x97 || op == 0xa4 || op == 0xa5:
		d := op >= 0xa4
		isMin := op == 0x96 || op == 0xa4
		canon := uint64(0x7fc00000)
		if d {
			canon = 0x7ff8000000000000
		}
		pre := byte(0xF3)
		if d {
			pre = 0xF2
		}
		sseOp := byte(0x5D) // MIN
		if !isMin {
			sseOp = 0x5F
		}
		gpOp := byte(0x09) // OR merges signs for min
		if !isMin {
			gpOp = 0x21 // AND for max
		}
		a.LdSlot(rCX, c.pop())
		a.LdSlot(rAX, c.pop())
		mov := a.MovdXR
		movBack := a.MovdRX
		if d {
			mov, movBack = a.MovqXR, a.MovqRX
		}
		mov(xmm0, rAX)
		mov(xmm1, rCX)
		a.Ucomis(d, xmm0, xmm1)
		atNaN := a.Len()
		a.Jcc(ccP, 0)
		atOp := a.Len()
		a.Jcc(ccNE, 0)
		a.BinRR(true, gpOp, rAX, rCX) // equal: combine the raw bit patterns
		done1 := a.Len()
		a.Jmp(0)
		a.PatchJcc(atOp)
		a.sse(pre, sseOp, xmm0, xmm1)
		movBack(rAX, xmm0)
		done2 := a.Len()
		a.Jmp(0)
		a.PatchJcc(atNaN)
		a.MovImm64(rAX, canon)
		a.PatchJmp(done1)
		a.PatchJmp(done2)
		a.StSlot(rAX, c.push())

	// trapping float->int truncations, mirroring the interpreter's
	// truncCheck in the float64 domain
	case op >= 0xa8 && op <= 0xab || op >= 0xae && op <= 0xb1:
		c.emitTrunc(op)

	// int->float conversions
	case op >= 0xb2 && op <= 0xb5 || op >= 0xb7 && op <= 0xba:
		d := op >= 0xb7
		rel := op - 0xb2
		if d {
			rel = op - 0xb7
		}
		signed := rel == 0 || rel == 2
		from64 := rel >= 2
		a.LdSlot(rAX, c.h-1)
		switch {
		case !from64 && signed: // i32_s: use the sign-extended low half
			a.Movsxd(rAX, rAX)
			a.Cvtsi2f(d, true, xmm0, rAX)
		case !from64 && !signed: // i32_u: zero-extend, then signed 64-bit
			a.MovRR32(rAX, rAX)
			a.Cvtsi2f(d, true, xmm0, rAX)
		case signed: // i64_s
			a.Cvtsi2f(d, true, xmm0, rAX)
		default: // i64_u: halve-and-double for the high-bit range
			a.TestRR(true, rAX)
			atBig := a.Len()
			a.Jcc(ccS, 0)
			a.Cvtsi2f(d, true, xmm0, rAX)
			done := a.Len()
			a.Jmp(0)
			a.PatchJcc(atBig)
			a.BinRR(true, 0x89, rCX, rAX) // MOV RCX, RAX
			a.AndImm32(true, rCX, 1)
			a.ShiftImm(true, 5, rAX, 1) // SHR
			a.BinRR(true, 0x09, rAX, rCX)
			a.Cvtsi2f(d, true, xmm0, rAX)
			pre := byte(0xF3)
			if d {
				pre = 0xF2
			}
			a.sse(pre, 0x58, xmm0, xmm0) // ADD to itself: x2
			a.PatchJmp(done)
		}
		c.fpStore1(d)

	case op == 0xb6: // f32.demote_f64
		a.LdSlot(rAX, c.h-1)
		a.MovqXR(xmm0, rAX)
		a.sse(0xF2, 0x5A, xmm0, xmm0) // CVTSD2SS
		a.MovdRX(rAX, xmm0)
		a.StSlot(rAX, c.h-1)
	case op == 0xbb: // f64.promote_f32
		a.LdSlot(rAX, c.h-1)
		a.MovdXR(xmm0, rAX)
		a.sse(0xF3, 0x5A, xmm0, xmm0) // CVTSS2SD
		a.MovqRX(rAX, xmm0)
		a.StSlot(rAX, c.h-1)

	case op >= 0xbc && op <= 0xbf: // reinterpret: bit-preserving no-ops

	default:
		return fmt.Errorf("%w: opcode %#x", ErrUnsupported, op)
	}
	return nil
}

// fBitMask applies a masked GP bit op to the top slot (abs/neg).
func (c *compiler) fBitMask(d bool, m32, m64 uint64, gpOp byte) {
	a := &c.a
	mask := m32
	if d {
		mask = m64
	}
	a.LdSlot(rAX, c.h-1)
	a.MovImm64(rCX, mask)
	a.BinRR(true, gpOp, rAX, rCX)
	a.StSlot(rAX, c.h-1)
}

// fpLoad1/fpStore1 move the top slot into/out of xmm0.
func (c *compiler) fpLoad1(d bool) {
	c.a.LdSlot(rAX, c.h-1)
	if d {
		c.a.MovqXR(xmm0, rAX)
	} else {
		c.a.MovdXR(xmm0, rAX)
	}
}

func (c *compiler) fpStore1(d bool) {
	if d {
		c.a.MovqRX(rAX, xmm0)
	} else {
		c.a.MovdRX(rAX, xmm0) // 32-bit move keeps the pattern zero-extended
	}
	c.a.StSlot(rAX, c.h-1)
}

// fpLoad2 pops v2 into xmm1 and v1 into xmm0.
func (c *compiler) fpLoad2(d bool) {
	c.a.LdSlot(rCX, c.pop())
	c.a.LdSlot(rAX, c.pop())
	if d {
		c.a.MovqXR(xmm0, rAX)
		c.a.MovqXR(xmm1, rCX)
	} else {
		c.a.MovdXR(xmm0, rAX)
		c.a.MovdXR(xmm1, rCX)
	}
}

func (c *compiler) fpPushResult(d bool) {
	if d {
		c.a.MovqRX(rAX, xmm0)
	} else {
		c.a.MovdRX(rAX, xmm0)
	}
	c.a.StSlot(rAX, c.push())
}

// truncBounds gives the interpreter's [lo, hi) window per opcode.
var truncBounds = map[byte][2]float64{
	0xa8: {-2147483648.0, 2147483648.0}, 0xa9: {0.0, 4294967296.0},
	0xaa: {-2147483648.0, 2147483648.0}, 0xab: {0.0, 4294967296.0},
	0xae: {-9223372036854775808.0, 9223372036854775808.0}, 0xaf: {0.0, 18446744073709551616.0},
	0xb0: {-9223372036854775808.0, 9223372036854775808.0}, 0xb1: {0.0, 18446744073709551616.0},
}

func (c *compiler) emitTrunc(op byte) {
	a := &c.a
	fromF32 := op == 0xa8 || op == 0xa9 || op == 0xae || op == 0xaf
	to64 := op >= 0xae
	signed := op == 0xa8 || op == 0xaa || op == 0xae || op == 0xb0
	b := truncBounds[op]

	a.LdSlot(rAX, c.pop())
	if fromF32 { // promote into the float64 comparison domain
		a.MovdXR(xmm0, rAX)
		a.sse(0xF3, 0x5A, xmm0, xmm0) // CVTSS2SD
	} else {
		a.MovqXR(xmm0, rAX)
	}
	// NaN -> invalid conversion
	a.Ucomis(true, xmm0, xmm0)
	ok := a.Len()
	a.Jcc(ccNP, 0)
	c.trap(StatusConvInvalid)
	a.PatchJcc(ok)
	// xmm1 = trunc(xmm0); trap unless lo <= t < hi
	a.Rounds(true, xmm1, xmm0, 3)
	a.MovImm64(rCX, math.Float64bits(b[0]))
	a.MovqXR(xmm2, rCX)
	a.Ucomis(true, xmm1, xmm2)
	okLo := a.Len()
	a.Jcc(ccAE, 0)
	c.trap(StatusConvOverflow)
	a.PatchJcc(okLo)
	a.MovImm64(rCX, math.Float64bits(b[1]))
	a.MovqXR(xmm2, rCX)
	a.Ucomis(true, xmm1, xmm2)
	okHi := a.Len()
	a.Jcc(ccB, 0)
	c.trap(StatusConvOverflow)
	a.PatchJcc(okHi)
	switch {
	case signed && to64:
		a.Cvttf2i(true, true, rAX, xmm1)
	case signed: // i32_s: 32-bit convert, zero-extended like the interpreter
		a.Cvttf2i(true, false, rAX, xmm1)
	case !to64: // i32_u fits in a signed 64-bit convert
		a.Cvttf2i(true, true, rAX, xmm1)
		a.MovRR32(rAX, rAX)
	default: // i64_u: values >= 2^63 go through a bias
		a.MovImm64(rCX, math.Float64bits(9223372036854775808.0))
		a.MovqXR(xmm2, rCX)
		a.Ucomis(true, xmm1, xmm2)
		atSmall := a.Len()
		a.Jcc(ccB, 0)
		a.sse(0xF2, 0x5C, xmm1, xmm2) // SUBSD: t - 2^63
		a.Cvttf2i(true, true, rAX, xmm1)
		a.MovImm64(rCX, 0x8000000000000000)
		a.BinRR(true, 0x01, rAX, rCX) // ADD back the bias
		done := a.Len()
		a.Jmp(0)
		a.PatchJcc(atSmall)
		a.Cvttf2i(true, true, rAX, xmm1)
		a.PatchJmp(done)
	}
	a.StSlot(rAX, c.push())
}

// satRange gives the clamp window and saturated results per trunc_sat sub-op.
var satRange = [8]struct {
	lo, hi   float64
	min, max uint64
}{
	{-2147483648.0, 2147483648.0, 0x80000000, 0x7fffffff},                                   // i32 f32_s
	{0, 4294967296.0, 0, 0xffffffff},                                                        // i32 f32_u
	{-2147483648.0, 2147483648.0, 0x80000000, 0x7fffffff},                                   // i32 f64_s
	{0, 4294967296.0, 0, 0xffffffff},                                                        // i32 f64_u
	{-9223372036854775808.0, 9223372036854775808.0, 0x8000000000000000, 0x7fffffffffffffff}, // i64 f32_s
	{0, 18446744073709551616.0, 0, 0xffffffffffffffff},                                      // i64 f32_u
	{-9223372036854775808.0, 9223372036854775808.0, 0x8000000000000000, 0x7fffffffffffffff}, // i64 f64_s
	{0, 18446744073709551616.0, 0, 0xffffffffffffffff},                                      // i64 f64_u
}

// emitTruncSat lowers a saturating float->int conversion: NaN -> 0, below
// the window -> min, at/above -> max, else truncate (interpreter parity).
func (c *compiler) emitTruncSat(sub byte) {
	a := &c.a
	r := satRange[sub]
	fromF32 := sub&2 == 0
	to64 := sub >= 4
	signed := sub&1 == 0

	a.LdSlot(rAX, c.pop())
	if fromF32 {
		a.MovdXR(xmm0, rAX)
		a.sse(0xF3, 0x5A, xmm0, xmm0) // promote to the float64 domain
	} else {
		a.MovqXR(xmm0, rAX)
	}
	var dones []int
	// NaN -> 0
	a.Ucomis(true, xmm0, xmm0)
	ok := a.Len()
	a.Jcc(ccNP, 0)
	a.BinRR(true, 0x31, rAX, rAX) // XOR: result 0
	dones = append(dones, a.Len())
	a.Jmp(0)
	a.PatchJcc(ok)
	// t = trunc(v); clamp against [lo, hi)
	a.Rounds(true, xmm1, xmm0, 3)
	a.MovImm64(rCX, math.Float64bits(r.lo))
	a.MovqXR(xmm2, rCX)
	a.Ucomis(true, xmm1, xmm2)
	okLo := a.Len()
	a.Jcc(ccAE, 0)
	a.MovImm64(rAX, r.min)
	dones = append(dones, a.Len())
	a.Jmp(0)
	a.PatchJcc(okLo)
	a.MovImm64(rCX, math.Float64bits(r.hi))
	a.MovqXR(xmm2, rCX)
	a.Ucomis(true, xmm1, xmm2)
	okHi := a.Len()
	a.Jcc(ccB, 0)
	a.MovImm64(rAX, r.max)
	dones = append(dones, a.Len())
	a.Jmp(0)
	a.PatchJcc(okHi)
	// in-window conversion (same shapes as the trapping variant)
	switch {
	case signed && to64:
		a.Cvttf2i(true, true, rAX, xmm1)
	case signed:
		a.Cvttf2i(true, false, rAX, xmm1)
	case !to64:
		a.Cvttf2i(true, true, rAX, xmm1)
		a.MovRR32(rAX, rAX)
	default: // u64 via the 2^63 bias
		a.MovImm64(rCX, math.Float64bits(9223372036854775808.0))
		a.MovqXR(xmm2, rCX)
		a.Ucomis(true, xmm1, xmm2)
		atSmall := a.Len()
		a.Jcc(ccB, 0)
		a.sse(0xF2, 0x5C, xmm1, xmm2)
		a.Cvttf2i(true, true, rAX, xmm1)
		a.MovImm64(rCX, 0x8000000000000000)
		a.BinRR(true, 0x01, rAX, rCX)
		done := a.Len()
		a.Jmp(0)
		a.PatchJcc(atSmall)
		a.Cvttf2i(true, true, rAX, xmm1)
		a.PatchJmp(done)
	}
	for _, at := range dones {
		a.PatchJmp(at)
	}
	a.StSlot(rAX, c.push())
}
