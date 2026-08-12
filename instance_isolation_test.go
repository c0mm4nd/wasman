package wasman

import (
	"bytes"
	"testing"

	"github.com/c0mm4nd/wasman/config"
)

// counterModule is a hand-assembled wasm module:
//
//	(module
//	  (table 1 funcref) (elem (i32.const 0) 0)
//	  (global $g (mut i32) (i32.const 0))
//	  (func $run (export "run") (global.set $g (i32.add (global.get $g) (i32.const 1))))
//	  (export "g" (global $g)))
var counterModule = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // magic+version
	0x01, 0x04, 0x01, 0x60, 0x00, 0x00, // type: () -> ()
	0x03, 0x02, 0x01, 0x00, // func: 1 x type 0
	0x04, 0x04, 0x01, 0x70, 0x00, 0x01, // table: funcref min 1
	0x06, 0x06, 0x01, 0x7f, 0x01, 0x41, 0x00, 0x0b, // global: (mut i32) = 0
	0x07, 0x0b, 0x02, 0x03, 0x72, 0x75, 0x6e, 0x00, 0x00, 0x01, 0x67, 0x03, 0x00, // exports: run, g
	0x09, 0x07, 0x01, 0x00, 0x41, 0x00, 0x0b, 0x01, 0x00, // elem: table 0 @0 -> func 0
	0x0a, 0x0b, 0x01, 0x09, 0x00, 0x23, 0x00, 0x41, 0x01, 0x6a, 0x24, 0x00, 0x0b, // code
}

// TestInstanceIsolation guards against re-instantiating the same *Module
// corrupting earlier instances: each instance must keep its own index spaces
// (globals/tables), not the Module's latest ones.
func TestInstanceIsolation(t *testing.T) {
	mod, err := NewModule(config.ModuleConfig{}, bytes.NewReader(counterModule))
	if err != nil {
		t.Fatal(err)
	}

	a, err := NewInstance(mod, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, _, err := a.CallExportedFunc("run"); err != nil {
			t.Fatal(err)
		}
	}
	if got := int32(a.Globals[0]); got != 2 {
		t.Fatalf("a.g = %d, want 2", got)
	}

	// a second instantiation of the SAME module must not affect instance a
	b, err := NewInstance(mod, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.IndexSpace == b.IndexSpace {
		t.Error("instances share an IndexSpace: re-instantiation corrupts earlier instances")
	}
	if a.IndexSpace.Tables[0] == b.IndexSpace.Tables[0] {
		t.Error("instances share a table object")
	}
	if _, _, err := a.CallExportedFunc("run"); err != nil {
		t.Fatal(err)
	}
	if got := int32(a.Globals[0]); got != 3 {
		t.Errorf("a.g = %d after 3 runs, want 3 (instance state leaked)", got)
	}
	if got := int32(b.Globals[0]); got != 0 {
		t.Errorf("b.g = %d, want 0 (b polluted by a)", got)
	}
}
