package wasm_test

import (
	"bytes"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/tollstation"
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

// TestMemoryCopySlowPath runs memory.copy under a toll station, which
// disables the inlined fast path — the same execution mode metered embedders use.
// A local read after the copy must survive (no operand/PC corruption).
func TestMemoryCopySlowPath(t *testing.T) {
	src := `
(module
  (memory 1)
  (data (i32.const 0) "\de\ad\be\ef")
  (func (export "t") (result i32)
    (local $x i32)
    (local.set $x (i32.const 12345))
    (memory.copy (i32.const 32) (i32.const 0) (i32.const 4))
    (memory.fill (i32.const 64) (i32.const 7) (i32.const 8))
    (local.get $x)))
`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod, err := wasman.NewModule(config.ModuleConfig{
		TollStation: tollstation.NewSimpleTollStation(1 << 20),
	}, bytes.NewReader(bin))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := wasman.NewLinker(config.LinkerConfig{}).Instantiate(mod)
	if err != nil {
		t.Fatal(err)
	}
	rets, _, err := ins.CallExportedFunc("t")
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if rets[0] != 12345 {
		t.Fatalf("local corrupted across bulk-memory ops (slow path): got %d, want 12345", rets[0])
	}
}

// TestBulkMemoryJIT runs the fill/copy exercises under the JIT and requires
// the results to match the interpreter, confirming the host-exit path the
// optimizing tier lowers these ops to.
func TestBulkMemoryJIT(t *testing.T) {
	src := `
(module
  (memory 1)
  (data (i32.const 0) "\de\ad\be\ef")
  (func (export "run") (result i32)
    (memory.fill (i32.const 100) (i32.const 0x77) (i32.const 50))
    (memory.copy (i32.const 200) (i32.const 0) (i32.const 4))
    (i32.add
      (i32.load8_u (i32.const 120))
      (i32.load8_u (i32.const 202)))))
`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, jit := range []bool{false, true} {
		mod, err := wasman.NewModule(config.ModuleConfig{EnableJIT: jit}, bytes.NewReader(bin))
		if err != nil {
			t.Fatalf("jit=%v: %v", jit, err)
		}
		ins, err := wasman.NewInstance(mod, nil)
		if err != nil {
			t.Fatalf("jit=%v: %v", jit, err)
		}
		rets, _, err := ins.CallExportedFunc("run")
		if err != nil {
			t.Fatalf("jit=%v: %v", jit, err)
		}
		if rets[0] != 0x77+0xBE {
			t.Fatalf("jit=%v got %#x want %#x", jit, rets[0], 0x77+0xBE)
		}
	}
}
