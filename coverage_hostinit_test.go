package wasman_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/wat"
)

// every u128 and u256 host operation, called from wasm: exercises the full
// reflection-free wideDirect dispatch table with checked results.
func TestWideIntFullDispatch(t *testing.T) {
	ops3 := []string{"add", "sub", "mul", "div_u", "rem_u", "div_s", "rem_s", "and", "or", "xor", "shl", "shr_u", "shr_s"}
	var b bytes.Buffer
	b.WriteString("(module\n")
	for _, ns := range []string{"u128", "u256"} {
		for _, op := range ops3 {
			b.WriteString(`(import "` + ns + `" "` + op + `" (func $` + ns + op + ` (param i32 i32 i32)))` + "\n")
		}
		b.WriteString(`(import "` + ns + `" "not" (func $` + ns + `not (param i32 i32)))` + "\n")
		b.WriteString(`(import "` + ns + `" "cmp_u" (func $` + ns + `cmpu (param i32 i32) (result i32)))` + "\n")
		b.WriteString(`(import "` + ns + `" "cmp_s" (func $` + ns + `cmps (param i32 i32) (result i32)))` + "\n")
		b.WriteString(`(import "` + ns + `" "iszero" (func $` + ns + `isz (param i32) (result i32)))` + "\n")
	}
	b.WriteString(`(import "u256" "mul_div" (func $md (param i32 i32 i32 i32)))
(import "u256" "isqrt" (func $sq (param i32 i32)))
(memory 1)
(func (export "run") (result i32)
	(local $acc i32)`)
	for _, ns := range []string{"u128", "u256"} {
		for _, op := range ops3 {
			b.WriteString("\n(call $" + ns + op + " (i32.const 128) (i32.const 0) (i32.const 64))")
		}
		b.WriteString("\n(call $" + ns + "not (i32.const 128) (i32.const 0))")
		b.WriteString("\n(local.set $acc (i32.add (local.get $acc) (call $" + ns + "cmpu (i32.const 0) (i32.const 64))))")
		b.WriteString("\n(local.set $acc (i32.add (local.get $acc) (call $" + ns + "cmps (i32.const 0) (i32.const 64))))")
		b.WriteString("\n(local.set $acc (i32.add (local.get $acc) (call $" + ns + "isz (i32.const 0))))")
	}
	b.WriteString(`
	(call $md (i32.const 160) (i32.const 0) (i32.const 64) (i32.const 96))
	(call $sq (i32.const 192) (i32.const 0))
	(local.get $acc)))`)
	bin, err := wat.Compile(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	run := func(jit bool) (uint64, []byte) {
		cfg := config.ModuleConfig{EnableWideInt: true, EnableJIT: jit}
		mod, err := wasman.NewModule(cfg, bytes.NewReader(bin))
		if err != nil {
			t.Fatal(err)
		}
		ins, err := wasman.NewInstance(mod, nil)
		if err != nil {
			t.Fatal(err)
		}
		binary.LittleEndian.PutUint64(ins.Memory.Value[0:], 1000) // a
		binary.LittleEndian.PutUint64(ins.Memory.Value[64:], 3)   // b
		binary.LittleEndian.PutUint64(ins.Memory.Value[96:], 7)   // c
		r, _, err := ins.CallExportedFunc("run")
		if err != nil {
			t.Fatal(err)
		}
		return r[0], append([]byte(nil), ins.Memory.Value[:256]...)
	}
	iv, im := run(false)
	jv, jm := run(true)
	// cmp_u(1000,3)=1 twice, cmp_s same, iszero(1000)=0: acc = 2*(1+1+0) = 4
	if iv != 4 || jv != 4 {
		t.Fatalf("acc: interp=%d jit=%d want 4", iv, jv)
	}
	if !bytes.Equal(im, jm) {
		t.Fatal("interp/jit wideint memory diverged")
	}
	// out-of-bounds pointer traps
	cfg := config.ModuleConfig{EnableWideInt: true}
	mod, _ := wasman.NewModule(cfg, bytes.NewReader(bin))
	ins, _ := wasman.NewInstance(mod, nil)
	ins.Memory.Value = ins.Memory.Value[:64] // shrink so ptr 128 is OOB
	if _, _, err := ins.CallExportedFunc("run"); err == nil {
		t.Fatal("OOB wideint pointer: expected a trap")
	}
}

// host functions with float parameters and returns go through the
// reflect-based bridge.
func TestHostFloatBridge(t *testing.T) {
	src := `(module
		(import "m" "mix" (func $mix (param f32 f64 i32) (result f64)))
		(func (export "go") (result f64)
			(call $mix (f32.const 1.5) (f64.const 2.25) (i32.const 4))))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	l := wasman.NewLinker(config.LinkerConfig{})
	l.DefineAdvancedFunc("m", "mix", func(ins *wasman.Instance) interface{} {
		return func(a float32, b float64, c uint32) float64 {
			return float64(a) + b + float64(c)
		}
	})
	mod, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := l.Instantiate(mod)
	if err != nil {
		t.Fatal(err)
	}
	r, _, err := ins.CallExportedFunc("go")
	if err != nil {
		t.Fatal(err)
	}
	if got := binaryFloat64(r[0]); got != 7.75 {
		t.Fatalf("mix = %v, want 7.75", got)
	}

	// an f32 RETURN travels back as the 32-bit pattern
	src32 := `(module
		(import "m" "half" (func $half (param f32) (result f32)))
		(func (export "go") (result i32)
			(i32.reinterpret_f32 (call $half (f32.const 5)))))`
	bin32, err := wat.Compile([]byte(src32))
	if err != nil {
		t.Fatal(err)
	}
	l32 := wasman.NewLinker(config.LinkerConfig{})
	l32.DefineAdvancedFunc("m", "half", func(ins *wasman.Instance) interface{} {
		return func(x float32) float32 { return x / 2 }
	})
	mod32, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin32))
	if err != nil {
		t.Fatal(err)
	}
	ins32, err := l32.Instantiate(mod32)
	if err != nil {
		t.Fatal(err)
	}
	r32, _, err := ins32.CallExportedFunc("go")
	if err != nil {
		t.Fatal(err)
	}
	if uint32(r32[0]) != math.Float32bits(2.5) {
		t.Fatalf("f32 return bits = %#x, want %#x", uint32(r32[0]), math.Float32bits(2.5))
	}
}

func binaryFloat64(bits uint64) float64 { return math.Float64frombits(bits) }

// start-function checks at instantiation: an index out of range and a
// non-nullary signature are runtime rejects (the module fields are public,
// mirroring a hand-built embedding).
func TestStartFunctionRejects(t *testing.T) {
	src := `(module (func (export "f") (param i32)))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
	if err != nil {
		t.Fatal(err)
	}
	mod.StartSection = []uint32{9}
	if _, err := wasman.NewInstance(mod, nil); err == nil {
		t.Fatal("start index out of range accepted")
	}
	mod2, _ := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
	mod2.StartSection = []uint32{0} // takes a param: invalid start signature
	if _, err := wasman.NewInstance(mod2, nil); err == nil ||
		!strings.Contains(err.Error(), "start function") {
		t.Fatalf("bad start signature: %v", err)
	}
	// a trapping start function aborts instantiation
	src3 := `(module (func $boom (unreachable)) (start $boom))`
	bin3, err := wat.Compile([]byte(src3))
	if err != nil {
		t.Fatal(err)
	}
	mod3, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin3))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wasman.NewInstance(mod3, nil); err == nil {
		t.Fatal("trapping start accepted")
	}
}

