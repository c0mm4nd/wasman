package wasman_test

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/wat"
	"github.com/c0mm4nd/wasman/wideint"
)

const wideOpsWat = `(module
  (import "u256" "mul_div" (func $md (param i32 i32 i32 i32)))
  (import "u256" "isqrt"   (func $sq (param i32 i32)))
  (memory (export "mem") 1)
  (func (export "muldiv") (param i32 i32 i32 i32)
    (call $md (local.get 0) (local.get 1) (local.get 2) (local.get 3)))
  (func (export "isqrt") (param i32 i32)
    (call $sq (local.get 0) (local.get 1))))`

func u256bytes(x *big.Int) []byte {
	var b [32]byte
	x.FillBytes(b[:])
	// FillBytes is big-endian; wideint is little-endian
	for i, j := 0, 31; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return b[:]
}

// TestWideMulDivSqrt drives the two DeFi primitives through wasm on both
// execution paths and checks them against the wideint reference (itself
// fuzzed against math/big), plus the c==0 and overflow boundaries.
func TestWideMulDivSqrt(t *testing.T) {
	bin, err := wat.Compile([]byte(wideOpsWat))
	if err != nil {
		t.Fatal(err)
	}
	big2 := func(s string) *big.Int { v, _ := new(big.Int).SetString(s, 10); return v }
	cases := []struct {
		a, b, c string
		trap    bool // mul_div overflow
	}{
		{"1000000", "2000000", "3", false},
		{"340282366920938463463374607431768211456", "340282366920938463463374607431768211456", "2", false},  // 2^128 * 2^128 / 2 = 2^255, exact 512-bit product
		{"115792089237316195423570985008687907853269984665640564039457584007913129639935", "5", "7", false}, // (2^256-1)*5/7
		{"1", "2", "0", false}, // c==0 -> 0, no trap
		{"115792089237316195423570985008687907853269984665640564039457584007913129639935", "2", "1", true}, // (2^256-1)*2/1 >= 2^256 -> overflow trap
	}
	for _, jit := range []bool{false, true} {
		mod, err := wasman.NewModule(config.ModuleConfig{EnableWideInt: true, EnableJIT: jit}, bytes.NewReader(bin))
		if err != nil {
			t.Fatalf("jit=%v load: %v", jit, err)
		}
		ins, err := wasman.NewInstance(mod, nil)
		if err != nil {
			t.Fatalf("jit=%v inst: %v", jit, err)
		}
		mem := ins.Memory.Value
		for _, tc := range cases {
			A, B, C := big2(tc.a), big2(tc.b), big2(tc.c)
			copy(mem[32:64], u256bytes(A))
			copy(mem[64:96], u256bytes(B))
			copy(mem[96:128], u256bytes(C))
			_, _, err := ins.CallExportedFunc("muldiv", 0, 32, 64, 96)
			if tc.trap {
				if err == nil {
					t.Errorf("jit=%v muldiv(%s,%s,%s): expected overflow trap", jit, tc.a, tc.b, tc.c)
				}
				continue
			}
			if err != nil {
				t.Fatalf("jit=%v muldiv: %v", jit, err)
			}
			want, _ := wideint.U256FromBytes(u256bytes(A)).MulDiv(
				wideint.U256FromBytes(u256bytes(B)), wideint.U256FromBytes(u256bytes(C)))
			var wb [32]byte
			want.PutBytes(wb[:])
			if !bytes.Equal(mem[0:32], wb[:]) {
				t.Errorf("jit=%v muldiv(%s,%s,%s) mismatch", jit, tc.a, tc.b, tc.c)
			}
		}
		// isqrt
		for _, s := range []string{"0", "1", "2", "1000000", "115792089237316195423570985008687907853269984665640564039457584007913129639935"} {
			A := big2(s)
			copy(mem[32:64], u256bytes(A))
			if _, _, err := ins.CallExportedFunc("isqrt", 0, 32); err != nil {
				t.Fatalf("jit=%v isqrt: %v", jit, err)
			}
			var wb [32]byte
			wideint.U256FromBytes(u256bytes(A)).Sqrt().PutBytes(wb[:])
			if !bytes.Equal(mem[0:32], wb[:]) {
				t.Errorf("jit=%v isqrt(%s) mismatch", jit, s)
			}
			// sanity: floor(sqrt)^2 <= a < (floor+1)^2
			got := new(big.Int).SetBytes(reverseBytes(mem[0:32]))
			sq := new(big.Int).Mul(got, got)
			if sq.Cmp(A) > 0 {
				t.Errorf("isqrt(%s) too large", s)
			}
		}
	}
}

func reverseBytes(b []byte) []byte {
	out := make([]byte, len(b))
	for i := range b {
		out[len(b)-1-i] = b[i]
	}
	return out
}
