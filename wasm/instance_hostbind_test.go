package wasm_test

import (
	"bytes"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/wat"
)

// Two modules import the same host function which reads THEIR memory.
// Each instance's import must stay bound to its own memory — the shared
// HostFunc must not be re-bound to the last instantiated module.
func TestHostFuncBindsPerInstance(t *testing.T) {
	src := func(marker byte) string {
		return `
(module
  (import "env" "peek" (func $peek (result i32)))
  (memory 1)
  (data (i32.const 0) "\` + string("0123456789abcdef"[marker>>4]) + string("0123456789abcdef"[marker&0xf]) + `")
  (func (export "read") (result i32) (call $peek)))
`
	}

	linker := wasman.NewLinker(config.LinkerConfig{})
	err := linker.DefineAdvancedFunc("env", "peek", func(ins *wasman.Instance) interface{} {
		return func() uint32 {
			return uint32(ins.Memory.Value[0])
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	newIns := func(marker byte) *wasman.Instance {
		bin, err := wat.Compile([]byte(src(marker)))
		if err != nil {
			t.Fatal(err)
		}
		mod, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
		if err != nil {
			t.Fatal(err)
		}
		ins, err := linker.Instantiate(mod)
		if err != nil {
			t.Fatal(err)
		}
		return ins
	}

	insA := newIns(0xaa)
	insB := newIns(0xbb) // instantiated later: must NOT steal A's binding

	retsA, _, err := insA.CallExportedFunc("read")
	if err != nil {
		t.Fatal(err)
	}
	if retsA[0] != 0xaa {
		t.Fatalf("module A read %#x from its import, want 0xaa (bound to the wrong instance's memory)", retsA[0])
	}

	retsB, _, err := insB.CallExportedFunc("read")
	if err != nil {
		t.Fatal(err)
	}
	if retsB[0] != 0xbb {
		t.Fatalf("module B read %#x, want 0xbb", retsB[0])
	}
}
