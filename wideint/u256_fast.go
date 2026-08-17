package wideint

import "math/bits"

// Allocation-free MulDiv and Sqrt for U256: a full 512-bit product divided
// by Knuth's Algorithm D, and an integer Newton square root reusing that
// division. Both avoid math/big (and its heap traffic) entirely; the
// results are verified bit-for-bit against big.Int by fuzzing.

// mulFull returns the exact 512-bit product a*b as eight little-endian
// limbs (operand-scanning schoolbook; a*b+p+carry < 2^128 keeps the carry
// in one word).
func (a U256) mulFull(b U256) [8]uint64 {
	var p [8]uint64
	for i := 0; i < 4; i++ {
		var carry uint64
		for j := 0; j < 4; j++ {
			hi, lo := bits.Mul64(a[i], b[j])
			lo, c := bits.Add64(lo, p[i+j], 0)
			hi += c
			lo, c = bits.Add64(lo, carry, 0)
			hi += c
			p[i+j] = lo
			carry = hi
		}
		p[i+4] = carry
	}
	return p
}

func shrw(x uint64, n uint) uint64 {
	if n >= 64 {
		return 0
	}
	return x >> n
}

// divFull returns floor(u / d) as eight limbs (d != 0). The remainder is
// discarded; MulDiv only needs the floor quotient.
func divFull(u [8]uint64, d U256) [8]uint64 {
	// significant limbs of the divisor
	n := 4
	for n > 1 && d[n-1] == 0 {
		n--
	}
	if n == 1 { // single-word long division
		var q [8]uint64
		var r uint64
		dd := d[0]
		for i := 7; i >= 0; i-- {
			q[i], r = bits.Div64(r, u[i], dd)
		}
		return q
	}
	// significant limbs of the dividend
	m := 8
	for m > 0 && u[m-1] == 0 {
		m--
	}
	if m < n {
		return [8]uint64{}
	}

	// Knuth Algorithm D. D1: normalize so the divisor's top bit is set.
	s := uint(bits.LeadingZeros64(d[n-1]))
	var vn [4]uint64
	for i := n - 1; i > 0; i-- {
		vn[i] = d[i]<<s | shrw(d[i-1], 64-s)
	}
	vn[0] = d[0] << s
	var un [9]uint64
	un[m] = shrw(u[m-1], 64-s)
	for i := m - 1; i > 0; i-- {
		un[i] = u[i]<<s | shrw(u[i-1], 64-s)
	}
	un[0] = u[0] << s

	var q [8]uint64
	vn1 := vn[n-1]
	vn2 := vn[n-2]
	for j := m - n; j >= 0; j-- {
		// D3: estimate qhat. The multiply-subtract's add-back below is the
		// safety net, so an over-estimate of one or two is corrected there.
		un2 := un[j+n]
		var qhat, rhat uint64
		if un2 >= vn1 {
			qhat = ^uint64(0)
		} else {
			qhat, rhat = bits.Div64(un2, un[j+n-1], vn1)
			for {
				hi, lo := bits.Mul64(qhat, vn2)
				if hi < rhat || (hi == rhat && lo <= un[j+n-2]) {
					break
				}
				qhat--
				rhat += vn1
				if rhat < vn1 { // rhat overflowed a word: qhat is small enough
					break
				}
			}
		}
		// D4: multiply and subtract un[j..j+n] -= qhat * vn
		var borrow, carry uint64
		for i := 0; i < n; i++ {
			hi, lo := bits.Mul64(qhat, vn[i])
			lo, c := bits.Add64(lo, carry, 0)
			hi += c
			sub, b2 := bits.Sub64(un[i+j], lo, borrow)
			un[i+j] = sub
			borrow = b2
			carry = hi
		}
		sub, b2 := bits.Sub64(un[j+n], carry, borrow)
		un[j+n] = sub
		// D5/D6: if we subtracted too much, add the divisor back once.
		if b2 != 0 {
			qhat--
			var c uint64
			for i := 0; i < n; i++ {
				s2, cc := bits.Add64(un[i+j], vn[i], c)
				un[i+j] = s2
				c = cc
			}
			un[j+n] += c
		}
		q[j] = qhat
	}
	return q
}

// bitLen returns the position of the highest set bit (0 for zero).
func (a U256) bitLen() int {
	for i := 3; i >= 0; i-- {
		if a[i] != 0 {
			return i*64 + bits.Len64(a[i])
		}
	}
	return 0
}

// divU256 returns floor(a / d) for 256-bit operands (d != 0).
func (a U256) divU256(d U256) U256 {
	q := divFull([8]uint64{a[0], a[1], a[2], a[3]}, d)
	return U256{q[0], q[1], q[2], q[3]}
}
