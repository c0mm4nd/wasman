package wideint

// Edge-case tests for the branches that the random/boundary sweeps in
// wideint_test.go rarely reach: division-by-zero conventions, signed
// comparisons across the sign boundary, MulDiv overflow, the rare
// normalization branches of Knuth Algorithm D (qhat saturation and the
// D6 add-back), Sqrt near-perfect squares, and the big.Int conversion
// helpers. Every arithmetic result is cross-checked against math/big.

import (
	"math/big"
	"math/rand"
	"testing"
)

var maxU64 = ^uint64(0)

// limbsToBig builds a big.Int from little-endian 64-bit limbs (works for
// both the 4-limb U256 and the 8-limb dividends of divremFull).
func limbsToBig(limbs []uint64) *big.Int {
	x := new(big.Int)
	for i := len(limbs) - 1; i >= 0; i-- {
		x.Lsh(x, 64)
		x.Or(x, new(big.Int).SetUint64(limbs[i]))
	}
	return x
}

// TestDivRemByZero pins the EVM convention: every division flavor with a
// zero divisor yields zero, never a trap.
func TestDivRemByZero(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	u128s := []U128{
		{}, {1, 0}, {maxU64, maxU64}, {0, 0x8000000000000000}, // 0, 1, -1, MinInt128
		rnd128(r), rnd128(r),
	}
	for _, a := range u128s {
		if !a.DivU(U128{}).IsZero() || !a.RemU(U128{}).IsZero() ||
			!a.DivS(U128{}).IsZero() || !a.RemS(U128{}).IsZero() {
			t.Fatalf("u128 div/rem by zero must be zero (a=%v)", a.big())
		}
	}
	u256s := []U256{
		{}, {1, 0, 0, 0}, {maxU64, maxU64, maxU64, maxU64}, {0, 0, 0, 0x8000000000000000},
		rnd256(r), rnd256(r),
	}
	for _, a := range u256s {
		if !a.DivU(U256{}).IsZero() || !a.RemU(U256{}).IsZero() ||
			!a.DivS(U256{}).IsZero() || !a.RemS(U256{}).IsZero() {
			t.Fatalf("u256 div/rem by zero must be zero (a=%v)", a.big())
		}
	}
	// MinValue / -1 and MinValue % -1 wrap per EVM semantics.
	min128, neg1_128 := U128{0, 0x8000000000000000}, U128{maxU64, maxU64}
	if min128.DivS(neg1_128) != min128 || !min128.RemS(neg1_128).IsZero() {
		t.Fatal("u128 MinValue / -1 must wrap with remainder 0")
	}
	min256 := U256{0, 0, 0, 0x8000000000000000}
	neg1_256 := U256{maxU64, maxU64, maxU64, maxU64}
	if min256.DivS(neg1_256) != min256 || !min256.RemS(neg1_256).IsZero() {
		t.Fatal("u256 MinValue / -1 must wrap with remainder 0")
	}
}

// TestCmpSSignBoundary exercises both mixed-sign returns of CmpS for both
// widths, checked against math/big on the two's-complement interpretation.
func TestCmpSSignBoundary(t *testing.T) {
	r := rand.New(rand.NewSource(4))
	pos256 := []U256{{}, {1, 0, 0, 0}, {maxU64, maxU64, maxU64, 0x7fffffffffffffff}}
	neg256 := []U256{{0, 0, 0, 0x8000000000000000}, {maxU64, maxU64, maxU64, maxU64}}
	for i := 0; i < 8; i++ {
		v := rnd256(r)
		v[3] &^= 1 << 63
		pos256 = append(pos256, v)
		v = rnd256(r)
		v[3] |= 1 << 63
		neg256 = append(neg256, v)
	}
	for _, p := range pos256 {
		for _, n := range neg256 {
			if got := p.CmpS(n); got != 1 {
				t.Fatalf("u256 CmpS(pos=%v, neg=%v)=%d want 1", p.big(), n.big(), got)
			}
			if got := n.CmpS(p); got != -1 {
				t.Fatalf("u256 CmpS(neg=%v, pos=%v)=%d want -1", n.big(), p.big(), got)
			}
			want := signedRef(p.big(), two256).Cmp(signedRef(n.big(), two256))
			if p.CmpS(n) != want {
				t.Fatalf("u256 CmpS disagrees with big for %v vs %v", p.big(), n.big())
			}
		}
	}
	// mirror for U128 (mixed-sign branches there too)
	p128, n128 := U128{5, 0}, U128{maxU64, maxU64}
	if p128.CmpS(n128) != 1 || n128.CmpS(p128) != -1 {
		t.Fatal("u128 CmpS mixed signs")
	}
}

