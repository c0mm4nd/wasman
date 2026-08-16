package wasm_test

import (
	"bytes"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/wat"
)

// TestMemoryFill exercises the bulk-memory memory.fill op: fill 4 bytes
// at offset 8 with 0xAB, then read one back.
func TestMemoryFill(t *testing.T) {
	src := `
(module
  (memory 1)
  (func (export "fill_and_read") (result i32)
    (memory.fill (i32.const 8) (i32.const 0xAB) (i32.const 4))
    (i32.load8_u (i32.const 10))))
`
	rets := runModule(t, src, "fill_and_read")
	if rets[0] != 0xAB {
		t.Fatalf("memory.fill wrote %#x, want 0xAB", rets[0])
	}
}

// TestMemoryCopy exercises memory.copy: seed bytes at 0..3, copy them to
// 16..19, read one back.
func TestMemoryCopy(t *testing.T) {
	src := `
(module
  (memory 1)
  (data (i32.const 0) "\de\ad\be\ef")
  (func (export "copy_and_read") (result i32)
    (memory.copy (i32.const 16) (i32.const 0) (i32.const 4))
    (i32.load8_u (i32.const 18))))
`
	rets := runModule(t, src, "copy_and_read")
	if rets[0] != 0xBE {
		t.Fatalf("memory.copy produced %#x at +2, want 0xBE", rets[0])
	}
}

// TestMemoryFillOutOfBounds ensures an over-long fill traps instead of
// corrupting memory past the page.
func TestMemoryFillOutOfBounds(t *testing.T) {
	src := `
(module
  (memory 1)
  (func (export "boom")
    (memory.fill (i32.const 65530) (i32.const 1) (i32.const 100))))
`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := wasman.NewLinker(config.LinkerConfig{}).Instantiate(mod)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ins.CallExportedFunc("boom"); err == nil {
		t.Fatal("out-of-bounds memory.fill did not trap")
	}
}

func runModule(t *testing.T, src, entry string) []uint64 {
	t.Helper()
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	mod, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ins, err := wasman.NewLinker(config.LinkerConfig{}).Instantiate(mod)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	rets, _, err := ins.CallExportedFunc(entry)
	if err != nil {
		t.Fatalf("call %s: %v", entry, err)
	}
	return rets
}
