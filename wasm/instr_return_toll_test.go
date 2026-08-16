package wasm_test

import (
	"bytes"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/tollstation"
	"github.com/c0mm4nd/wasman/wat"
)

// TestReturnUnderTollStation guards a control-flow bug where the toll
// charge clobbered the internal return sentinel: a `return` in the middle
// of a function (anything following it in the enclosing block) was dropped
// once a TollStation was configured, so execution fell through to the tail
// instead of returning. Reported downstream — every contract with a
// runtime assertion (require/assert compiles to `if cond { <divergent> }`)
// misfired. Covers the repro plus the general mid-function return.
func TestReturnUnderTollStation(t *testing.T) {
	cases := []struct {
		name, src string
	}{
		{"return-then-unreachable", `
(module (memory 1)
  (func (export "main")
    (block
      (br_if 0 (i64.ne (i64.const 800) (i64.const 800)))
      (return))
    (unreachable)))`},
		{"if-divergent-not-taken", `
(module
  (func (export "main") (param i32) (result i32)
    (block
      (br_if 0 (i32.eqz (local.get 0)))
      (unreachable))
    (i32.const 7)))`},
		{"early-return-with-value", `
(module
  (func (export "main") (param i32) (result i32)
    (if (local.get 0) (then (i32.const 1) (return)))
    (i32.const 2)))`},
	}
	for _, tc := range cases {
		bin, err := wat.Compile([]byte(tc.src))
		if err != nil {
			t.Fatalf("%s compile: %v", tc.name, err)
		}
		// with and without a toll station: results must agree and neither trap
		var results [2][]uint64
		for i, withToll := range []bool{false, true} {
			cfg := config.ModuleConfig{}
			if withToll {
				cfg.TollStation = tollstation.NewSimpleTollStation(1 << 24)
			}
			mod, err := wasman.NewModule(cfg, bytes.NewReader(bin))
			if err != nil {
				t.Fatalf("%s toll=%v load: %v", tc.name, withToll, err)
			}
			ins, err := wasman.NewInstance(mod, nil)
			if err != nil {
				t.Fatalf("%s toll=%v inst: %v", tc.name, withToll, err)
			}
			var arg []uint64
			if len(mod.Exports()) > 0 && mod.Exports()[0].Type != nil && len(mod.Exports()[0].Type.InputTypes) > 0 {
				arg = []uint64{0} // the value that must NOT take the divergent branch
			}
			rets, _, err := ins.CallExportedFunc("main", arg...)
			if err != nil {
				t.Fatalf("%s toll=%v: trapped (BUG): %v", tc.name, withToll, err)
			}
			results[i] = rets
		}
		if len(results[0]) != len(results[1]) {
			t.Fatalf("%s: result arity differs with/without toll", tc.name)
		}
		for j := range results[0] {
			if results[0][j] != results[1][j] {
				t.Fatalf("%s result[%d]: no-toll %d, toll %d", tc.name, j, results[0][j], results[1][j])
			}
		}
	}
}
