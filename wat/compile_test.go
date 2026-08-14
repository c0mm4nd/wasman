package wat_test

import (
	"bytes"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/wat"
)

// mustCompileAndInstantiate compiles the text, loads it through the real
// decoder+validator, and instantiates it with the given linker
func mustCompileAndInstantiate(t *testing.T, src string, linker *wasman.Linker) *wasman.Instance {
	t.Helper()

	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	module, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
	if err != nil {
		t.Fatalf("decode/validate compiled binary: %v", err)
	}

	if linker == nil {
		linker = wasman.NewLinker(config.LinkerConfig{})
	}
	ins, err := linker.Instantiate(module)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	return ins
}

func call1(t *testing.T, ins *wasman.Instance, name string, args ...uint64) uint64 {
	t.Helper()
	rets, _, err := ins.CallExportedFunc(name, args...)
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	if len(rets) != 1 {
		t.Fatalf("call %s: got %d results", name, len(rets))
	}
	return rets[0]
}

func TestCompileArith(t *testing.T) {
	ins := mustCompileAndInstantiate(t, `
	(module
	  (func (export "add") (param i32 i32) (result i32)
	    local.get 0
	    local.get 1
	    i32.add))`, nil)

	if got := call1(t, ins, "add", 2, 40); got != 42 {
		t.Fatalf("add(2,40) = %d, want 42", got)
	}
}

func TestCompileNamedLocalsAndFolded(t *testing.T) {
	ins := mustCompileAndInstantiate(t, `
	(module
	  (func (export "mix") (param $a i32) (param $b i32) (result i32)
	    (local $tmp i32)
	    (local.set $tmp (i32.mul (local.get $a) (i32.const 3)))
	    (i32.sub (local.get $tmp) (local.get $b))))`, nil)

	if got := call1(t, ins, "mix", 10, 8); got != 22 {
		t.Fatalf("mix(10,8) = %d, want 22", got)
	}
}

func TestCompileBlockLoopBr(t *testing.T) {
	ins := mustCompileAndInstantiate(t, `
	(module
	  (func (export "sum") (param $n i32) (result i32)
	    (local $acc i32)
	    (block $out
	      (loop $again
	        (br_if $out (i32.eqz (local.get $n)))
	        (local.set $acc (i32.add (local.get $acc) (local.get $n)))
	        (local.set $n (i32.sub (local.get $n) (i32.const 1)))
	        (br $again)))
	    (local.get $acc)))`, nil)

	if got := call1(t, ins, "sum", 10); got != 55 {
		t.Fatalf("sum(10) = %d, want 55", got)
	}
}

func TestCompileIfElse(t *testing.T) {
	ins := mustCompileAndInstantiate(t, `
	(module
	  (func (export "max") (param $a i32) (param $b i32) (result i32)
	    (if (result i32) (i32.gt_s (local.get $a) (local.get $b))
	      (then (local.get $a))
	      (else (local.get $b)))))`, nil)

	if got := call1(t, ins, "max", 3, 9); got != 9 {
		t.Fatalf("max(3,9) = %d, want 9", got)
	}
	if got := call1(t, ins, "max", 12, 9); got != 12 {
		t.Fatalf("max(12,9) = %d, want 12", got)
	}
}

func TestCompileMemoryAndData(t *testing.T) {
	ins := mustCompileAndInstantiate(t, `
	(module
	  (memory (export "mem") 1)
	  (data (i32.const 8) "\01\02hello")
	  (func (export "peek") (param $p i32) (result i32)
	    (i32.load8_u (local.get $p))))`, nil)

	if got := call1(t, ins, "peek", 8); got != 1 {
		t.Fatalf("peek(8) = %d, want 1", got)
	}
	if got := call1(t, ins, "peek", 10); got != 'h' {
		t.Fatalf("peek(10) = %d, want 'h'", got)
	}
	if !bytes.Equal(ins.Memory.Value[8:15], []byte("\x01\x02hello")) {
		t.Fatal("data segment not initialized")
	}
}

func TestCompileImportsAndCall(t *testing.T) {
	linker := wasman.NewLinker(config.LinkerConfig{})
	var logged []uint64
	err := linker.DefineAdvancedFunc("env", "note", func(ins *wasman.Instance) interface{} {
		return func(v uint32) {
			logged = append(logged, uint64(v))
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	ins := mustCompileAndInstantiate(t, `
	(module
	  (import "env" "note" (func $note (param i32)))
	  (func $twice (param $v i32)
	    (call $note (local.get $v))
	    (call $note (local.get $v)))
	  (func (export "go")
	    (call $twice (i32.const 7))))`, linker)

	if _, _, err := ins.CallExportedFunc("go"); err != nil {
		t.Fatal(err)
	}
	if len(logged) != 2 || logged[0] != 7 || logged[1] != 7 {
		t.Fatalf("host calls = %v, want [7 7]", logged)
	}
}

func TestCompileGlobals(t *testing.T) {
	ins := mustCompileAndInstantiate(t, `
	(module
	  (global $counter (mut i32) (i32.const 100))
	  (func (export "bump") (result i32)
	    (global.set $counter (i32.add (global.get $counter) (i32.const 1)))
	    (global.get $counter)))`, nil)

	if got := call1(t, ins, "bump"); got != 101 {
		t.Fatalf("bump() = %d, want 101", got)
	}
	if got := call1(t, ins, "bump"); got != 102 {
		t.Fatalf("bump() = %d, want 102", got)
	}
}

func TestCompileI64AndMemarg(t *testing.T) {
	ins := mustCompileAndInstantiate(t, `
	(module
	  (memory 1)
	  (func (export "roundtrip") (param $v i64) (result i64)
	    (i64.store offset=16 (i32.const 0) (local.get $v))
	    (i64.load offset=16 align=8 (i32.const 0))))`, nil)

	if got := call1(t, ins, "roundtrip", 0xdeadbeefcafe); got != 0xdeadbeefcafe {
		t.Fatalf("roundtrip = %x, want deadbeefcafe", got)
	}
}

func TestCompileNegativeConst(t *testing.T) {
	ins := mustCompileAndInstantiate(t, `
	(module
	  (func (export "neg") (result i32)
	    (i32.add (i32.const -40) (i32.const -2))))`, nil)

	if got := int32(uint32(call1(t, ins, "neg"))); got != -42 {
		t.Fatalf("neg() = %d, want -42", got)
	}
}

func TestCompileDeterministic(t *testing.T) {
	src := []byte(`
	(module
	  (import "env" "f" (func $f (param i32) (result i32)))
	  (memory 1) (data (i32.const 0) "abc")
	  (global $g (mut i64) (i64.const 5))
	  (func (export "main") (result i32) (call $f (i32.const 1))))`)

	a, err := wat.Compile(src)
	if err != nil {
		t.Fatal(err)
	}
	b, err := wat.Compile(src)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("compilation is not deterministic")
	}
}

func TestCompileRejectsGarbage(t *testing.T) {
	for _, src := range []string{
		`(module (func (export "x") (i32.bogus)))`,
		`(module (func (block $a (br $b))))`,
		`(module (funk))`,
		`(module (func (call $missing)))`,
	} {
		if _, err := wat.Compile([]byte(src)); err == nil {
			t.Fatalf("compile(%q) should fail", src)
		}
	}
}
