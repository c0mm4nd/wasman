package wasman_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/wat"
)

// linker definition surface: tables, memories, globals of every supported
// kind, module maps, shadowing rules and signature rejects.
func TestLinkerDefinitions(t *testing.T) {
	l := wasman.NewLinker(config.LinkerConfig{})
	if err := l.DefineTable("env", "tab", 4); err != nil {
		t.Fatal(err)
	}
	if err := l.DefineMemory("env", "mem", make([]byte, 65536)); err != nil {
		t.Fatal(err)
	}
	// a module using all three imported kinds
	src := `(module
		(import "env" "tab" (table 4 funcref))
		(import "env" "mem" (memory 1))
		(import "env" "gi32" (global (mut i32)))
		(import "env" "gi64" (global (mut i64)))
		(import "env" "gf32" (global (mut f32)))
		(import "env" "gf64" (global (mut f64)))
		(func (export "peek") (result i32) (i32.load (i32.const 0))))`
	for name, v := range map[string]interface{}{
		"gi32": int32(1), "gi64": int64(2), "gf32": float32(1.5), "gf64": float64(2.5),
	} {
		if err := l.DefineGlobal("env", name, v); err != nil {
			t.Fatalf("DefineGlobal %s: %v", name, err)
		}
	}
	bin, err := wat.Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := l.Instantiate(mod)
	if err != nil {
		t.Fatal(err)
	}
	ins.Memory.Value[0] = 42
	if r, _, err := ins.CallExportedFunc("peek"); err != nil || r[0] != 42 {
		t.Fatalf("peek: %v %v", r, err)
	}

	// unsupported global kind
	if err := l.DefineGlobal("env", "bad", "str"); err == nil {
		t.Fatal("string global accepted")
	}
	// unsupported host signature kinds
	if err := l.DefineFunc("env", "badf", func(s string) {}); err == nil {
		t.Fatal("string param accepted")
	}
	if err := l.DefineFunc("env", "badr", func() string { return "" }); err == nil {
		t.Fatal("string return accepted")
	}
	if err := l.DefineFunc("env", "okf", func(a int32) int32 { return a }); err != nil {
		t.Fatal(err)
	}

	// DisableShadowing rejects redefinition of every kind
	ns := wasman.NewLinker(config.LinkerConfig{DisableShadowing: true})
	for i, def := range []func() error{
		func() error { return ns.DefineFunc("m", "x", func() {}) },
		func() error {
			return ns.DefineAdvancedFunc("m", "y", func(ins *wasman.Instance) interface{} { return func() {} })
		},
		func() error { return ns.DefineGlobal("m", "z", int64(1)) },
		func() error { return ns.DefineTable("m", "t", 1) },
		func() error { return ns.DefineMemory("m", "mm", nil) },
	} {
		if err := def(); err != nil {
			t.Fatalf("def %d: %v", i, err)
		}
		if err := def(); err == nil {
			t.Fatalf("def %d: shadowing allowed", i)
		}
	}

	// NewLinkerWithModuleMap seeds the module set
	seeded := wasman.NewLinkerWithModuleMap(config.LinkerConfig{}, l.Modules)
	if _, ok := seeded.Modules["env"]; !ok {
		t.Fatal("module map not carried over")
	}

	// NewModule propagates reader errors
	if _, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(nil)); err == nil {
		t.Fatal("empty reader accepted")
	}
}

// every wideint host op traps on an out-of-bounds pointer in EVERY pointer
// argument position — the per-argument bounds checks are consensus surface.
func TestWideIntOOBEveryArg(t *testing.T) {
	type opSig struct {
		ns, op string
		args   int // pointer args
	}
	var ops []opSig
	for _, ns := range []string{"u128", "u256"} {
		for _, op := range []string{"add", "sub", "mul", "div_u", "rem_u", "div_s", "rem_s", "and", "or", "xor"} {
			ops = append(ops, opSig{ns, op, 3})
		}
		// shifts: (dst, a, bits) — only two pointer args
		for _, op := range []string{"shl", "shr_u", "shr_s"} {
			ops = append(ops, opSig{ns, op, 2})
		}
		ops = append(ops, opSig{ns, "not", 2}, opSig{ns, "cmp_u", 2}, opSig{ns, "cmp_s", 2}, opSig{ns, "iszero", 1})
	}
	ops = append(ops, opSig{"u256", "mul_div", 4}, opSig{"u256", "isqrt", 2})

	var b bytes.Buffer
	b.WriteString("(module\n")
	for _, o := range ops {
		params := strings.Repeat(" i32", o.args)
		if o.args != 3 && (o.op == "shl" || o.op == "shr_u" || o.op == "shr_s") {
			params = " i32 i32 i32" // (dst, a, bits)
		}
		res := ""
		if strings.HasPrefix(o.op, "cmp") || o.op == "iszero" {
			res = " (result i32)"
		}
		fmt.Fprintf(&b, "(import %q %q (func $%s_%s (param%s)%s))\n", o.ns, o.op, o.ns, o.op, params, res)
	}
	b.WriteString("(memory 1)\n")
	// one export per (op, bad-arg position): all other pointers valid
	for i, o := range ops {
		n := o.args
		if o.op == "shl" || o.op == "shr_u" || o.op == "shr_s" {
			n = 2 // the bits argument is a plain integer, not a pointer
		}
		for pos := 0; pos < n; pos++ {
			fmt.Fprintf(&b, "(func (export \"e%d_%d\")", i, pos)
			drop := strings.HasPrefix(o.op, "cmp") || o.op == "iszero"
			if drop {
				b.WriteString(" (drop")
			}
			fmt.Fprintf(&b, " (call $%s_%s", o.ns, o.op)
			total := o.args
			if o.op == "shl" || o.op == "shr_u" || o.op == "shr_s" {
				total = 3
			}
			for a := 0; a < total; a++ {
				v := 64 * (a + 1) // valid, well-separated pointers
				if a == pos {
					v = 70000 // out of bounds
				}
				if a == 2 && (o.op == "shl" || o.op == "shr_u" || o.op == "shr_s") {
					v = 1 // shift amount, not a pointer
				}
				fmt.Fprintf(&b, " (i32.const %d)", v)
			}
			b.WriteString(")")
			if drop {
				b.WriteString(")")
			}
			b.WriteString(")\n")
		}
	}
	b.WriteString(")")

	bin, err := wat.Compile(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	mod, err := wasman.NewModule(config.ModuleConfig{EnableWideInt: true, Recover: true}, bytes.NewReader(bin))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := wasman.NewInstance(mod, nil)
	if err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint64(ins.Memory.Value[64:], 3) // keep divisors nonzero
	for i, o := range ops {
		n := o.args
		if o.op == "shl" || o.op == "shr_u" || o.op == "shr_s" {
			n = 2
		}
		for pos := 0; pos < n; pos++ {
			if _, _, err := ins.CallExportedFunc(fmt.Sprintf("e%d_%d", i, pos)); err == nil {
				t.Errorf("%s.%s arg %d out of bounds: no trap", o.ns, o.op, pos)
			}
		}
	}
}
