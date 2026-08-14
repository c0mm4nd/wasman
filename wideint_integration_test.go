package wasman_test

import (
	"bytes"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/wideint"
)

func wideInstance(t *testing.T, enable, jit bool) (*wasman.Instance, error) {
	t.Helper()
	raw, err := os.ReadFile("testdata/wideint.wasm")
	if err != nil {
		t.Fatal(err)
	}
	mod, err := wasman.NewModule(config.ModuleConfig{
		EnableWideInt: enable, EnableJIT: jit,
	}, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return wasman.NewInstance(mod, nil)
}

// TestWideIntDisabledByDefault: without the opt-in, the imports must not
// resolve.
func TestWideIntDisabledByDefault(t *testing.T) {
	if _, err := wideInstance(t, false, false); err == nil {
		t.Fatal("wide-int imports resolved without EnableWideInt")
	}
}

// TestWideIntOps drives every exported operation through wasm and checks
// it against the wideint package (itself verified against math/big).
func TestWideIntOps(t *testing.T) {
	for _, jit := range []bool{false, true} {
		ins, err := wideInstance(t, true, jit)
		if err != nil {
			t.Fatal(err)
		}
		mem := ins.Memory.Value
		r := rand.New(rand.NewSource(7))

		edge := []uint64{0, 1, ^uint64(0), 0x8000000000000000}
		vals128 := []wideint.U128{{}, {Lo: 1}, {Lo: ^uint64(0), Hi: ^uint64(0)},
			{Hi: 0x8000000000000000}}
		vals256 := []wideint.U256{{}, {1}, {^uint64(0), ^uint64(0), ^uint64(0), ^uint64(0)},
			{0, 0, 0, 0x8000000000000000}}
		for i := 0; i < 24; i++ {
			vals128 = append(vals128, wideint.U128{Lo: r.Uint64(), Hi: edge[i%4] ^ r.Uint64()})
			vals256 = append(vals256, wideint.U256{r.Uint64(), r.Uint64(), r.Uint64(), r.Uint64()})
		}

		call := func(fn string, args ...uint64) uint64 {
			rets, _, err := ins.CallExportedFunc(fn, args...)
			if err != nil {
				t.Fatalf("%s: %v", fn, err)
			}
			if len(rets) == 0 {
				return 0
			}
			return rets[0]
		}

		bin128 := map[string]func(a, b wideint.U128) wideint.U128{
			"add": wideint.U128.Add, "sub": wideint.U128.Sub, "mul": wideint.U128.Mul,
			"div_u": wideint.U128.DivU, "rem_u": wideint.U128.RemU,
			"div_s": wideint.U128.DivS, "rem_s": wideint.U128.RemS,
			"and": wideint.U128.And, "or": wideint.U128.Or, "xor": wideint.U128.Xor,
		}
		for _, a := range vals128 {
			for _, b := range vals128[:8] {
				a.PutBytes(mem[16:32])
				b.PutBytes(mem[32:48])
				for name, ref := range bin128 {
					call("u128_"+name, 0, 16, 32)
					if got := wideint.U128FromBytes(mem[0:16]); got != ref(a, b) {
						t.Fatalf("u128 %s(%v,%v) jit=%v: got %v want %v", name, a, b, jit, got, ref(a, b))
					}
				}
				if got := int32(call("u128_cmp_u", 16, 32)); int(got) != a.CmpU(b) {
					t.Fatalf("u128 cmp_u: %d", got)
				}
				if got := int32(call("u128_cmp_s", 16, 32)); int(got) != a.CmpS(b) {
					t.Fatalf("u128 cmp_s: %d", got)
				}
			}
			for _, n := range []uint64{0, 1, 64, 127, 128, 999} {
				a.PutBytes(mem[16:32])
				call("u128_shl", 0, 16, n)
				if got := wideint.U128FromBytes(mem[0:16]); got != a.Shl(uint(n)) {
					t.Fatalf("u128 shl %d", n)
				}
				call("u128_shr_u", 0, 16, n)
				if got := wideint.U128FromBytes(mem[0:16]); got != a.ShrU(uint(n)) {
					t.Fatalf("u128 shr_u %d", n)
				}
				call("u128_shr_s", 0, 16, n)
				if got := wideint.U128FromBytes(mem[0:16]); got != a.ShrS(uint(n)) {
					t.Fatalf("u128 shr_s %d", n)
				}
			}
			a.PutBytes(mem[16:32])
			call("u128_not", 0, 16)
			if got := wideint.U128FromBytes(mem[0:16]); got != a.Not() {
				t.Fatal("u128 not")
			}
			want := uint64(0)
			if a.IsZero() {
				want = 1
			}
			if got := call("u128_iszero", 16); got != want {
				t.Fatal("u128 iszero")
			}
		}

		bin256 := map[string]func(a, b wideint.U256) wideint.U256{
			"add": wideint.U256.Add, "sub": wideint.U256.Sub, "mul": wideint.U256.Mul,
			"div_u": wideint.U256.DivU, "rem_u": wideint.U256.RemU,
			"div_s": wideint.U256.DivS, "rem_s": wideint.U256.RemS,
			"and": wideint.U256.And, "or": wideint.U256.Or, "xor": wideint.U256.Xor,
		}
		for _, a := range vals256 {
			for _, b := range vals256[:8] {
				a.PutBytes(mem[64:96])
				b.PutBytes(mem[96:128])
				for name, ref := range bin256 {
					call("u256_"+name, 0, 64, 96)
					if got := wideint.U256FromBytes(mem[0:32]); got != ref(a, b) {
						t.Fatalf("u256 %s jit=%v: got %v want %v", name, jit, got, ref(a, b))
					}
				}
				if got := int32(call("u256_cmp_s", 64, 96)); int(got) != a.CmpS(b) {
					t.Fatalf("u256 cmp_s: %d", got)
				}
			}
			for _, n := range []uint64{0, 1, 64, 255, 256, 999} {
				a.PutBytes(mem[64:96])
				call("u256_shl", 0, 64, n)
				if got := wideint.U256FromBytes(mem[0:32]); got != a.Shl(uint(n)) {
					t.Fatalf("u256 shl %d", n)
				}
				call("u256_shr_s", 0, 64, n)
				if got := wideint.U256FromBytes(mem[0:32]); got != a.ShrS(uint(n)) {
					t.Fatalf("u256 shr_s %d", n)
				}
			}
		}

		// out-of-bounds pointers trap and leave the instance usable
		if _, _, err := ins.CallExportedFunc("u256_add", 0, 65535, 0); err == nil ||
			!strings.Contains(err.Error(), "out of bounds") {
			t.Fatalf("oob: %v", err)
		}
		if _, _, err := ins.CallExportedFunc("u128_iszero", 0); err != nil {
			t.Fatal("instance unusable after trap")
		}
	}
}

func benchWide(b *testing.B, fn string) {
	raw, _ := os.ReadFile("testdata/wideint.wasm")
	mod, err := wasman.NewModule(config.ModuleConfig{EnableWideInt: true, EnableJIT: true}, bytes.NewReader(raw))
	if err != nil {
		b.Fatal(err)
	}
	ins, err := wasman.NewInstance(mod, nil)
	if err != nil {
		b.Fatal(err)
	}
	// nonzero operands so division does real work
	for i := 64; i < 128; i++ {
		ins.Memory.Value[i] = byte(i*37 + 1)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := ins.CallExportedFunc(fn, 1_000_000); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWideMul256(b *testing.B) { benchWide(b, "bench_mul256") }
func BenchmarkWideDiv256(b *testing.B) { benchWide(b, "bench_div256") }

// BenchmarkWideAdd256 measures one u256 add through wasm; the baseline
// tier keeps the host-call path, the optimizing tier inlines it.
func BenchmarkWideAdd256(b *testing.B) {
	raw, _ := os.ReadFile("testdata/wideint.wasm")
	mod, err := wasman.NewModule(config.ModuleConfig{EnableWideInt: true, EnableJIT: true}, bytes.NewReader(raw))
	if err != nil {
		b.Fatal(err)
	}
	ins, err := wasman.NewInstance(mod, nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := ins.CallExportedFunc("bench_add256", 1_000_000); err != nil {
			b.Fatal(err)
		}
	}
}
