package wasman_test

import (
	"bytes"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/tollstation"
	"github.com/c0mm4nd/wasman/wat"
)

// runTollBoth runs entry(arg) under a fresh TollStation on the interpreter
// (WASMAN_JIT_TIER forces it off) and the baseline JIT, returning (gas, trapped).
func runTollBoth(t *testing.T, src, entry string, max uint64, arg []uint64) {
	t.Helper()
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	run := func(jit bool) (uint64, bool) {
		cfg := config.ModuleConfig{
			EnableJIT:   jit,
			TollStation: tollstation.NewSimpleTollStation(max),
		}
		mod, err := wasman.NewModule(cfg, bytes.NewReader(bin))
		if err != nil {
			t.Fatalf("load jit=%v: %v", jit, err)
		}
		ins, err := wasman.NewInstance(mod, nil)
		if err != nil {
			t.Fatalf("inst jit=%v: %v", jit, err)
		}
		_, _, callErr := ins.CallExportedFunc(entry, arg...)
		return cfg.TollStation.GetToll(), callErr != nil
	}
	ig, it := run(false)
	jg, jt := run(true)
	if ig != jg || it != jt {
		t.Errorf("%s(max=%d): interp gas=%d trap=%v, jit gas=%d trap=%v", entry, max, ig, it, jg, jt)
	}
}

func TestTollJITConsistency(t *testing.T) {
	mods := []struct {
		name, src, entry string
		arg              []uint64
	}{
		{"straight", `(module (func (export "e")(result i32)
			(i32.add (i32.const 1)(i32.const 2))))`, "e", nil},
		{"loop", `(module (func (export "e")(param i32)(result i64)(local i64)
			(block (loop (br_if 1 (i32.eqz (local.get 0)))
				(local.set 1 (i64.add (local.get 1)(i64.extend_i32_u (local.get 0))))
				(local.set 0 (i32.sub (local.get 0)(i32.const 1)))(br 0)))
			(local.get 1)))`, "e", []uint64{50}},
		{"return-mid", `(module (func (export "e")(param i32)(result i32)
			(if (local.get 0)(then (i32.const 9)(return)))(i32.const 3)))`, "e", []uint64{1}},
		{"if-else", `(module (func (export "e")(param i32)(result i32)
			(if (result i32)(local.get 0)(then (i32.const 7))(else (i32.const 8)))))`, "e", []uint64{0}},
		{"br_table", `(module (func (export "e")(param i32)(result i32)
			(block (block (block (br_table 0 1 2 (local.get 0)))
			(return (i32.const 10)))(return (i32.const 20)))(i32.const 30)))`, "e", []uint64{1}},
		{"recursion", `(module (func $f (export "e")(param i32)(result i32)
			(if (result i32)(i32.lt_s (local.get 0)(i32.const 2))(then (local.get 0))
			(else (i32.add (call $f (i32.sub (local.get 0)(i32.const 1)))
			(call $f (i32.sub (local.get 0)(i32.const 2))))))))`, "e", []uint64{12}},
		{"unreachable", `(module (func (export "e")(unreachable)))`, "e", nil},
		{"memfill", `(module (memory 1)(func (export "e")(result i32)
			(memory.fill (i32.const 0)(i32.const 7)(i32.const 32))(i32.load8_u (i32.const 5))))`, "e", nil},
	}
	for _, m := range mods {
		// several caps: generous (no trap) and tight (trap mid-execution)
		for _, max := range []uint64{1 << 30, 20, 8, 3, 1} {
			runTollBoth(t, m.src, m.entry, max, m.arg)
		}
	}
}

// BenchmarkTollMetered compares the metered interpreter with the
// inline-metered baseline JIT on an arithmetic loop.
func BenchmarkTollMetered(b *testing.B) {
	src := `(module (func (export "sum")(param i32)(result i64)(local i64)
		(block (loop (br_if 1 (i32.eqz (local.get 0)))
			(local.set 1 (i64.add (local.get 1)(i64.extend_i32_u (local.get 0))))
			(local.set 0 (i32.sub (local.get 0)(i32.const 1)))(br 0)))
		(local.get 1)))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		b.Fatal(err)
	}
	for _, jit := range []struct {
		name string
		on   bool
	}{{"interp", false}, {"jit", true}} {
		b.Run(jit.name, func(b *testing.B) {
			mod, _ := wasman.NewModule(config.ModuleConfig{
				EnableJIT: jit.on, TollStation: tollstation.NewSimpleTollStation(1 << 62),
			}, bytes.NewReader(bin))
			ins, _ := wasman.NewInstance(mod, nil)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, _, err := ins.CallExportedFunc("sum", 10000); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
