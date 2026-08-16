package wideint

import (
	"math/big"
	"testing"
)

// pad returns a fixed-length little-endian copy of b (truncated or
// zero-extended), so arbitrary fuzz bytes map onto a wide integer.
func bytesN(n int, v byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = v
	}
	return b
}

func pad(b []byte, n int) []byte {
	out := make([]byte, n)
	copy(out, b)
	return out
}

// FuzzU128 checks every U128 operation against math/big on arbitrary
// inputs, including the division edge cases (by-zero, MinInt/-1) that the
// module-level fuzzers cannot reach through the host-import boundary.
func FuzzU128(f *testing.F) {
	f.Add([]byte{1}, []byte{2}, uint(0))
	f.Add([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x80}, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, uint(1))
	f.Fuzz(func(t *testing.T, ab, bb []byte, sh uint) {
		a := U128FromBytes(pad(ab, 16))
		b := U128FromBytes(pad(bb, 16))
		A, B := a.big(), b.big()
		chk := func(name string, got U128, want *big.Int) {
			if got.big().Cmp(ref128(want)) != 0 {
				t.Fatalf("%s(%v,%v): got %v want %v", name, A, B, got.big(), ref128(want))
			}
		}
		chk("add", a.Add(b), new(big.Int).Add(A, B))
		chk("sub", a.Sub(b), new(big.Int).Sub(A, B))
		chk("mul", a.Mul(b), new(big.Int).Mul(A, B))
		chk("and", a.And(b), new(big.Int).And(A, B))
		chk("or", a.Or(b), new(big.Int).Or(A, B))
		chk("xor", a.Xor(b), new(big.Int).Xor(A, B))
		chk("not", a.Not(), new(big.Int).Not(A))
		chk("neg", a.Neg(), new(big.Int).Neg(A))
		chk("shl", a.Shl(sh%512), new(big.Int).Lsh(A, sh%512))
		chk("shru", a.ShrU(sh%512), new(big.Int).Rsh(A, sh%512))
		chk("shrs", a.ShrS(sh%512), new(big.Int).Rsh(signedRef(A, two128), sh%512))
		// division: by-zero yields zero, else matches big
		if b.IsZero() {
			if !a.DivU(b).IsZero() || !a.RemU(b).IsZero() ||
				!a.DivS(b).IsZero() || !a.RemS(b).IsZero() {
				t.Fatal("div/rem by zero must be zero")
			}
		} else {
			chk("divu", a.DivU(b), new(big.Int).Quo(A, B))
			chk("remu", a.RemU(b), new(big.Int).Rem(A, B))
			sa, sb := signedRef(A, two128), signedRef(B, two128)
			chk("divs", a.DivS(b), new(big.Int).Quo(sa, sb))
			chk("rems", a.RemS(b), new(big.Int).Rem(sa, sb))
		}
		if a.CmpU(b) != A.Cmp(B) {
			t.Fatalf("cmpu(%v,%v)", A, B)
		}
		if a.CmpS(b) != signedRef(A, two128).Cmp(signedRef(B, two128)) {
			t.Fatalf("cmps(%v,%v)", A, B)
		}
		// byte round-trip
		var buf [16]byte
		a.PutBytes(buf[:])
		if U128FromBytes(buf[:]) != a {
			t.Fatal("u128 round-trip")
		}
	})
}

// FuzzU256 is the 256-bit counterpart.
func FuzzU256(f *testing.F) {
	f.Add([]byte{3}, []byte{7}, uint(0))
	// divisor with a nonzero third limb exercises the multi-limb big path
	f.Add(bytesN(32, 0xff), append(make([]byte, 17), 1), uint(3))
	f.Fuzz(func(t *testing.T, ab, bb []byte, sh uint) {
		a := U256FromBytes(pad(ab, 32))
		b := U256FromBytes(pad(bb, 32))
		A, B := a.big(), b.big()
		chk := func(name string, got U256, want *big.Int) {
			if got.big().Cmp(ref256(want)) != 0 {
				t.Fatalf("%s(%v,%v): got %v want %v", name, A, B, got.big(), ref256(want))
			}
		}
		chk("add", a.Add(b), new(big.Int).Add(A, B))
		chk("sub", a.Sub(b), new(big.Int).Sub(A, B))
		chk("mul", a.Mul(b), new(big.Int).Mul(A, B))
		chk("and", a.And(b), new(big.Int).And(A, B))
		chk("or", a.Or(b), new(big.Int).Or(A, B))
		chk("xor", a.Xor(b), new(big.Int).Xor(A, B))
		chk("not", a.Not(), new(big.Int).Not(A))
		chk("neg", a.Neg(), new(big.Int).Neg(A))
		chk("shl", a.Shl(sh%1024), new(big.Int).Lsh(A, sh%1024))
		chk("shru", a.ShrU(sh%1024), new(big.Int).Rsh(A, sh%1024))
		chk("shrs", a.ShrS(sh%1024), new(big.Int).Rsh(signedRef(A, two256), sh%1024))
		if b.IsZero() {
			if !a.DivU(b).IsZero() || !a.DivS(b).IsZero() {
				t.Fatal("div by zero must be zero")
			}
		} else {
			chk("divu", a.DivU(b), new(big.Int).Quo(A, B))
			chk("remu", a.RemU(b), new(big.Int).Rem(A, B))
			sa, sb := signedRef(A, two256), signedRef(B, two256)
			chk("divs", a.DivS(b), new(big.Int).Quo(sa, sb))
			chk("rems", a.RemS(b), new(big.Int).Rem(sa, sb))
		}
		if a.CmpU(b) != A.Cmp(B) {
			t.Fatalf("cmpu(%v,%v)", A, B)
		}
		if a.CmpS(b) != signedRef(A, two256).Cmp(signedRef(B, two256)) {
			t.Fatalf("cmps(%v,%v)", A, B)
		}
		var buf [32]byte
		a.PutBytes(buf[:])
		if U256FromBytes(buf[:]) != a {
			t.Fatal("u256 round-trip")
		}
	})
}
