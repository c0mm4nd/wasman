package wasman

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/tollstation"
	"github.com/c0mm4nd/wasman/wasm"
	"github.com/c0mm4nd/wasman/wat"
)

// TestJITHostCallInstanceBinding guards a consensus-critical JIT bug: when
// several modules are instantiated on ONE linker (as an embedder does for a
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

// call_indirect to an IMPORTED HOST function via an element segment, with
// two instances on one linker. The table slot must hold the calling
// instance's own binding, not the shared last-binder object.
func TestIndirectHostBinding(t *testing.T) {
	src := `(module
		(import "env" "copy0to8" (func $h))
		(memory (export "mem") 1)
		(table 1 funcref)
		(elem (i32.const 0) $h)
		(func (export "run") (result i32)
			(call_indirect (i32.const 0))
			(i32.load8_u (i32.const 8))))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Skipf("wat: %v", err)
	}
	for _, mode := range []struct {
		name string
		jit  bool
	}{{"interp", false}, {"jit+toll", true}} {
		t.Run(mode.name, func(t *testing.T) {
			l := NewLinker(config.LinkerConfig{})
			l.DefineAdvancedFunc("env", "copy0to8", func(ins *Instance) interface{} {
				return func() {
					if ins.Memory != nil && len(ins.Memory.Value) > 8 {
						ins.Memory.Value[8] = ins.Memory.Value[0]
					}
				}
			})
			mk := func() *Instance {
				cfg := config.ModuleConfig{EnableJIT: mode.jit}
				if mode.jit {
					cfg.TollStation = tollstation.NewSimpleTollStation(1 << 30)
				}
				mod, err := NewModule(cfg, bytes.NewReader(bin))
				if err != nil {
					t.Fatal(err)
				}
				ins, err := l.Instantiate(mod)
				if err != nil {
					t.Fatal(err)
				}
				return ins
			}
			a, b := mk(), mk()
			a.Memory.Value[0] = 0xA1
			b.Memory.Value[0] = 0xB2
			r, _, err := a.CallExportedFunc("run")
			if err != nil {
				t.Fatal(err)
			}
			if r[0] != 0xA1 {
				t.Errorf("call_indirect host ran against wrong instance: got %#x want 0xA1", r[0])
			}
		})
	}
}

// T2: a JIT-compiled caller invoking a WASM function imported from ANOTHER
// instance (callCross from native code), under the metered baseline. The
// callee must run against its OWN memory and the shared toll must include
// both sides.
func TestJITCrossModuleCall(t *testing.T) {
	calleeSrc := `(module
		(memory (export "mem") 1)
		(func (export "add10") (param i32) (result i32)
			;; reads its own memory[0] as a bias
			(i32.add (i32.add (local.get 0) (i32.const 10))
			         (i32.load8_u (i32.const 0)))))`
	callerSrc := `(module
		(import "b" "add10" (func $a (param i32) (result i32)))
		(memory 1)
		(func (export "run") (param i32) (result i32)
			(call $a (local.get 0))))`
	cb, err := wat.Compile([]byte(calleeSrc))
	if err != nil {
		t.Fatal(err)
	}
	ca, err := wat.Compile([]byte(callerSrc))
	if err != nil {
		t.Fatal(err)
	}
	run := func(jit bool) (uint64, uint64) {
		ts := tollstation.NewSimpleTollStation(1 << 30)
		cfg := config.ModuleConfig{EnableJIT: jit, TollStation: ts}
		l := NewLinker(config.LinkerConfig{})
		modB, err := NewModule(cfg, bytes.NewReader(cb))
		if err != nil {
			t.Fatal(err)
		}
		insB, err := l.Instantiate(modB)
		if err != nil {
			t.Fatal(err)
		}
		insB.Memory.Value[0] = 7 // callee-side bias lives in B's memory
		l.Define("b", modB)
		modA, err := NewModule(cfg, bytes.NewReader(ca))
		if err != nil {
			t.Fatal(err)
		}
		insA, err := l.Instantiate(modA)
		if err != nil {
			t.Fatal(err)
		}
		r, _, err := insA.CallExportedFunc("run", 100)
		if err != nil {
			t.Fatal(err)
		}
		return r[0], ts.GetToll()
	}
	iv, ig := run(false)
	jv, jg := run(true)
	if iv != 117 { // 100 + 10 + 7 from B's memory
		t.Errorf("interp cross-module result %d, want 117", iv)
	}
	if iv != jv || ig != jg {
		t.Errorf("cross-module DIVERGE: interp(v=%d gas=%d) jit(v=%d gas=%d)", iv, ig, jv, jg)
	}
}

// T3: the module-template pattern downstream state machines use: decode a
// module ONCE, then instantiate per call from a shallow copy carrying a
// fresh config. Sequential reuse must give independent, identical runs, the
// template must stay unmutated, and concurrent instantiation+execution from
// the shared template (each call with its own linker, as embedders do) must
// be race-free.
func TestModuleTemplateReuse(t *testing.T) {
	src := `(module
		(memory 1)
		(global $g (mut i32) (i32.const 0))
		(func (export "main") (result i32)
			(local $i i32)
			(block $b (loop $l
				(br_if $b (i32.ge_u (local.get $i) (i32.const 1000)))
				(local.set $i (i32.add (local.get $i) (i32.const 1)))
				(br $l)))
			(global.set $g (i32.add (global.get $g) (i32.const 1)))
			(i32.store (i32.const 0) (i32.add (i32.load (i32.const 0)) (i32.const 1)))
			(global.get $g)))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
	if err != nil {
		t.Fatal(err)
	}
	tmplIdx := tmpl.IndexSpace

	oneCall := func() (uint64, uint64) {
		m := *tmpl // embedder-style shallow copy
		m.ModuleConfig = config.ModuleConfig{EnableJIT: true,
			TollStation: tollstation.NewSimpleTollStation(1 << 30)}
		ins, err := NewInstance(&m, nil)
		if err != nil {
			t.Error(err)
			return 0, 0
		}
		r, _, err := ins.CallExportedFunc("main")
		if err != nil {
			t.Error(err)
			return 0, 0
		}
		return r[0], m.ModuleConfig.TollStation.GetToll()
	}

	// sequential: every call sees FRESH state (global=1, not accumulated)
	// and burns the same toll from its own fresh station
	v1, g1 := oneCall()
	v2, g2 := oneCall()
	if v1 != 1 || v2 != 1 {
		t.Errorf("template reuse leaked state across calls: run1=%d run2=%d, want 1,1", v1, v2)
	}
	if g1 != g2 || g1 == 0 {
		t.Errorf("toll not independent per call: %d vs %d", g1, g2)
	}
	// the shared template must be untouched by instantiations
	if tmpl.IndexSpace != tmplIdx {
		t.Error("template IndexSpace was mutated by a per-call instantiation")
	}
	if tmpl.ModuleConfig.TollStation != nil || tmpl.ModuleConfig.EnableJIT {
		t.Error("template ModuleConfig was mutated by a per-call instantiation")
	}

	// concurrent instantiation + execution from the shared template
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if v, _ := oneCall(); v != 1 {
					t.Errorf("concurrent call got %d, want 1", v)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// A SHARED table: module A exports a table whose element is A's imported
// host function; module B imports that table and call_indirects through it.
// The element was placed by A, so the host call must run bound to A
// (owner semantics), regardless of which instance makes the indirect call.
func TestSharedTableHostBinding(t *testing.T) {
	aSrc := `(module
		(import "env" "mark" (func $h))
		(table (export "tab") 1 funcref)
		(elem (i32.const 0) $h)
		(memory (export "mem") 1))`
	bSrc := `(module
		(import "a" "tab" (table 1 funcref))
		(memory (export "mem") 1)
		(type $v (func))
		(func (export "run")
			(call_indirect (type $v) (i32.const 0))))`
	ab, err := wat.Compile([]byte(aSrc))
	if err != nil {
		t.Skipf("wat table export: %v", err)
	}
	bb, err := wat.Compile([]byte(bSrc))
	if err != nil {
		t.Skipf("wat table import: %v", err)
	}
	for _, jit := range []bool{false, true} {
		l := NewLinker(config.LinkerConfig{})
		l.DefineAdvancedFunc("env", "mark", func(ins *Instance) interface{} {
			return func() {
				if ins.Memory != nil && len(ins.Memory.Value) > 8 {
					ins.Memory.Value[8] = ins.Memory.Value[0] // stamp OWN memory
				}
			}
		})
		cfg := config.ModuleConfig{EnableJIT: jit}
		if jit {
			cfg.TollStation = tollstation.NewSimpleTollStation(1 << 30)
		}
		modA, err := NewModule(cfg, bytes.NewReader(ab))
		if err != nil {
			t.Fatal(err)
		}
		insA, err := l.Instantiate(modA)
		if err != nil {
			t.Fatal(err)
		}
		l.Define("a", modA)
		modB, err := NewModule(cfg, bytes.NewReader(bb))
		if err != nil {
			t.Fatal(err)
		}
		insB, err := l.Instantiate(modB)
		if err != nil {
			t.Fatal(err)
		}
		insA.Memory.Value[0] = 0xAA
		insB.Memory.Value[0] = 0xBB
		if _, _, err := insB.CallExportedFunc("run"); err != nil {
			t.Fatalf("jit=%v: %v", jit, err)
		}
		if insA.Memory.Value[8] != 0xAA {
			t.Errorf("jit=%v: elem owner semantics broken: A.mem[8]=%#x want 0xAA", jit, insA.Memory.Value[8])
		}
		if insB.Memory.Value[8] != 0 {
			t.Errorf("jit=%v: host ran against the CALLER instance: B.mem[8]=%#x want 0", jit, insB.Memory.Value[8])
		}
	}
}

// A host function failing (returned error AND panic) inside nested
// JIT-compiled calls: the trap must surface identically on both engines,
// gas must match, and the instance must stay fully reusable afterwards.
func TestNestedTrapRecovery(t *testing.T) {
	src := `(module
		(import "env" "boom" (func $boom (param i32)))
		(memory 1)
		(func $inner (param $x i32) (result i32)
			(call $boom (local.get $x))
			(i32.add (local.get $x) (i32.const 1)))
		(func $mid (param $x i32) (result i32)
			(call $inner (local.get $x)))
		(func (export "run") (param $x i32) (result i32)
			(call $mid (local.get $x)))
		(func (export "clean") (result i32) (i32.const 41)))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	errBoom := errors.New("boom")
	for _, failMode := range []string{"error", "panic"} {
		run := func(jit bool) (trapped bool, cleanVal uint64, gas uint64) {
			ts := tollstation.NewSimpleTollStation(1 << 30)
			l := NewLinker(config.LinkerConfig{})
			l.DefineAdvancedFunc("env", "boom", func(ins *Instance) interface{} {
				return func(x uint32) error {
					if x == 7 {
						if failMode == "panic" {
							panic(errBoom)
						}
						return errBoom
					}
					return nil
				}
			})
			cfg := config.ModuleConfig{Recover: true, EnableJIT: jit, TollStation: ts}
			mod, err := NewModule(cfg, bytes.NewReader(bin))
			if err != nil {
				t.Fatal(err)
			}
			ins, err := l.Instantiate(mod)
			if err != nil {
				t.Fatal(err)
			}
			_, _, callErr := ins.CallExportedFunc("run", 7) // traps in nested host
			r, _, cleanErr := ins.CallExportedFunc("clean") // must still work
			if cleanErr != nil {
				t.Fatalf("jit=%v %s: instance unusable after trap: %v", jit, failMode, cleanErr)
			}
			return callErr != nil, r[0], ts.GetToll()
		}
		it, iv, ig := run(false)
		jt, jv, jg := run(true)
		if !it || !jt {
			t.Errorf("%s: trap not surfaced (interp=%v jit=%v)", failMode, it, jt)
		}
		if iv != 41 || jv != 41 {
			t.Errorf("%s: clean call after trap wrong (interp=%d jit=%d)", failMode, iv, jv)
		}
		if ig != jg {
			t.Errorf("%s: GAS DIVERGE after trap: interp=%d jit=%d", failMode, ig, jg)
		}
	}
}

// Toll exhaustion at EVERY boundary of a nested call chain: for each cap
// from 1 to full-cost+1, interp and metered JIT must agree on trap/no-trap
// AND on the exact toll consumed. This sweeps the charge points around
// call sites, loop back-edges and host exits.
func TestTollBoundarySweep(t *testing.T) {
	src := `(module
		(import "env" "nop" (func $h))
		(memory 1)
		(func $leaf (param $x i32) (result i32)
			(call $h)
			(i32.add (local.get $x) (i32.const 1)))
		(func (export "run") (result i32)
			(local $i i32) (local $acc i32)
			(block $b (loop $l
				(br_if $b (i32.ge_u (local.get $i) (i32.const 4)))
				(local.set $acc (i32.add (local.get $acc) (call $leaf (local.get $i))))
				(local.set $i (i32.add (local.get $i) (i32.const 1)))
				(br $l)))
			(local.get $acc)))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	run := func(jit bool, cap uint64) (bool, uint64, uint64) {
		ts := tollstation.NewSimpleTollStation(cap)
		l := NewLinker(config.LinkerConfig{})
		l.DefineAdvancedFunc("env", "nop", func(ins *Instance) interface{} { return func() {} })
		cfg := config.ModuleConfig{Recover: true, EnableJIT: jit, TollStation: ts}
		mod, err := NewModule(cfg, bytes.NewReader(bin))
		if err != nil {
			t.Fatal(err)
		}
		ins, err := l.Instantiate(mod)
		if err != nil {
			t.Fatal(err)
		}
		r, _, callErr := ins.CallExportedFunc("run")
		var v uint64
		if callErr == nil && len(r) > 0 {
			v = r[0]
		}
		return callErr != nil, v, ts.GetToll()
	}
	// full cost with an ample cap
	_, fullV, fullCost := run(false, 1<<30)
	for cap := uint64(1); cap <= fullCost+1; cap++ {
		it, iv, ig := run(false, cap)
		jt, jv, jg := run(true, cap)
		if it != jt || iv != jv || ig != jg {
			t.Fatalf("cap=%d DIVERGE: interp(trap=%v v=%d gas=%d) jit(trap=%v v=%d gas=%d)",
				cap, it, iv, ig, jt, jv, jg)
		}
	}
	// sanity: the ample-cap run returns the right value on both engines
	if jt, jv, _ := run(true, 1<<30); jt || jv != fullV {
		t.Fatalf("full run wrong under jit: trap=%v v=%d want %d", jt, jv, fullV)
	}
}

// A mutable global exported by A and imported by B aliases ONE storage
// cell: JIT'd writes in B must be visible to JIT'd reads in A and vice
// versa, matching the interpreter exactly.
func TestCrossInstanceGlobal(t *testing.T) {
	aSrc := `(module
		(global (export "g") (mut i64) (i64.const 5))
		(func (export "read") (result i64) (global.get 0))
		(func (export "bump") (global.set 0 (i64.add (global.get 0) (i64.const 100)))))`
	bSrc := `(module
		(import "a" "g" (global $g (mut i64)))
		(func (export "write") (param i64) (global.set $g (local.get 0)))
		(func (export "read") (result i64) (global.get $g)))`
	ab, err := wat.Compile([]byte(aSrc))
	if err != nil {
		t.Skipf("wat global export: %v", err)
	}
	bb, err := wat.Compile([]byte(bSrc))
	if err != nil {
		t.Skipf("wat global import: %v", err)
	}
	run := func(jit bool) []uint64 {
		cfg := config.ModuleConfig{EnableJIT: jit}
		if jit {
			cfg.TollStation = tollstation.NewSimpleTollStation(1 << 30)
		}
		l := NewLinker(config.LinkerConfig{})
		modA, err := NewModule(cfg, bytes.NewReader(ab))
		if err != nil {
			t.Fatal(err)
		}
		insA, err := l.Instantiate(modA)
		if err != nil {
			t.Fatal(err)
		}
		l.Define("a", modA)
		modB, err := NewModule(cfg, bytes.NewReader(bb))
		if err != nil {
			t.Fatal(err)
		}
		insB, err := l.Instantiate(modB)
		if err != nil {
			t.Fatal(err)
		}
		var out []uint64
		step := func(vals []uint64, err error) {
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, vals...)
		}
		r, _, err := insA.CallExportedFunc("read") // 5
		step(r, err)
		_, _, err = insB.CallExportedFunc("write", 42) // cell = 42
		step(nil, err)
		r, _, err = insA.CallExportedFunc("read") // 42: B's write visible in A
		step(r, err)
		_, _, err = insA.CallExportedFunc("bump") // cell = 142
		step(nil, err)
		r, _, err = insB.CallExportedFunc("read") // 142: A's write visible in B
		step(r, err)
		return out
	}
	iv := run(false)
	jv := run(true)
	want := []uint64{5, 42, 142}
	for i := range want {
		if iv[i] != want[i] || jv[i] != want[i] {
			t.Fatalf("step %d: interp=%d jit=%d want %d", i, iv[i], jv[i], want[i])
		}
	}
}

// A memory exported by A and imported by B is ONE buffer. JIT'd stores in
// B must land in A's view; growing it mid-execution (both via the
// memory.grow opcode and via a HOST function appending pages while an
// outer JIT frame is suspended) must keep every pointer fresh.
func TestSharedMemoryAndGrow(t *testing.T) {
	aSrc := `(module (memory (export "mem") 1 4))`
	bSrc := `(module
		(import "a" "mem" (memory 1 4))
		(import "env" "hostgrow" (func $hg))
		(func (export "poke") (param i32 i32)
			(i32.store (local.get 0) (local.get 1)))
		(func (export "growstore") (result i32)
			;; grow by 1 page via the opcode, then store past the OLD limit
			(drop (memory.grow (i32.const 1)))
			(i32.store (i32.const 70000) (i32.const 777))
			(i32.load (i32.const 70000)))
		(func (export "hostgrowstore") (result i32)
			;; the HOST grows the memory while this JIT frame is suspended
			(call $hg)
			(i32.store (i32.const 135000) (i32.const 888))
			(i32.load (i32.const 135000))))`
	ab, err := wat.Compile([]byte(aSrc))
	if err != nil {
		t.Skipf("wat mem export: %v", err)
	}
	bb, err := wat.Compile([]byte(bSrc))
	if err != nil {
		t.Skipf("wat mem import: %v", err)
	}
	run := func(jit bool) (uint64, uint64, uint32) {
		cfg := config.ModuleConfig{EnableJIT: jit, MaxMemoryPages: 4}
		if jit {
			cfg.TollStation = tollstation.NewSimpleTollStation(1 << 30)
		}
		l := NewLinker(config.LinkerConfig{})
		l.DefineAdvancedFunc("env", "hostgrow", func(ins *Instance) interface{} {
			return func() { // append one page in place (relocates the slice)
				ins.Memory.Value = append(ins.Memory.Value, make([]byte, 65536)...)
			}
		})
		modA, err := NewModule(cfg, bytes.NewReader(ab))
		if err != nil {
			t.Fatal(err)
		}
		insA, err := l.Instantiate(modA)
		if err != nil {
			t.Fatal(err)
		}
		l.Define("a", modA)
		modB, err := NewModule(cfg, bytes.NewReader(bb))
		if err != nil {
			t.Fatal(err)
		}
		insB, err := l.Instantiate(modB)
		if err != nil {
			t.Fatal(err)
		}
		// B stores into the shared buffer; A must see it
		if _, _, err := insB.CallExportedFunc("poke", 100, 123456); err != nil {
			t.Fatal(err)
		}
		shared := binary.LittleEndian.Uint32(insA.Memory.Value[100:])
		r1, _, err := insB.CallExportedFunc("growstore")
		if err != nil {
			t.Fatalf("jit=%v growstore: %v", jit, err)
		}
		r2, _, err := insB.CallExportedFunc("hostgrowstore")
		if err != nil {
			t.Fatalf("jit=%v hostgrowstore: %v", jit, err)
		}
		return r1[0], r2[0], shared
	}
	i1, i2, is := run(false)
	j1, j2, js := run(true)
	if is != 123456 || js != 123456 {
		t.Errorf("shared store invisible to exporter: interp=%d jit=%d", is, js)
	}
	if i1 != 777 || j1 != 777 {
		t.Errorf("store past old limit after memory.grow: interp=%d jit=%d want 777", i1, j1)
	}
	if i2 != 888 || j2 != 888 {
		t.Errorf("store after HOST-grown memory: interp=%d jit=%d want 888", i2, j2)
	}
}

// Interrupt() from another goroutine must stop JIT-compiled code too. The
// JIT polls at call boundaries, so the spinning loop carries a host call.
func TestInterruptJIT(t *testing.T) {
	src := `(module
		(import "env" "tick" (func $tick))
		(func (export "spin")
			(loop $l (call $tick) (br $l))))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	l := NewLinker(config.LinkerConfig{})
	l.DefineAdvancedFunc("env", "tick", func(ins *Instance) interface{} { return func() {} })
	mod, err := NewModule(config.ModuleConfig{EnableJIT: true, Recover: true,
		TollStation: tollstation.NewSimpleTollStation(1 << 22)}, bytes.NewReader(bin))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := l.Instantiate(mod)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		ins.Interrupt()
	}()
	done := make(chan error, 1)
	go func() {
		_, _, err := ins.CallExportedFunc("spin")
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, wasm.ErrInterrupted) {
			t.Fatalf("want ErrInterrupted, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("JIT loop not interruptible within 5s")
	}
	// the instance stays usable: a fresh call runs (and, unresumed by any
	// interrupt, terminates at the toll bound rather than hanging)
	if _, _, err := ins.CallExportedFunc("spin"); err == nil {
		t.Fatal("unbounded spin returned cleanly")
	}
}

// Every wide-integer host operation must consume identical toll and
// produce identical memory under the interpreter and the metered JIT.
func TestWideIntGasParity(t *testing.T) {
	src := `(module
		(import "u256" "add" (func $add (param i32 i32 i32)))
		(import "u256" "mul" (func $mul (param i32 i32 i32)))
		(import "u256" "div_u" (func $div (param i32 i32 i32)))
		(import "u256" "mul_div" (func $md (param i32 i32 i32 i32)))
		(import "u256" "isqrt" (func $sq (param i32 i32)))
		(import "u256" "cmp_u" (func $cmp (param i32 i32) (result i32)))
		(memory 1)
		(func (export "run") (result i32)
			(call $add (i32.const 96) (i32.const 0) (i32.const 32))
			(call $mul (i32.const 128) (i32.const 96) (i32.const 64))
			(call $div (i32.const 160) (i32.const 128) (i32.const 32))
			(call $md (i32.const 192) (i32.const 128) (i32.const 0) (i32.const 64))
			(call $sq (i32.const 224) (i32.const 192))
			(call $cmp (i32.const 224) (i32.const 160))))`
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	run := func(jit bool) (uint64, uint64, []byte) {
		ts := tollstation.NewSimpleTollStation(1 << 30)
		cfg := config.ModuleConfig{EnableJIT: jit, EnableWideInt: true, TollStation: ts}
		mod, err := NewModule(cfg, bytes.NewReader(bin))
		if err != nil {
			t.Fatal(err)
		}
		ins, err := NewInstance(mod, nil)
		if err != nil {
			t.Fatal(err)
		}
		binary.LittleEndian.PutUint64(ins.Memory.Value[0:], 12345678901)
		binary.LittleEndian.PutUint64(ins.Memory.Value[32:], 987654321)
		binary.LittleEndian.PutUint64(ins.Memory.Value[64:], 1000003)
		r, _, err := ins.CallExportedFunc("run")
		if err != nil {
			t.Fatal(err)
		}
		return r[0], ts.GetToll(), append([]byte(nil), ins.Memory.Value[:256]...)
	}
	iv, ig, im := run(false)
	jv, jg, jm := run(true)
	if iv != jv || ig != jg || !bytes.Equal(im, jm) {
		t.Fatalf("wideint parity broken: interp(v=%d gas=%d) jit(v=%d gas=%d) memEq=%v",
			iv, ig, jv, jg, bytes.Equal(im, jm))
	}
}
