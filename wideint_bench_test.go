package wasman_test

import (
	"bytes"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/tollstation"
	"github.com/c0mm4nd/wasman/wat"
)

const wideBenchWat = `(module
  (import "u256" "mul_div" (func $md (param i32 i32 i32 i32)))
  (import "u256" "isqrt"   (func $sq (param i32 i32)))
  (import "u256" "div_u"   (func $dv (param i32 i32 i32)))
  (import "u256" "add"     (func $ad (param i32 i32 i32)))
  (memory (export "mem") 1)
  (func (export "muldiv") (param $n i32) (block (loop
    (br_if 1 (i32.eqz (local.get $n)))
    (call $md (i32.const 0) (i32.const 32) (i32.const 64) (i32.const 96))
    (local.set $n (i32.sub (local.get $n) (i32.const 1))) (br 0))))
  (func (export "isqrt") (param $n i32) (block (loop
    (br_if 1 (i32.eqz (local.get $n)))
    (call $sq (i32.const 0) (i32.const 32))
    (local.set $n (i32.sub (local.get $n) (i32.const 1))) (br 0))))
  (func (export "divu") (param $n i32) (block (loop
    (br_if 1 (i32.eqz (local.get $n)))
    (call $dv (i32.const 0) (i32.const 32) (i32.const 96))
    (local.set $n (i32.sub (local.get $n) (i32.const 1))) (br 0))))
  (func (export "add") (param $n i32) (block (loop
    (br_if 1 (i32.eqz (local.get $n)))
    (call $ad (i32.const 0) (i32.const 32) (i32.const 64))
    (local.set $n (i32.sub (local.get $n) (i32.const 1))) (br 0)))))`

func benchWideOp(b *testing.B, entry string, toll bool) {
	bin, err := wat.Compile([]byte(wideBenchWat))
	if err != nil {
		b.Fatal(err)
	}
	cfg := config.ModuleConfig{EnableWideInt: true}
	if toll {
		cfg.TollStation = tollstation.NewSimpleTollStation(1 << 62)
	}
	mod, _ := wasman.NewModule(cfg, bytes.NewReader(bin))
	ins, _ := wasman.NewInstance(mod, nil)
	mem := ins.Memory.Value
	// nonzero operands so division/sqrt do real work
	for i := 32; i < 128; i++ {
		mem[i] = byte(i*7 + 1)
	}
	mem[96], mem[97] = 0xd, 0x3 // small-ish nonzero divisor
	b.ResetTimer()
	if _, _, err := ins.CallExportedFunc(entry, uint64(b.N)); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkE2EAdd(b *testing.B)    { benchWideOp(b, "add", false) }
func BenchmarkE2EDivU(b *testing.B)   { benchWideOp(b, "divu", false) }
func BenchmarkE2EMulDiv(b *testing.B) { benchWideOp(b, "muldiv", false) }
func BenchmarkE2EIsqrt(b *testing.B)  { benchWideOp(b, "isqrt", false) }

// a metered embedder's actual path: a TollStation disables the JIT and the fast
// dispatch, so this is the metered interpreter cost.
func BenchmarkE2EMulDivToll(b *testing.B) { benchWideOp(b, "muldiv", true) }
func BenchmarkE2EIsqrtToll(b *testing.B)  { benchWideOp(b, "isqrt", true) }
