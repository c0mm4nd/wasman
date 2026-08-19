package wasm

import (
	"testing"

	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
	"github.com/c0mm4nd/wasman/utils"
)

// The per-instance bound copy of a host function must carry every dispatch-
// relevant field through the REAL NewInstance path. Dropping wideOp silently
// downgrades built-in wide-integer operations from the reflection-free
// wideDirect path to reflect.Call — same results and toll, but a large
// per-call overhead on hot DeFi math.
func TestHostFuncCopyKeepsWideOp(t *testing.T) {
	sig := &types.FuncType{}
	shared := &HostFunc{
		Signature: sig,
		Generator: func(ins *Instance) interface{} { return func() {} },
	}
	shared.SetWideOp(42)

	host := &Module{
		IndexSpace: &IndexSpace{Functions: []fn{shared}},
		ExportSection: map[string]*segments.ExportSegment{
			"f": {Name: "f", Desc: &segments.ExportDesc{Kind: segments.KindFunction, Index: 0}},
		},
	}
	importer := &Module{
		ModuleConfig: config.ModuleConfig{},
		TypeSection:  []*types.FuncType{sig},
		ImportSection: []*segments.ImportSegment{{
			Module: "m", Name: "f",
			Desc: &segments.ImportDesc{Kind: segments.KindFunction, TypeIndexPtr: utils.Uint32Ptr(0)},
		}},
	}
	ins, err := NewInstance(importer, map[string]*Module{"m": host})
	if err != nil {
		t.Fatal(err)
	}
	bound, ok := ins.Functions[0].(*HostFunc)
	if !ok {
		t.Fatalf("ins.Functions[0] is %T, want *HostFunc", ins.Functions[0])
	}
	if bound == shared {
		t.Fatal("ins.Functions[0] is the shared object, not a per-instance copy")
	}
	if bound.wideOp != 42 {
		t.Fatalf("per-instance copy lost wideOp: got %d want 42", bound.wideOp)
	}
}
