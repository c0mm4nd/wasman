package wasman

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/tollstation"
	"github.com/c0mm4nd/wasman/wat"
)

// TestJITHostCallInstanceBinding guards a consensus-critical JIT bug: when
// several modules are instantiated on ONE linker (as ngcore does for a
// contract and its service dependencies), each instance gets its own bound
// copy of a host import, but a single shared HostFunc is re-bound to whichever
// instance instantiated LAST. The JIT must invoke host calls through the
// per-instance binding (ins.Functions), exactly like the interpreter — not the
// shared object — or a contract's host call runs against another instance's
// memory and silently reads/writes zeros.
//
// The host func copies the CALLING instance's memory[0] into memory[8]; each
// instance is seeded with a distinct marker, so a mis-bound host call reads
// the wrong instance's memory. Covered for every JIT config (metered baseline
// and the optimizing tier) and for 3 instances in various orders.
func TestJITHostCallInstanceBinding(t *testing.T) {
	src := `(module
		(import "env" "copy0to8" (func $c))
		(memory (export "mem") 1)
		(func (export "run") (result i32)
			(call $c)
			(i32.load8_u (i32.const 8))))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	type mode struct {
		name string
		cfg  func() config.ModuleConfig
	}
	modes := []mode{
		{"interp", func() config.ModuleConfig { return config.ModuleConfig{} }},
		{"opt-jit", func() config.ModuleConfig { return config.ModuleConfig{EnableJIT: true} }},
		{"baseline+toll", func() config.ModuleConfig {
			return config.ModuleConfig{EnableJIT: true, TollStation: tollstation.NewSimpleTollStation(1 << 30)}
		}},
	}
	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			l := NewLinker(config.LinkerConfig{})
			l.DefineAdvancedFunc("env", "copy0to8", func(ins *Instance) interface{} {
				return func() {
					if ins.Memory != nil && len(ins.Memory.Value) > 8 {
						ins.Memory.Value[8] = ins.Memory.Value[0]
					}
				}
			})
			mk := func() *Instance {
				mod, err := NewModule(m.cfg(), bytes.NewReader(bin))
				if err != nil {
					t.Fatal(err)
				}
				ins, err := l.Instantiate(mod)
				if err != nil {
					t.Fatal(err)
				}
				return ins
			}
			// three instances on ONE linker; the shared HostFunc binds to the last
			a, b, c := mk(), mk(), mk()
			a.Memory.Value[0] = 0xA1
			b.Memory.Value[0] = 0xB2
			c.Memory.Value[0] = 0xC3
			// every instance's host call must see its OWN memory, regardless of
			// instantiation order relative to the shared last-binder (c)
			for _, tc := range []struct {
				ins  *Instance
				want uint64
			}{{a, 0xA1}, {b, 0xB2}, {c, 0xC3}, {a, 0xA1}} {
				r, _, err := tc.ins.CallExportedFunc("run")
				if err != nil {
					t.Fatal(err)
				}
				if r[0] != tc.want {
					t.Errorf("host call ran against wrong instance: got %#x want %#x", r[0], tc.want)
				}
			}
		})
	}
}

// TestJITWideIntInstanceBinding is the same guard for the wide-integer host
// modules (u256/u128): they resolve through the same per-instance binding and
// operate on the calling instance's linear memory, so a shared-object dispatch
// would compute a contract's u256 math against another contract's memory.
func TestJITWideIntInstanceBinding(t *testing.T) {
	// dst = a + b, all in this instance's memory
	src := `(module
		(import "u256" "add" (func $add (param i32 i32 i32)))
		(memory (export "mem") 1)
		(func (export "run") (result i64)
			(call $add (i32.const 64) (i32.const 0) (i32.const 32))
			(i64.load (i32.const 64))))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	l := NewLinker(config.LinkerConfig{})
	mk := func(seed uint64) *Instance {
		cfg := config.ModuleConfig{EnableJIT: true, EnableWideInt: true,
			TollStation: tollstation.NewSimpleTollStation(1 << 30)}
		mod, err := NewModule(cfg, bytes.NewReader(bin))
		if err != nil {
			t.Fatal(err)
		}
		ins, err := l.Instantiate(mod)
		if err != nil {
			t.Fatal(err)
		}
		binary.LittleEndian.PutUint64(ins.Memory.Value[0:], seed)  // a = seed
		binary.LittleEndian.PutUint64(ins.Memory.Value[32:], 1000) // b = 1000
		return ins
	}
	a, b := mk(5), mk(9)
	_ = b // b instantiated last -> shared u256.add binds here
	// a's u256.add must read a's memory (5 + 1000 = 1005), not b's (9 + 1000)
	r, _, err := a.CallExportedFunc("run")
	if err != nil {
		t.Fatal(err)
	}
	if r[0] != 1005 {
		t.Errorf("u256.add ran against wrong instance: got %d want 1005", r[0])
	}
}
