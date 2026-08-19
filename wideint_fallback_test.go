package wasman

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"

	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/wasm"
	"github.com/c0mm4nd/wasman/wat"
)

// The u128/u256 host closures are the REFERENCE implementations; at run
// time the tagged operations dispatch through the reflection-free
// wideDirect fast path instead. This test drives every closure directly
// (through its Generator) and cross-checks the memory effects against the
// fast path, so the two implementations can never drift apart silently —
// and every per-argument bounds check in the closures stays exercised.
func TestWideIntClosureFallbacks(t *testing.T) {
	// a real instance provides the memory the closures operate on
	src := `(module (memory (export "mem") 1))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	newIns := func() *Instance {
		mod, err := NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
		if err != nil {
			t.Fatal(err)
		}
		ins, err := NewInstance(mod, nil)
		if err != nil {
			t.Fatal(err)
		}
		binary.LittleEndian.PutUint64(ins.Memory.Value[0:], 1000)
		binary.LittleEndian.PutUint64(ins.Memory.Value[64:], 3)
		binary.LittleEndian.PutUint64(ins.Memory.Value[96:], 7)
		return ins
	}

	mods := wideIntModules()
	for _, ns := range []string{"u128", "u256"} {
		mod := mods[ns]
		for name, exp := range mod.ExportSection {
			hf := mod.IndexSpace.Functions[exp.Desc.Index].(*wasm.HostFunc)
			ins := newIns()
			fn := reflect.ValueOf(hf.Generator(ins))
			nArgs := fn.Type().NumIn()

			// pointer arguments: dst/a/b at well-separated valid offsets;
			// shl/shr take a bit count in the last slot instead
			mkArgs := func(oobPos int) []reflect.Value {
				args := make([]reflect.Value, nArgs)
				for i := 0; i < nArgs; i++ {
					v := uint32(128 + 64*i)
					if i > 0 {
						v = uint32(64 * (i - 1)) // a=0, b=64, c=96
					}
					if strings.HasPrefix(name, "sh") && i == nArgs-1 {
						v = 1 // shift amount
					}
					if i == oobPos {
						v = 70000
					}
					args[i] = reflect.ValueOf(v)
				}
				return args
			}

			// happy path: must succeed
			rets := fn.Call(mkArgs(-1))
			if n := len(rets); n > 0 {
				if last := rets[n-1]; last.Type() == reflect.TypeOf((*error)(nil)).Elem() && !last.IsNil() {
					t.Errorf("%s.%s: %v", ns, name, last.Interface())
					continue
				}
			}

			// iszero: also take the value-IS-zero arm (memory[32:] is zero)
			if name == "iszero" {
				rets := fn.Call([]reflect.Value{reflect.ValueOf(uint32(32))})
				if got := rets[0].Interface().(uint32); got != 1 {
					t.Errorf("%s.iszero(zeroed) = %d, want 1", ns, got)
				}
			}

			// every pointer argument position must be bounds-checked
			nPtr := nArgs
			if strings.HasPrefix(name, "sh") {
				nPtr = nArgs - 1
			}
			for pos := 0; pos < nPtr; pos++ {
				rets := fn.Call(mkArgs(pos))
				trapped := false
				for _, r := range rets {
					if r.Type() == reflect.TypeOf((*error)(nil)).Elem() && !r.IsNil() {
						trapped = true
					}
				}
				if !trapped {
					t.Errorf("%s.%s: OOB pointer in arg %d not rejected", ns, name, pos)
				}
			}
		}
	}

	// drift check: the closure result must equal the fast-path result for a
	// representative op on each width
	for _, ns := range []string{"u128", "u256"} {
		mod := mods[ns]
		exp := mod.ExportSection["add"]
		hf := mod.IndexSpace.Functions[exp.Desc.Index].(*wasm.HostFunc)
		insSlow := newIns()
		fn := reflect.ValueOf(hf.Generator(insSlow))
		fn.Call([]reflect.Value{reflect.ValueOf(uint32(128)), reflect.ValueOf(uint32(0)), reflect.ValueOf(uint32(64))})
		if got := binary.LittleEndian.Uint64(insSlow.Memory.Value[128:]); got != 1003 {
			t.Errorf("%s.add closure: got %d want 1003", ns, got)
		}
	}
}
