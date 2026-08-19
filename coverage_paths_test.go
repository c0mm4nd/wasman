package wasman_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/wasm"
	"github.com/c0mm4nd/wasman/wat"
)

// call_indirect under the native-ABI optimizing tier: the in-table fast
// path for a matching entry, and the runtime traps (type mismatch, null
// entry, index out of bounds) that exit through the mirror checks.
func TestOptJITIndirect(t *testing.T) {
	src := `(module
		(type $ii (func (param i32) (result i32)))
		(type $v (func))
		(table 4 funcref)
		(func $double (type $ii) (i32.mul (local.get 0) (i32.const 2)))
		(func $noop (type $v))
		(elem (i32.const 0) $double $noop)
		(func (export "go") (param i32 i32) (result i32)
			(call_indirect (type $ii) (local.get 1) (local.get 0))))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod, err := wasman.NewModule(config.ModuleConfig{EnableJIT: true}, bytes.NewReader(bin))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := wasman.NewInstance(mod, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r, _, err := ins.CallExportedFunc("go", 0, 21); err != nil || r[0] != 42 {
		t.Fatalf("fast indirect: %v %v", r, err)
	}
	for name, slot := range map[string]uint64{"type mismatch": 1, "null entry": 2, "out of bounds": 9} {
		if _, _, err := ins.CallExportedFunc("go", slot, 1); err == nil {
			t.Errorf("%s (slot %d): expected a trap", name, slot)
		}
	}
	// the instance must stay usable after each trap
	if r, _, err := ins.CallExportedFunc("go", 0, 5); err != nil || r[0] != 10 {
		t.Fatalf("after traps: %v %v", r, err)
	}
}

// native-ABI error paths: a trapping callee deep in a native call chain,
// and call-depth exhaustion enforced inside the generated prologue.
func TestOptJITNativeTraps(t *testing.T) {
	src := `(module
		(func $div (param i32 i32) (result i32) (i32.div_s (local.get 0) (local.get 1)))
		(func $mid (param i32) (result i32) (call $div (i32.const 6) (local.get 0)))
		(func (export "run") (param i32) (result i32) (call $mid (local.get 0)))
		(func $rec (param i32) (result i32)
			(if (result i32) (i32.eqz (local.get 0))
				(then (i32.const 0))
				(else (call $rec (i32.sub (local.get 0) (i32.const 1))))))
		(func (export "deep") (param i32) (result i32) (call $rec (local.get 0))))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	depth := uint64(64)
	mod, err := wasman.NewModule(config.ModuleConfig{EnableJIT: true, CallDepthLimit: &depth},
		bytes.NewReader(bin))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := wasman.NewInstance(mod, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r, _, err := ins.CallExportedFunc("run", 3); err != nil || r[0] != 2 {
		t.Fatalf("native chain: %v %v", r, err)
	}
	if _, _, err := ins.CallExportedFunc("run", 0); err == nil {
		t.Fatal("div by zero in native chain: expected a trap")
	}
	if r, _, err := ins.CallExportedFunc("deep", 10); err != nil || r[0] != 0 {
		t.Fatalf("shallow recursion: %v %v", r, err)
	}
	if _, _, err := ins.CallExportedFunc("deep", 100000); !errors.Is(err, wasm.ErrCallStackExhausted) {
		t.Fatalf("deep recursion: want ErrCallStackExhausted, got %v", err)
	}
}

// variedTollStation prices opcodes non-uniformly: such a station cannot be
// metered inline, so the JIT must stay off and the metered interpreter must
// still produce the right result.
type variedTollStation struct{ total, max uint64 }

func (v *variedTollStation) GetOpPrice(op byte) uint64 { return uint64(op&3) + 1 }
func (v *variedTollStation) GetToll() uint64           { return v.total }
func (v *variedTollStation) AddToll(t uint64) error    { v.total += t; return nil }
func (v *variedTollStation) GetMax() uint64            { return v.max }

func TestNonUniformTollDisablesJIT(t *testing.T) {
	src := `(module (func (export "f") (result i32)
		(i32.add (i32.const 40) (i32.const 2))))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	ts := &variedTollStation{max: 1 << 30}
	mod, err := wasman.NewModule(config.ModuleConfig{EnableJIT: true, TollStation: ts},
		bytes.NewReader(bin))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := wasman.NewInstance(mod, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, _, err := ins.CallExportedFunc("f")
	if err != nil || r[0] != 42 {
		t.Fatalf("non-uniform toll run: %v %v", r, err)
	}
	if ts.total == 0 {
		t.Fatal("non-uniform station was never charged (JIT bypassed metering?)")
	}
}

// CallExportedFunc error surface: unknown export, non-function export,
// wrong arity, and a body (validation skipped) returning fewer values than
// its type promises.
func TestCallExportedFuncErrors(t *testing.T) {
	src := `(module
		(memory (export "mem") 1)
		(func (export "one") (param i32) (result i32) (local.get 0)))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := wasman.NewInstance(mod, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ins.CallExportedFunc("nope"); !errors.Is(err, wasm.ErrExportedFuncNotFound) {
		t.Fatalf("unknown export: %v", err)
	}
	if _, _, err := ins.CallExportedFunc("mem"); !errors.Is(err, wasm.ErrExportedFuncNotFound) {
		t.Fatalf("non-function export: %v", err)
	}
	if _, _, err := ins.CallExportedFunc("one"); !errors.Is(err, wasm.ErrInvalidArgNum) {
		t.Fatalf("arity mismatch: %v", err)
	}

	// SkipValidation lets an ill-typed body through; the missing result must
	// come back as a clean error, not a stack underflow
	badSrc := `(module (func (export "lie") (result i32) (nop)))`
	badBin, err := wat.Compile([]byte(badSrc))
	if err != nil {
		t.Fatal(err)
	}
	badMod, err := wasman.NewModule(config.ModuleConfig{SkipValidation: true}, bytes.NewReader(badBin))
	if err != nil {
		t.Fatal(err)
	}
	badIns, err := wasman.NewInstance(badMod, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := badIns.CallExportedFunc("lie"); err == nil {
		t.Fatal("missing result value: expected an error")
	}
}

// Reset's reallocation branch: when the live memory's capacity shrank below
// the snapshot, Reset must allocate a fresh buffer.
func TestResetReallocates(t *testing.T) {
	src := `(module (memory 1)
		(data (i32.const 0) "seed")
		(func (export "smash") (i32.store (i32.const 0) (i32.const -1))))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := wasman.NewInstance(mod, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ins.CallExportedFunc("smash"); err != nil {
		t.Fatal(err)
	}
	// force the make() branch by capping the slice below the snapshot length
	ins.Memory.Value = ins.Memory.Value[:8:8]
	ins.Reset()
	if got := string(ins.Memory.Value[:4]); got != "seed" {
		t.Fatalf("Reset did not restore the snapshot: %q", got)
	}
}

// a context cancelled while the body is spinning interrupts the run.
func TestContextCancelMidRun(t *testing.T) {
	src := `(module
		(import "env" "tick" (func $tick))
		(func (export "spin") (loop $l (call $tick) (br $l))))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	l := wasman.NewLinker(config.LinkerConfig{})
	l.DefineAdvancedFunc("env", "tick", func(ins *wasman.Instance) interface{} { return func() {} })
	mod, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := l.Instantiate(mod)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, _, err := ins.CallExportedFuncWithContext(ctx, "spin"); err == nil {
		t.Fatal("cancelled context: expected an error")
	}
}
