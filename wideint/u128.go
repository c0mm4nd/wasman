// Package wideint implements 128- and 256-bit integer arithmetic for the
// optional wide-integer host module (config.ModuleConfig.EnableWideInt).
//
// Values are little-endian limb sequences, matching the memory layout of
// __int128 on every wasm toolchain and of common big-number libraries.
// Division follows EVM conventions: division by zero yields zero (no
// trap), signed division truncates toward zero and MinValue / -1 wraps
// back to MinValue.
package wideint

import (
	"encoding/binary"
	"math/big"
	"math/bits"
)

// U128 is a 128-bit integer as two little-endian 64-bit limbs.
type U128 struct{ Lo, Hi uint64 }

// U128FromBytes reads 16 little-endian bytes.
func U128FromBytes(b []byte) U128 {
	return U128{Lo: binary.LittleEndian.Uint64(b), Hi: binary.LittleEndian.Uint64(b[8:])}
}

// PutBytes writes 16 little-endian bytes.
func (a U128) PutBytes(b []byte) {
	binary.LittleEndian.PutUint64(b, a.Lo)
	binary.LittleEndian.PutUint64(b[8:], a.Hi)
}

// Add returns a + b (wrapping).
func (a U128) Add(b U128) U128 {
	lo, c := bits.Add64(a.Lo, b.Lo, 0)
	hi, _ := bits.Add64(a.Hi, b.Hi, c)
	return U128{lo, hi}
}

// Sub returns a - b (wrapping).
func (a U128) Sub(b U128) U128 {
	lo, brw := bits.Sub64(a.Lo, b.Lo, 0)
	hi, _ := bits.Sub64(a.Hi, b.Hi, brw)
	return U128{lo, hi}
}

// Mul returns a * b (wrapping).
func (a U128) Mul(b U128) U128 {
	hi, lo := bits.Mul64(a.Lo, b.Lo)
	hi += a.Lo*b.Hi + a.Hi*b.Lo
	return U128{lo, hi}
}

// IsZero reports a == 0.
func (a U128) IsZero() bool { return a.Lo == 0 && a.Hi == 0 }

// CmpU compares unsigned: -1, 0 or 1.
func (a U128) CmpU(b U128) int {
	if a.Hi != b.Hi {
		if a.Hi < b.Hi {
			return -1
		}
		return 1
	}
	if a.Lo != b.Lo {
		if a.Lo < b.Lo {
			return -1
		}
		return 1
	}
	return 0
}

// Neg returns two's complement negation.
func (a U128) Neg() U128 { return U128{}.Sub(a) }

// Sign reports the two's-complement sign bit.
func (a U128) Sign() bool { return a.Hi>>63 != 0 }

// CmpS compares signed: -1, 0 or 1.
func (a U128) CmpS(b U128) int {
	as, bs := a.Sign(), b.Sign()
	if as != bs {
		if as {
			return -1
		}
		return 1
	}
	return a.CmpU(b)
}

// And, Or, Xor, Not are the bitwise operations.
func (a U128) And(b U128) U128 { return U128{a.Lo & b.Lo, a.Hi & b.Hi} }
func (a U128) Or(b U128) U128  { return U128{a.Lo | b.Lo, a.Hi | b.Hi} }
func (a U128) Xor(b U128) U128 { return U128{a.Lo ^ b.Lo, a.Hi ^ b.Hi} }
func (a U128) Not() U128       { return U128{^a.Lo, ^a.Hi} }

// Shl shifts left; n >= 128 yields zero.
func (a U128) Shl(n uint) U128 {
	switch {
	case n >= 128:
		return U128{}
	case n >= 64:
		return U128{0, a.Lo << (n - 64)}
	case n == 0:
		return a
	}
	return U128{a.Lo << n, a.Hi<<n | a.Lo>>(64-n)}
}

// ShrU shifts right logically; n >= 128 yields zero.
func (a U128) ShrU(n uint) U128 {
	switch {
	case n >= 128:
		return U128{}
	case n >= 64:
		return U128{a.Hi >> (n - 64), 0}
	case n == 0:
		return a
	}
	return U128{a.Lo>>n | a.Hi<<(64-n), a.Hi >> n}
}

// ShrS shifts right arithmetically; n >= 128 fills with the sign.
func (a U128) ShrS(n uint) U128 {
	fill := uint64(0)
	if a.Sign() {
		fill = ^uint64(0)
	}
	switch {
	case n >= 128:
		return U128{fill, fill}
	case n >= 64:
		return U128{uint64(int64(a.Hi) >> (n - 64)), fill}
	case n == 0:
		return a
	}
	return U128{a.Lo>>n | a.Hi<<(64-n), uint64(int64(a.Hi) >> n)}
}

// DivU and RemU are unsigned division; division by zero yields zero.
func (a U128) DivU(b U128) U128 { q, _ := a.divRemU(b); return q }
func (a U128) RemU(b U128) U128 { _, r := a.divRemU(b); return r }

func (a U128) divRemU(b U128) (U128, U128) {
	if b.IsZero() {
		return U128{}, U128{}
	}
	if b.Hi == 0 {
		if a.Hi < b.Lo {
			lo, rem := bits.Div64(a.Hi, a.Lo, b.Lo)
			return U128{lo, 0}, U128{rem, 0}
		}
		qhi, rhi := a.Hi/b.Lo, a.Hi%b.Lo
		qlo, rem := bits.Div64(rhi, a.Lo, b.Lo)
		return U128{qlo, qhi}, U128{rem, 0}
	}
	// divisor spans both limbs: rare path through math/big
	q, r := new(big.Int).QuoRem(a.big(), b.big(), new(big.Int))
	return u128FromBig(q), u128FromBig(r)
}

// DivS and RemS are signed (truncated); by-zero yields zero, and the
// remainder's sign follows the dividend.
func (a U128) DivS(b U128) U128 {
	if b.IsZero() {
		return U128{}
	}
	an, bn := a.Sign(), b.Sign()
	x, y := a, b
	if an {
		x = x.Neg()
	}
	if bn {
		y = y.Neg()
	}
	q := x.DivU(y)
	if an != bn {
		q = q.Neg()
	}
	return q
}

func (a U128) RemS(b U128) U128 {
	if b.IsZero() {
		return U128{}
	}
	an, bn := a.Sign(), b.Sign()
	x, y := a, b
	if an {
		x = x.Neg()
	}
	if bn {
		y = y.Neg()
	}
	r := x.RemU(y)
	if an {
		r = r.Neg()
	}
	return r
}

func (a U128) big() *big.Int {
	return new(big.Int).SetBits([]big.Word{big.Word(a.Lo), big.Word(a.Hi)})
}

func u128FromBig(x *big.Int) U128 {
	var u U128
	w := x.Bits()
	if len(w) > 0 {
		u.Lo = uint64(w[0])
	}
	if len(w) > 1 {
		u.Hi = uint64(w[1])
	}
	return u
}
