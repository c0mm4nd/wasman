package wideint

import (
	"math/big"
	"math/rand"
	"testing"
)

var two128 = new(big.Int).Lsh(big.NewInt(1), 128)
var two256 = new(big.Int).Lsh(big.NewInt(1), 256)

func ref128(x *big.Int) *big.Int { return new(big.Int).Mod(x, two128) }
func ref256(x *big.Int) *big.Int { return new(big.Int).Mod(x, two256) }

// signedRef interprets an unsigned big value as two's complement.
func signedRef(x, wrap *big.Int) *big.Int {
	half := new(big.Int).Rsh(wrap, 1)
	if x.Cmp(half) >= 0 {
		return new(big.Int).Sub(x, wrap)
	}
	return new(big.Int).Set(x)
}

func rnd128(r *rand.Rand) U128 { return U128{r.Uint64(), r.Uint64()} }
func rnd256(r *rand.Rand) U256 { return U256{r.Uint64(), r.Uint64(), r.Uint64(), r.Uint64()} }

var edges64 = []uint64{0, 1, 2, 0x7fffffffffffffff, 0x8000000000000000, ^uint64(0)}

func TestU128AgainstBig(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	var cases []U128
	for _, lo := range edges64 {
		for _, hi := range edges64 {
			cases = append(cases, U128{lo, hi})
		}
	}
	for i := 0; i < 200; i++ {
		cases = append(cases, rnd128(r))
	}
	for _, a := range cases {
		for _, b := range cases[:16] {
			ab, bb := a.big(), b.big()
			check := func(name string, got U128, want *big.Int) {
				if got.big().Cmp(ref128(want)) != 0 {
					t.Fatalf("%s(%v,%v): got %v want %v", name, ab, bb, got.big(), ref128(want))
				}
			}
			check("add", a.Add(b), new(big.Int).Add(ab, bb))
			check("sub", a.Sub(b), new(big.Int).Sub(ab, bb))
			check("mul", a.Mul(b), new(big.Int).Mul(ab, bb))
			if !b.IsZero() {
				check("divu", a.DivU(b), new(big.Int).Quo(ab, bb))
				check("remu", a.RemU(b), new(big.Int).Rem(ab, bb))
				sa, sb := signedRef(ab, two128), signedRef(bb, two128)
				check("divs", a.DivS(b), new(big.Int).Quo(sa, sb))
				check("rems", a.RemS(b), new(big.Int).Rem(sa, sb))
			} else if !a.DivU(b).IsZero() || !a.DivS(b).IsZero() {
				t.Fatal("division by zero must be zero")
			}
			if got, want := a.CmpU(b), ab.Cmp(bb); got != want {
				t.Fatalf("cmpu(%v,%v)=%d want %d", ab, bb, got, want)
			}
			sa, sb := signedRef(ab, two128), signedRef(bb, two128)
			if got, want := a.CmpS(b), sa.Cmp(sb); got != want {
				t.Fatalf("cmps=%d want %d", got, want)
			}
		}
		for _, n := range []uint{0, 1, 63, 64, 65, 127, 128, 300} {
			ab := a.big()
			check := func(name string, got U128, want *big.Int) {
				if got.big().Cmp(ref128(want)) != 0 {
					t.Fatalf("%s(%v,%d): got %v want %v", name, ab, n, got.big(), ref128(want))
				}
			}
			check("shl", a.Shl(n), new(big.Int).Lsh(ab, n))
			check("shru", a.ShrU(n), new(big.Int).Rsh(ab, n))
			sa := signedRef(ab, two128)
			check("shrs", a.ShrS(n), new(big.Int).Rsh(sa, n))
		}
	}
	// MinInt128 / -1 wraps back to MinInt128 (EVM semantics)
	min := U128{0, 0x8000000000000000}
	neg1 := U128{^uint64(0), ^uint64(0)}
	if min.DivS(neg1) != min {
		t.Fatal("MinInt128 / -1 must wrap")
	}
}

func TestU256AgainstBig(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	var cases []U256
	for _, w := range edges64 {
		cases = append(cases, U256{w, 0, 0, 0}, U256{0, w, 0, 0}, U256{0, 0, 0, w},
			U256{w, w, w, w})
	}
	for i := 0; i < 150; i++ {
		cases = append(cases, rnd256(r))
	}
	for _, a := range cases {
		for _, b := range cases[:12] {
			ab, bb := a.big(), b.big()
			check := func(name string, got U256, want *big.Int) {
				if got.big().Cmp(ref256(want)) != 0 {
					t.Fatalf("%s(%v,%v): got %v want %v", name, ab, bb, got.big(), ref256(want))
				}
			}
			check("add", a.Add(b), new(big.Int).Add(ab, bb))
			check("sub", a.Sub(b), new(big.Int).Sub(ab, bb))
			check("mul", a.Mul(b), new(big.Int).Mul(ab, bb))
			if !b.IsZero() {
				check("divu", a.DivU(b), new(big.Int).Quo(ab, bb))
				check("remu", a.RemU(b), new(big.Int).Rem(ab, bb))
				sa, sb := signedRef(ab, two256), signedRef(bb, two256)
				check("divs", a.DivS(b), new(big.Int).Quo(sa, sb))
				check("rems", a.RemS(b), new(big.Int).Rem(sa, sb))
			}
			if got, want := a.CmpU(b), ab.Cmp(bb); got != want {
				t.Fatalf("cmpu=%d want %d", got, want)
			}
			sa, sb := signedRef(ab, two256), signedRef(bb, two256)
			if got, want := a.CmpS(b), sa.Cmp(sb); got != want {
				t.Fatalf("cmps=%d want %d", got, want)
			}
		}
		for _, n := range []uint{0, 1, 63, 64, 100, 191, 192, 255, 256, 400} {
			ab := a.big()
			check := func(name string, got U256, want *big.Int) {
				if got.big().Cmp(ref256(want)) != 0 {
					t.Fatalf("%s(%v,%d): got %v want %v", name, ab, n, got.big(), ref256(want))
				}
			}
			check("shl", a.Shl(n), new(big.Int).Lsh(ab, n))
			check("shru", a.ShrU(n), new(big.Int).Rsh(ab, n))
			sa := signedRef(ab, two256)
			check("shrs", a.ShrS(n), new(big.Int).Rsh(sa, n))
		}
	}
	min := U256{0, 0, 0, 0x8000000000000000}
	neg1 := U256{^uint64(0), ^uint64(0), ^uint64(0), ^uint64(0)}
	if min.DivS(neg1) != min {
		t.Fatal("MinInt256 / -1 must wrap")
	}
	// round-trip bytes
	buf := make([]byte, 32)
	for i := 0; i < 20; i++ {
		v := rnd256(r)
		v.PutBytes(buf)
		if U256FromBytes(buf) != v {
			t.Fatal("byte round-trip")
		}
	}
}