// TestMulDivEdges checks the c == 0 and quotient-overflow paths of both
// MulDiv implementations against the full-width big.Int product.
func TestMulDivEdges(t *testing.T) {
	r := rand.New(rand.NewSource(5))

	// c == 0: result 0, ok false
	if got, ok := (U128{7, 7}).MulDiv(U128{9, 9}, U128{}); !got.IsZero() || ok {
		t.Fatal("u128 MulDiv by zero must be (0,false)")
	}
	if got, ok := (U256{7, 7, 7, 7}).MulDiv(U256{9, 9, 9, 9}, U256{}); !got.IsZero() || ok {
		t.Fatal("u256 MulDiv by zero must be (0,false)")
	}

	// quotient overflow: max*max/1 exceeds the width; the low limbs come
	// back with ok == false
	max128 := U128{maxU64, maxU64}
	one128 := U128{1, 0}
	got128, ok := max128.MulDiv(max128, one128)
	want128 := new(big.Int).And(new(big.Int).Mul(max128.big(), max128.big()), mask128)
	if ok || got128.big().Cmp(want128) != 0 {
		t.Fatalf("u128 MulDiv overflow: got (%v,%v) want (%v,false)", got128.big(), ok, want128)
	}
	max256 := U256{maxU64, maxU64, maxU64, maxU64}
	one256 := U256{1, 0, 0, 0}
	got256, ok := max256.MulDiv(max256, one256)
	want256 := new(big.Int).And(new(big.Int).Mul(max256.big(), max256.big()), mask256)
	if ok || got256.big().Cmp(want256) != 0 {
		t.Fatalf("u256 MulDiv overflow: got (%v,%v) want (%v,false)", got256.big(), ok, want256)
	}

	// random cross-check including exact (in-range) quotients
	for i := 0; i < 500; i++ {
		a, b, c := rnd256(r), rnd256(r), rnd256(r)
		if i%3 == 0 {
			c = U256{r.Uint64() | 1, 0, 0, 0} // single-limb divisor path
		}
		got, ok := a.MulDiv(b, c)
		full := new(big.Int).Quo(new(big.Int).Mul(a.big(), b.big()), c.big())
		wantOK := full.BitLen() <= 256
		want := new(big.Int).And(full, mask256)
		if ok != wantOK || got.big().Cmp(want) != 0 {
			t.Fatalf("u256 MulDiv(%v,%v,%v): got (%v,%v) want (%v,%v)",
				a.big(), b.big(), c.big(), got.big(), ok, want, wantOK)
		}
		a128, b128, c128 := rnd128(r), rnd128(r), rnd128(r)
		got128, ok := a128.MulDiv(b128, c128)
		full = new(big.Int).Quo(new(big.Int).Mul(a128.big(), b128.big()), c128.big())
		wantOK = full.BitLen() <= 128
		want = new(big.Int).And(full, mask128)
		if ok != wantOK || got128.big().Cmp(want) != 0 {
			t.Fatalf("u128 MulDiv(%v,%v,%v): got (%v,%v) want (%v,%v)",
				a128.big(), b128.big(), c128.big(), got128.big(), ok, want, wantOK)
		}
	}
}

// TestKnuthRareBranches drives the two Algorithm D branches that random
// operands essentially never hit: qhat saturating at 2^64-1 (un[j+n] equal
// to the normalized divisor's top limb) and the D6 add-back after an
// over-subtraction. The fixed vectors below were found by exhaustive
// search over near-boundary limbs; each is verified against math/big.
func TestKnuthRareBranches(t *testing.T) {
	type divCase struct {
		name string
		u    [8]uint64 // little-endian 512-bit dividend
		d    U256
	}
	cases := []divCase{
		// 256/256 through the public API: qhat estimate saturates (n=2)
		{"qhat-saturation-256", [8]uint64{0xfffffffffffffffe, 0x956b400d48604236, maxU64, 0}, U256{0xfffffffffffffffd, maxU64, 0, 0}},
		// 256/256 through the public API: D6 add-back (n=3)
		{"add-back-256", [8]uint64{0, maxU64, maxU64, 0}, U256{0x6d7acf370641dfb5, maxU64, maxU64, 0}},
		// 512-bit dividend (the MulDiv path): hits both branches with n=4
		{"qhat-and-add-back-512", [8]uint64{0xfffffffffffffffd, 0xfffffffffffffffd, 0, 0xfd0f14759e4874f1, 0, 0, 0xfffffffffffffffe, 0}, U256{0xfffffffffffffffe, 0, maxU64, 0xfffffffffffffffe}},
		// u < d: zero quotient, remainder u
		{"u-less-than-d", [8]uint64{5, 6, 0, 0, 0, 0, 0, 0}, U256{1, 2, 3, 4}},
		// single-limb divisor long-division path over all 8 limbs
		{"single-limb-divisor", [8]uint64{maxU64, maxU64, maxU64, maxU64, maxU64, maxU64, maxU64, maxU64}, U256{0x8000000000000001, 0, 0, 0}},
	}
	for _, tc := range cases {
		q, rem := divremFull(tc.u, tc.d)
		ub, db := limbsToBig(tc.u[:]), tc.d.big()
		wantQ, wantR := new(big.Int).QuoRem(ub, db, new(big.Int))
		if limbsToBig(q[:]).Cmp(wantQ) != 0 || rem.big().Cmp(wantR) != 0 {
			t.Fatalf("%s: divremFull(%v,%v) = (%v,%v) want (%v,%v)",
				tc.name, ub, db, limbsToBig(q[:]), rem.big(), wantQ, wantR)
		}
		// when the dividend fits in 256 bits the public DivU/RemU must agree
		if tc.u[4]|tc.u[5]|tc.u[6]|tc.u[7] == 0 {
			a := U256{tc.u[0], tc.u[1], tc.u[2], tc.u[3]}
			if a.DivU(tc.d).big().Cmp(wantQ) != 0 || a.RemU(tc.d).big().Cmp(wantR) != 0 {
				t.Fatalf("%s: DivU/RemU disagree with big", tc.name)
			}
		}
	}

	// randomized stress with near-boundary limbs (0, max, max-k), the
	// population where the rare branches live, cross-checked against big
	r := rand.New(rand.NewSource(6))
	limb := func() uint64 {
		switch r.Intn(4) {
		case 0:
			return maxU64
		case 1:
			return 0
		case 2:
			return maxU64 - uint64(r.Intn(3))
		default:
			return r.Uint64()
		}
	}
	for i := 0; i < 20000; i++ {
		var u [8]uint64
		var d U256
		nd := 2 + r.Intn(3)
		for j := 0; j < nd; j++ {
			d[j] = limb()
		}
		if d[nd-1] == 0 {
			d[nd-1] = 1 + r.Uint64()
		}
		mu := nd + r.Intn(8-nd+1)
		for j := 0; j < mu; j++ {
			u[j] = limb()
		}
		q, rem := divremFull(u, d)
		ub, db := limbsToBig(u[:]), d.big()
		wantQ, wantR := new(big.Int).QuoRem(ub, db, new(big.Int))
		if limbsToBig(q[:]).Cmp(wantQ) != 0 || rem.big().Cmp(wantR) != 0 {
			t.Fatalf("divremFull(%v,%v) = (%v,%v) want (%v,%v)",
				ub, db, limbsToBig(q[:]), rem.big(), wantQ, wantR)
		}
	}
}