// import resolution failures: missing module, missing name, wrong kind and
// signature mismatch each surface a distinct error.
func TestImportResolutionErrors(t *testing.T) {
	mk := func(src string) *wasman.Module {
		bin, err := wat.Compile([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		mod, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
		if err != nil {
			t.Fatal(err)
		}
		return mod
	}
	l := wasman.NewLinker(config.LinkerConfig{})
	l.DefineAdvancedFunc("env", "f", func(ins *wasman.Instance) interface{} {
		return func() uint32 { return 1 }
	})
	cases := []struct{ name, src string }{
		{"missing module", `(module (import "ghost" "f" (func)))`},
		{"missing name", `(module (import "env" "ghost" (func)))`},
		{"sig mismatch", `(module (import "env" "f" (func (param i32))))`},
		{"kind mismatch", `(module (import "env" "f" (memory 1)))`},
	}
	for _, c := range cases {
		if _, err := l.Instantiate(mk(c.src)); err == nil {
			t.Errorf("%s: accepted", c.name)
		}
	}
	// export-kind lookup: a global exported under the imported name
	l2 := wasman.NewLinker(config.LinkerConfig{})
	if err := l2.DefineGlobal("env", "g", int64(5)); err != nil {
		t.Fatal(err)
	}
	if _, err := l2.Instantiate(mk(`(module (import "env" "g" (func)))`)); err == nil {
		t.Error("func import resolved against a global export")
	}
	if _, err := l2.Instantiate(mk(`(module (import "env" "g" (global (mut i64))) (func (export "r") (result i64) (global.get 0)))`)); err != nil {
		t.Errorf("global import: %v", err)
	}
}

// a callee trapping across a module boundary unwinds through callCross.
func TestCallCrossTrap(t *testing.T) {
	calleeSrc := `(module (func (export "boom") (result i32) (unreachable)))`
	callerSrc := `(module (import "b" "boom" (func $b (result i32)))
		(func (export "go") (result i32) (call $b)))`
	cb, _ := wat.Compile([]byte(calleeSrc))
	ca, _ := wat.Compile([]byte(callerSrc))
	l := wasman.NewLinker(config.LinkerConfig{})
	modB, err := wasman.NewModule(config.ModuleConfig{Recover: true}, bytes.NewReader(cb))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Instantiate(modB); err != nil {
		t.Fatal(err)
	}
	l.Define("b", modB)
	modA, err := wasman.NewModule(config.ModuleConfig{Recover: true}, bytes.NewReader(ca))
	if err != nil {
		t.Fatal(err)
	}
	insA, err := l.Instantiate(modA)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := insA.CallExportedFunc("go"); err == nil {
		t.Fatal("cross-module trap swallowed")
	}
	_ = segments.KindFunction // keep the import if cases evolve
}
