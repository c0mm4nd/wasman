package wasman_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/wat"
)

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestSmallBranches(t *testing.T) {
	// NewModule propagates reader failures
	if _, err := wasman.NewModule(config.ModuleConfig{}, failReader{}); err == nil {
		t.Fatal("failing reader accepted")
	}

	// DefineMemory on a linker that has never seen the module name creates it
	l := wasman.NewLinker(config.LinkerConfig{})
	if err := l.DefineMemory("freshmod", "mem", make([]byte, 100)); err != nil {
		t.Fatal(err)
	}
	// a partial page rounds the declared minimum up
	src := `(module (import "freshmod" "mem" (memory 1))
		(func (export "sz") (result i32) (memory.size)))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Instantiate(mod); err != nil {
		t.Fatalf("imported host memory: %v", err)
	}

	// DefineAdvancedFunc rejects generators yielding invalid signatures
	if err := l.DefineAdvancedFunc("freshmod", "bad", func(ins *wasman.Instance) interface{} {
		return func(s string) {}
	}); err == nil {
		t.Fatal("invalid advanced signature accepted")
	}

	// EnableWideInt merges the u128/u256 modules WITH caller-provided extern
	// modules (both must resolve)
	host := wasman.NewLinker(config.LinkerConfig{})
	host.DefineAdvancedFunc("env", "id", func(ins *wasman.Instance) interface{} {
		return func(x uint32) uint32 { return x }
	})
	src2 := `(module
		(import "env" "id" (func $id (param i32) (result i32)))
		(import "u256" "iszero" (func $z (param i32) (result i32)))
		(memory 1)
		(func (export "go") (result i32)
			(i32.add (call $id (i32.const 40)) (call $z (i32.const 0)))))`
	bin2, err := wat.Compile([]byte(src2))
	if err != nil {
		t.Fatal(err)
	}
	mod2, err := wasman.NewModule(config.ModuleConfig{EnableWideInt: true}, bytes.NewReader(bin2))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := wasman.NewInstance(mod2, host.Modules)
	if err != nil {
		t.Fatal(err)
	}
	// memory[0:32] is zero, so iszero contributes 1: 40 + 1 + 1 more below
	r, _, err := ins.CallExportedFunc("go")
	if err != nil || r[0] != 41 {
		t.Fatalf("wideint+extern merge: %v %v", r, err)
	}
}
