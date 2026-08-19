package wat_test

import (
	"bytes"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/wat"
)

// mustCompileAndDecode compiles the text and checks the binary loads through
// the real decoder+validator (without instantiating, so import-only modules
// work too)
func mustCompileAndDecode(t *testing.T, src string) {
	t.Helper()

	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if _, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin)); err != nil {
		t.Fatalf("decode/validate compiled binary: %v", err)
	}
}

// TestCompileUnusualForms covers valid-but-unusual syntax: named imported
// memories, inline data on a named memory, legacy bare table indices in elem
// segments, string-form identifiers and every string escape shape.
func TestCompileUnusualForms(t *testing.T) {
	for _, c := range []struct {
		name, src string
	}{
		{"named memory import", `(module (import "m" "n" (memory $mm 1)) (export "m2" (memory $mm)))`},
		{"inline data on named memory", `(module (memory $m (data "hi")) (export "mem" (memory $m)))`},
		{"elem legacy bare table index", `(module (table 1 funcref) (func $f) (elem 0 (i32.const 0) $f))`},
		{"data legacy bare memory index", `(module (memory 1) (data 0 (i32.const 0) "x"))`},
		{"string-form id", `(module (func $"my func" (export "f")) (start $"my func"))`},
		{"string escapes", `(module (func (export "a\tb\nc\rd\u{1_F600}")))`},
	} {
		t.Run(c.name, func(t *testing.T) {
			mustCompileAndDecode(t, c.src)
		})
	}
}