// TestSqrtEdges covers the small-value early return and the Newton
// convergence around perfect squares for both widths.
func TestSqrtEdges(t *testing.T) {
	if got := (U256{}).Sqrt(); !got.IsZero() {
		t.Fatalf("sqrt(0)=%v want 0", got.big())
	}
	if got := (U256{1, 0, 0, 0}).Sqrt(); got != (U256{1, 0, 0, 0}) {
		t.Fatalf("sqrt(1)=%v want 1", got.big())
	}
	if got := (U128{}).Sqrt(); !got.IsZero() {
		t.Fatal("u128 sqrt(0) must be 0")
	}
	r := rand.New(rand.NewSource(7))
	check := func(a U256) {
		want := new(big.Int).Sqrt(a.big())
		if got := a.Sqrt(); got.big().Cmp(want) != 0 {
			t.Fatalf("sqrt(%v)=%v want %v", a.big(), got.big(), want)
		}
	}
	one := U256{1, 0, 0, 0}
	for i := 0; i < 300; i++ {
		x := U256{r.Uint64(), r.Uint64(), 0, 0} // root fits in 128 bits
		sq := x.Mul(x)
		check(sq)          // exact square
		check(sq.Sub(one)) // one below: floor must round down to x-1
		check(sq.Add(one)) // one above: floor stays x
		check(rnd256(r))
	}
	// max value: no overflow in the Newton initial guess
	check(U256{maxU64, maxU64, maxU64, maxU64})
	// u128 near-perfect squares against big
	for i := 0; i < 300; i++ {
		x := U128{r.Uint64(), 0}
		sq := x.Mul(x)
		for _, v := range []U128{sq, sq.Sub(U128{1, 0}), sq.Add(U128{1, 0})} {
			want := new(big.Int).Sqrt(v.big())
			if got := v.Sqrt(); got.big().Cmp(want) != 0 {
				t.Fatalf("u128 sqrt(%v)=%v want %v", v.big(), got.big(), want)
			}
		}
	}
}

// TestBigConversions round-trips both directions of the big.Int bridge,
// including the otherwise-unreached u256FromBig, and bitLen on zero.
func TestBigConversions(t *testing.T) {
	r := rand.New(rand.NewSource(8))
	values := []U256{
		{}, {1, 0, 0, 0}, {0, 0, 0, 1 << 63}, {maxU64, maxU64, maxU64, maxU64},
	}
	for i := 0; i < 50; i++ {
		values = append(values, rnd256(r))
	}
	for _, v := range values {
		if got := u256FromBig(v.big()); got != v {
			t.Fatalf("u256FromBig(big(%v)) = %v", v.big(), got.big())
		}
		if got, want := v.bitLen(), v.big().BitLen(); got != want {
			t.Fatalf("bitLen(%v)=%d want %d", v.big(), got, want)
		}
	}
	if got := (U256{}).bitLen(); got != 0 {
		t.Fatalf("bitLen(0)=%d want 0", got)
	}
	// u128 mirror
	for i := 0; i < 50; i++ {
		v := rnd128(r)
		if got := u128FromBig(v.big()); got != v {
			t.Fatalf("u128FromBig(big(%v)) = %v", v.big(), got.big())
		}
	}
}
