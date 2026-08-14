//go:build (darwin || linux) && arm64

package jit

import (
	"fmt"
	"math"
)

// float instruction encodings: FP<->GP moves, arithmetic, compares,
// conversions. S-form and D-form differ by one bit group, selected by `d`.
func fsel(d bool, dw, sw uint32) uint32 {
	if d {
		return dw
	}
	return sw
}

// FMovToFP moves a GP register's bits into V0/V1/V2 (f index).
func (a *Asm) FMovToFP(d bool, f, r uint32) {
	a.word(fsel(d, 0x9E670000, 0x1E270000) | r<<5 | f)
}

// FMovFromFP moves V<f>'s bits into a GP register.
func (a *Asm) FMovFromFP(d bool, r, f uint32) {
	a.word(fsel(d, 0x9E660000, 0x1E260000) | f<<5 | r)
}

// FBin emits a two-operand float op into V0 (base encodings hold S-form).
func (a *Asm) FBin(d bool, base uint32) {
	if d {
		base |= 0x00400000
	}
	a.word(base | 1<<16 | 0<<5 | 0) // op V0, V0, V1
}

// FUn emits a one-operand float op V0 <- op(V0).
func (a *Asm) FUn(d bool, base uint32) {
	if d {
		base |= 0x00400000
	}
	a.word(base)
}

// FCmp emits FCMP V<n>, V<m>.
func (a *Asm) FCmp(d bool, n, m uint32) {
	a.word(fsel(d, 0x1E602000, 0x1E202000) | m<<16 | n<<5)
}

const (
	condVS = 6 // unordered after FCMP
	condVC = 7 // ordered
	condMI = 4 // float less-than
)

// emitFloat handles the float opcode ranges; the operand stack keeps f32 as
// zero-extended bit patterns and f64 as raw bits, exactly like the
// interpreter, so moves through the GP registers are bit-preserving.
func (c *compiler) emitFloat(op byte) error {
	a := &c.a
	switch {
	// comparisons: fcmp + cset with unordered-false conditions (ne is true
	// on unordered, which NE already gives)
	case op >= 0x5b && op <= 0x66:
		d := op >= 0x61
		rel := op - 0x5b
		if d {
			rel = op - 0x61
		}
		c.ldr(RegT1, c.pop())
		c.ldr(RegT0, c.pop())
		a.FMovToFP(d, 0, RegT0)
		a.FMovToFP(d, 1, RegT1)
		a.FCmp(d, 0, 1)
		conds := [6]uint32{condEQ, condNE, condMI, condGT, condLS, condGE}
		a.Cset(RegT0, conds[rel])
		c.str(RegT0, c.push())

	case op == 0x8b || op == 0x99: // abs: clear the sign bit (GP domain)
		c.fBitMask(op == 0x99, 0x7fffffff, 0x7fffffffffffffff, 0x0A000000, 0x8A000000)
	case op == 0x8c || op == 0x9a: // neg: flip the sign bit
		c.fBitMask(op == 0x9a, 0x80000000, 0x8000000000000000, 0x4A000000, 0xCA000000)

	case op == 0x98 || op == 0xa6: // copysign: v1 magnitude, v2 sign
		d := op == 0xa6
		mag, sign := uint64(0x7fffffff), uint64(0x80000000)
		if d {
			mag, sign = 0x7fffffffffffffff, 0x8000000000000000
		}
		c.ldr(RegT1, c.pop())
		c.ldr(RegT0, c.pop())
		a.MovImm64(RegT2, mag)
		a.word(0x8A000000 | RegT2<<16 | RegT0<<5 | RegT0) // AND X0, X0, X2
		a.MovImm64(RegT2, sign)
		a.word(0x8A000000 | RegT2<<16 | RegT1<<5 | RegT1) // AND X1, X1, X2
		a.word(0xAA000000 | RegT1<<16 | RegT0<<5 | RegT0) // ORR
		c.str(RegT0, c.push())

	// unary: ceil/floor/trunc/nearest/sqrt
	case op >= 0x8d && op <= 0x91 || op >= 0x9b && op <= 0x9f:
		d := op >= 0x9b
		rel := op - 0x8d
		if d {
			rel = op - 0x9b
		}
		bases := [5]uint32{0x1E24C000, 0x1E254000, 0x1E25C000, 0x1E244000, 0x1E21C000}
		c.ldr(RegT0, c.h-1)
		a.FMovToFP(d, 0, RegT0)
		a.FUn(d, bases[rel])
		a.FMovFromFP(d, RegT0, 0)
		c.str(RegT0, c.h-1)

	// binary arithmetic: add/sub/mul/div
	case op >= 0x92 && op <= 0x95 || op >= 0xa0 && op <= 0xa3:
		d := op >= 0xa0
		rel := op - 0x92
		if d {
			rel = op - 0xa0
		}
		bases := [4]uint32{0x1E202800, 0x1E203800, 0x1E200800, 0x1E201800}
		c.ldr(RegT1, c.pop())
		c.ldr(RegT0, c.pop())
		a.FMovToFP(d, 0, RegT0)
		a.FMovToFP(d, 1, RegT1)
		a.FBin(d, bases[rel])
		a.FMovFromFP(d, RegT0, 0)
		c.str(RegT0, c.push())

	// min/max: any NaN operand produces the canonical NaN literal, exactly
	// like the interpreter; otherwise the hardware op matches (incl. -0)
	case op == 0x96 || op == 0x97 || op == 0xa4 || op == 0xa5:
		d := op >= 0xa4
		base := uint32(0x1E205800) // FMIN
		if op == 0x97 || op == 0xa5 {
			base = 0x1E204800 // FMAX
		}
		canon := uint64(0x7fc00000)
		if d {
			canon = 0x7ff8000000000000
		}
		c.ldr(RegT1, c.pop())
		c.ldr(RegT0, c.pop())
		a.FMovToFP(d, 0, RegT0)
		a.FMovToFP(d, 1, RegT1)
		a.FCmp(d, 0, 1)
		ordered := a.Len()
		a.Bcond(condVC, 0)
		a.MovImm64(RegT0, canon)
		done := a.Len()
		a.B(0)
		a.PatchBcond(ordered, condVC)
		a.FBin(d, base)
		a.FMovFromFP(d, RegT0, 0)
		a.PatchB(done)
		c.str(RegT0, c.push())

	// trapping float->int truncations, mirroring the interpreter's
	// truncCheck: NaN traps as invalid; trunc(v) outside [lo, hi) traps as
	// overflow; the comparison domain is always float64
	case op >= 0xa8 && op <= 0xab || op >= 0xae && op <= 0xb1:
		c.emitTrunc(op)

	// int->float conversions (exact hardware equivalents of Go's)
	case op >= 0xb2 && op <= 0xb5 || op >= 0xb7 && op <= 0xba:
		d := op >= 0xb7
		rel := op - 0xb2
		if d {
			rel = op - 0xb7
		}
		c.ldr(RegT0, c.h-1)
		signed := rel == 0 || rel == 2
		from64 := rel >= 2
		var w uint32
		if signed {
			w = fsel(d, 0x1E620000, 0x1E220000) // SCVTF
		} else {
			w = fsel(d, 0x1E630000, 0x1E230000) // UCVTF
		}
		if from64 {
			w |= 0x80000000 // sf: 64-bit source register
		}
		a.word(w | RegT0<<5 | 0)
		a.FMovFromFP(d, RegT0, 0)
		c.str(RegT0, c.h-1)

	case op == 0xb6: // f32.demote_f64
		c.ldr(RegT0, c.h-1)
		a.FMovToFP(true, 0, RegT0)
		a.word(0x1E624000) // FCVT S0, D0
		a.FMovFromFP(false, RegT0, 0)
		c.str(RegT0, c.h-1)
	case op == 0xbb: // f64.promote_f32
		c.ldr(RegT0, c.h-1)
		a.FMovToFP(false, 0, RegT0)
		a.word(0x1E22C000) // FCVT D0, S0
		a.FMovFromFP(true, RegT0, 0)
		c.str(RegT0, c.h-1)

	case op >= 0xbc && op <= 0xbf: // reinterpret: bit-preserving no-ops

	default:
		return fmt.Errorf("%w: opcode %#x", ErrUnsupported, op)
	}
	return nil
}

