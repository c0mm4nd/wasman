package wasman_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/tollstation"
	"github.com/c0mm4nd/wasman/wat"
)

// TestConfigMatrix is the guard against config-dependent behaviour bugs
// like the toll-station return regression: every ModuleConfig variant that
// only changes an execution path (not the observable result) must produce
// the SAME outcome — same trap/no-trap, same return values — on a battery
// of modules exercising mid-function returns, divergent branches, loops,
// br_table, recursion, bulk memory, traps, and non-NaN float arithmetic.
//
// A single configuration would have missed the return bug; this crosses
// the interpreter fast path, the metered path, the NaN-canonicalization
// path and both JIT tiers against one oracle.
func TestConfigMatrix(t *testing.T) {
	depth := uint64(1024)

	// non-NaN-producing modules, so NaN canonicalization cannot legitimately
	// change any result; every variant must agree with the default.
	mods := []struct {
		name, src, entry string
		args             []uint64
		want             []uint64
		wantTrap         bool
	}{
		{"return-mid", `(module (func (export "e") (param i32) (result i32)
			(if (local.get 0) (then (i32.const 1) (return))) (i32.const 2)))`,
			"e", []uint64{0}, []uint64{2}, false},
		{"return-in-block-unreachable", `(module (memory 1) (func (export "e") (result i32)
			(block (br_if 0 (i64.ne (i64.const 800) (i64.const 800))) (i32.const 5) (return))
			(unreachable)))`,
			"e", nil, []uint64{5}, false},
		{"loop-sum", `(module (func (export "e") (param i32) (result i64) (local i64)
			(block (loop
				(br_if 1 (i32.eqz (local.get 0)))
				(local.set 1 (i64.add (local.get 1) (i64.extend_i32_u (local.get 0))))
				(local.set 0 (i32.sub (local.get 0) (i32.const 1)))
				(br 0)))
			(local.get 1)))`,
			"e", []uint64{100}, []uint64{5050}, false},
		{"br_table", `(module (func (export "e") (param i32) (result i32)
			(block (block (block
				(br_table 0 1 2 (local.get 0)))
				(return (i32.const 10)))
				(return (i32.const 20)))
			(i32.const 30)))`,
			"e", []uint64{1}, []uint64{20}, false},
		{"recursion", `(module (func $f (export "e") (param i32) (result i32)
			(if (result i32) (i32.lt_s (local.get 0) (i32.const 2))
				(then (local.get 0))
				(else (i32.add (call $f (i32.sub (local.get 0) (i32.const 1)))
					(call $f (i32.sub (local.get 0) (i32.const 2))))))))`,
			"e", []uint64{15}, []uint64{610}, false},
		{"bulk-memory", `(module (memory 1) (data (i32.const 0) "\aa\bb\cc\dd")
			(func (export "e") (result i32)
				(memory.fill (i32.const 64) (i32.const 0x33) (i32.const 16))
				(memory.copy (i32.const 128) (i32.const 0) (i32.const 4))
				(i32.add (i32.load8_u (i32.const 70)) (i32.load8_u (i32.const 130)))))`,
			"e", nil, []uint64{0x33 + 0xcc}, false}, // fill=0x33 at 70, copy data[2]=0xcc at 130
		{"float-nonan", `(module (func (export "e") (result f64)
			(f64.add (f64.mul (f64.const 1.5) (f64.const 2.0)) (f64.const 0.5))))`,
			"e", nil, []uint64{0x400c000000000000}, false}, // 3.5
		{"div-zero-trap", `(module (func (export "e") (result i32)
			(i32.div_u (i32.const 1) (i32.const 0))))`,
			"e", nil, nil, true},
		{"unreachable-trap", `(module (func (export "e") (unreachable)))`,
			"e", nil, nil, true},
		{"oob-trap", `(module (memory 1) (func (export "e") (result i32)
			(i32.load (i32.const 0x7fffffff))))`,
			"e", nil, nil, true},
		{"return-then-loop", `(module (func (export "e") (param i32) (result i32)
			(if (local.get 0) (then (i32.const 7) (return)))
			(loop (br 0)) (unreachable)))`, // divergent loop tail after the return
			"e", []uint64{1}, []uint64{7}, false},
	}

	// each variant only flips an execution path; results must not change.
	variants := []struct {
		name string
		cfg  func() config.ModuleConfig
		// float: whether this variant rejects float modules (DisableFloatPoint)
		rejectsFloat bool
	}{
		{"default", func() config.ModuleConfig { return config.ModuleConfig{} }, false},
		{"recover", func() config.ModuleConfig { return config.ModuleConfig{Recover: true} }, false},
		{"depthlimit", func() config.ModuleConfig { return config.ModuleConfig{CallDepthLimit: &depth} }, false},
		{"maxmem", func() config.ModuleConfig { return config.ModuleConfig{MaxMemoryPages: 64} }, false},
		{"toll", func() config.ModuleConfig {
			return config.ModuleConfig{TollStation: tollstation.NewSimpleTollStation(1 << 40)}
		}, false},
		{"canon", func() config.ModuleConfig { return config.ModuleConfig{CanonicalizeNaNs: true} }, false},
		{"jit", func() config.ModuleConfig { return config.ModuleConfig{EnableJIT: true} }, false},
		{"jit+depth", func() config.ModuleConfig {
			return config.ModuleConfig{EnableJIT: true, CallDepthLimit: &depth}
		}, false},
		{"toll+canon+depth", func() config.ModuleConfig {
			return config.ModuleConfig{
				TollStation: tollstation.NewSimpleTollStation(1 << 40), CanonicalizeNaNs: true, CallDepthLimit: &depth,
			}
		}, false},
		{"skipvalidation", func() config.ModuleConfig { return config.ModuleConfig{SkipValidation: true} }, false},
		{"wideint", func() config.ModuleConfig { return config.ModuleConfig{EnableWideInt: true} }, false},
		{"all", func() config.ModuleConfig {
			return config.ModuleConfig{
				Recover: true, CallDepthLimit: &depth, MaxMemoryPages: 64,
				TollStation: tollstation.NewSimpleTollStation(1 << 40), CanonicalizeNaNs: true, EnableWideInt: true,
			}
		}, false},
		{"nofloat", func() config.ModuleConfig { return config.ModuleConfig{DisableFloatPoint: true} }, true},
	}

	for _, m := range mods {
		bin, err := wat.Compile([]byte(m.src))
		if err != nil {
			t.Fatalf("%s compile: %v", m.name, err)
		}
		isFloat := m.name == "float-nonan"
		for _, v := range variants {
			trapped, got, loadErr := runVariant(bin, v.cfg(), m.entry, m.args)
			if v.rejectsFloat && isFloat {
				if loadErr == nil {
					t.Errorf("%s/%s: DisableFloatPoint accepted a float module", m.name, v.name)
				}
				continue
			}
			if loadErr != nil {
				t.Errorf("%s/%s: unexpected load error: %v", m.name, v.name, loadErr)
				continue
			}
			if trapped != m.wantTrap {
				t.Errorf("%s/%s: trapped=%v, want %v", m.name, v.name, trapped, m.wantTrap)
				continue
			}
			if !m.wantTrap && !equalU64(got, m.want) {
				t.Errorf("%s/%s: got %v, want %v", m.name, v.name, got, m.want)
			}
		}
	}
}

