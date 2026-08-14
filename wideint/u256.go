package wideint

import (
	"encoding/binary"
	"math/big"
	"math/bits"
)

// U256 is a 256-bit integer as four little-endian 64-bit limbs.
type U256 [4]uint64

// U256FromBytes reads 32 little-endian bytes.
func U256FromBytes(b []byte) U256 {
	var u U256
	for i := range u {
		u[i] = binary.LittleEndian.Uint64(b[i*8:])
	}
	return u
}

// PutBytes writes 32 little-endian bytes.
func (a U256) PutBytes(b []byte) {
	for i, w := range a {
		binary.LittleEndian.PutUint64(b[i*8:], w)
	}
}

// Add returns a + b (wrapping).
func (a U256) Add(b U256) U256 {
	var out U256
	c := uint64(0)
	for i := 0; i < 4; i++ {
		out[i], c = bits.Add64(a[i], b[i], c)
	}
	return out
}

// Sub returns a - b (wrapping).
func (a U256) Sub(b U256) U256 {
	var out U256
	brw := uint64(0)
	for i := 0; i < 4; i++ {
		out[i], brw = bits.Sub64(a[i], b[i], brw)
	}
	return out
}

// Mul returns a * b (wrapping schoolbook, low 256 bits).
func (a U256) Mul(b U256) U256 {
	var out U256
	for i := 0; i < 4; i++ {
		carry := uint64(0)
		for j := 0; i+j < 4; j++ {
			hi, lo := bits.Mul64(a[i], b[j])
			var c1, c2 uint64
			out[i+j], c1 = bits.Add64(out[i+j], lo, 0)
			out[i+j], c2 = bits.Add64(out[i+j], carry, 0)
			carry = hi + c1 + c2
		}
	}
	return out
}

// IsZero reports a == 0.
func (a U256) IsZero() bool { return a[0]|a[1]|a[2]|a[3] == 0 }

// CmpU compares unsigned: -1, 0 or 1.
func (a U256) CmpU(b U256) int {
	for i := 3; i >= 0; i-- {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Neg returns two's complement negation.
func (a U256) Neg() U256 { return U256{}.Sub(a) }

// Sign reports the two's-complement sign bit.
func (a U256) Sign() bool { return a[3]>>63 != 0 }

// CmpS compares signed: -1, 0 or 1.
func (a U256) CmpS(b U256) int {
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
func (a U256) And(b U256) U256 {
	return U256{a[0] & b[0], a[1] & b[1], a[2] & b[2], a[3] & b[3]}
}
func (a U256) Or(b U256) U256 {
	return U256{a[0] | b[0], a[1] | b[1], a[2] | b[2], a[3] | b[3]}
}
func (a U256) Xor(b U256) U256 {
	return U256{a[0] ^ b[0], a[1] ^ b[1], a[2] ^ b[2], a[3] ^ b[3]}
}
func (a U256) Not() U256 { return U256{^a[0], ^a[1], ^a[2], ^a[3]} }

// Shl shifts left; n >= 256 yields zero.
func (a U256) Shl(n uint) U256 {
	if n >= 256 {
		return U256{}
	}
	limb, bit := n/64, n%64
	var out U256
	for i := 3; i >= int(limb); i-- {
		out[i] = a[i-int(limb)] << bit
		if bit != 0 && i-int(limb)-1 >= 0 {
			out[i] |= a[i-int(limb)-1] >> (64 - bit)
		}
	}
	return out
}

// ShrU shifts right logically; n >= 256 yields zero.
func (a U256) ShrU(n uint) U256 {
	if n >= 256 {
		return U256{}
	}
	limb, bit := n/64, n%64
	var out U256
	for i := 0; i+int(limb) < 4; i++ {
		out[i] = a[i+int(limb)] >> bit
		if bit != 0 && i+int(limb)+1 < 4 {
			out[i] |= a[i+int(limb)+1] << (64 - bit)
		}
	}
	return out
}

// ShrS shifts right arithmetically; n >= 256 fills with the sign.
func (a U256) ShrS(n uint) U256 {
	if !a.Sign() {
		return a.ShrU(n)
	}
	if n >= 256 {
		return U256{^uint64(0), ^uint64(0), ^uint64(0), ^uint64(0)}
	}
	// (a >> n) | (sign-fill << (256 - n))
	out := a.ShrU(n)
	fill := U256{^uint64(0), ^uint64(0), ^uint64(0), ^uint64(0)}.Shl(256 - n)
	return out.Or(fill)
}

// DivU and RemU are unsigned division; division by zero yields zero.
func (a U256) DivU(b U256) U256 { q, _ := a.divRemU(b); return q }
func (a U256) RemU(b U256) U256 { _, r := a.divRemU(b); return r }

func (a U256) divRemU(b U256) (U256, U256) {
	if b.IsZero() {
		return U256{}, U256{}
	}
	if b[1]|b[2]|b[3] == 0 && a[1]|a[2]|a[3] == 0 {
		return U256{a[0] / b[0]}, U256{a[0] % b[0]}
	}
	q, r := new(big.Int).QuoRem(a.big(), b.big(), new(big.Int))
	return u256FromBig(q), u256FromBig(r)
}

// DivS and RemS are signed (truncated); by-zero yields zero, and the
// remainder's sign follows the dividend.
func (a U256) DivS(b U256) U256 {
	if b.IsZero() {
		return U256{}
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

func (a U256) RemS(b U256) U256 {
	if b.IsZero() {
		return U256{}
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

// big conversions go through big-endian bytes so they stay correct on
// hosts where big.Word is 32 bits (386, arm, wasm).
func (a U256) big() *big.Int {
	var b [32]byte
	for i, w := range a {
		binary.BigEndian.PutUint64(b[(3-i)*8:], w)
	}
	return new(big.Int).SetBytes(b[:])
}

func u256FromBig(x *big.Int) U256 {
	var b [32]byte
	x.FillBytes(b[:])
	var u U256
	for i := range u {
		u[i] = binary.BigEndian.Uint64(b[(3-i)*8:])
	}
	return u
}