// fBitMask pops one value and applies a masked bit op (abs/neg).
func (c *compiler) fBitMask(d bool, m32, m64 uint64, opW, opX uint32) {
	a := &c.a
	mask, w := m32, opW
	if d {
		mask, w = m64, opX
	}
	c.ldr(RegT0, c.h-1)
	a.MovImm64(RegT2, mask)
	a.word(w | RegT2<<16 | RegT0<<5 | RegT0)
	if !d {
		a.Uxtw(RegT0, RegT0) // keep the f32 pattern zero-extended
	}
	c.str(RegT0, c.h-1)
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

	c.ldr(RegT0, c.pop())
	if fromF32 { // promote to the float64 comparison domain
		a.FMovToFP(false, 0, RegT0)
		a.word(0x1E22C000) // FCVT D0, S0
	} else {
		a.FMovToFP(true, 0, RegT0)
	}
	// NaN -> invalid conversion
	a.FCmp(true, 0, 0)
	ok := a.Len()
	a.Bcond(condVC, 0)
	a.Movz(RegStatus, StatusConvInvalid, 0)
	a.Ret()
	a.PatchBcond(ok, condVC)
	// D1 = trunc(D0); trap unless lo <= D1 < hi (infinities fail these)
	a.word(0x1E65C000 | 0<<5 | 1) // FRINTZ D1, D0
	a.MovImm64(RegT2, math.Float64bits(b[0]))
	a.FMovToFP(true, 2, RegT2)
	a.FCmp(true, 1, 2)
	okLo := a.Len()
	a.Bcond(condGE, 0)
	a.Movz(RegStatus, StatusConvOverflow, 0)
	a.Ret()
	a.PatchBcond(okLo, condGE)
	a.MovImm64(RegT2, math.Float64bits(b[1]))
	a.FMovToFP(true, 2, RegT2)
	a.FCmp(true, 1, 2)
	okHi := a.Len()
	a.Bcond(condMI, 0)
	a.Movz(RegStatus, StatusConvOverflow, 0)
	a.Ret()
	a.PatchBcond(okHi, condMI)
	// convert the truncated value; 32-bit targets stay zero-extended like
	// the interpreter's uint64(uint32(...)) representation
	w := uint32(0x1E780000) // FCVTZS Wd, Dn
	if !signed {
		w = 0x1E790000 // FCVTZU
	}
	if to64 {
		w |= 0x80000000
	}
	a.word(w | 1<<5 | RegT0)
	c.str(RegT0, c.push())
}