func runVariant(bin []byte, cfg config.ModuleConfig, entry string, args []uint64) (trapped bool, got []uint64, loadErr error) {
	mod, err := wasman.NewModule(cfg, bytes.NewReader(bin))
	if err != nil {
		return false, nil, err
	}
	ins, err := wasman.NewInstance(mod, nil)
	if err != nil {
		return false, nil, err
	}
	rets, _, callErr := ins.CallExportedFunc(entry, args...)
	if callErr != nil {
		return true, nil, nil
	}
	return false, rets, nil
}

func equalU64(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestDisableFloatPointOfficial verifies the flag against binaries produced
// by the reference wat2wasm: a float-heavy contract (mandel) is rejected,
// an integer one (sum) is accepted. This is the guarantee ngcore relies on
// to keep consensus execution deterministic.
func TestDisableFloatPointOfficial(t *testing.T) {
	cases := []struct {
		path       string
		wantReject bool
	}{
		{"bench/testdata/mandel.wasm", true}, // f64 escape-time: must reject
		{"bench/testdata/sum.wasm", false},   // i64 loop: must accept
		{"bench/testdata/hash.wasm", false},  // i64 mixing: must accept
		{"bench/testdata/memrw.wasm", false}, // i32 memory: must accept
	}
	for _, c := range cases {
		bin, err := os.ReadFile(c.path)
		if err != nil {
			t.Skipf("fixture %s missing", c.path)
		}
		_, err = wasman.NewModule(config.ModuleConfig{DisableFloatPoint: true}, bytes.NewReader(bin))
		if c.wantReject && err == nil {
			t.Errorf("%s: DisableFloatPoint accepted a float contract", c.path)
		}
		if !c.wantReject && err != nil {
			t.Errorf("%s: DisableFloatPoint wrongly rejected an integer contract: %v", c.path, err)
		}
	}
}
