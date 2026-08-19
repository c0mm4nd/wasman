package wasman_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/wat"
)

// invalid modules the static validator must reject: wat performs no type
// checking, so every case below reaches wasman's validator. Each hits a
// distinct reject branch.
func TestValidatorRejects(t *testing.T) {
	cases := []struct {
		name, src, msg string
	}{
		{"stack underflow", `(module (func (result i32) (i32.add (i32.const 1))))`, ""},
		{"type mismatch add", `(module (func (result i32) (i32.add (i32.const 1) (i64.const 2))))`, ""},
		{"result mismatch", `(module (func (result i32) (i64.const 1)))`, ""},
		{"too many results", `(module (func (i32.const 1)))`, ""},
		{"unknown local", `(module (func (drop (local.get 9))))`, ""},
		{"local.set type", `(module (func (local i32) (local.set 0 (i64.const 1))))`, ""},
		{"unknown global", `(module (func (drop (global.get 3))))`, ""},
		{"set immutable global", `(module (global i32 (i32.const 1)) (func (global.set 0 (i32.const 2))))`, ""},
		{"global.set type", `(module (global (mut i32) (i32.const 1)) (func (global.set 0 (i64.const 2))))`, ""},
		{"unknown func", `(module (func (call 9)))`, ""},
		{"call arg type", `(module (func $f (param i32)) (func (call $f (i64.const 1))))`, ""},
		{"br depth", `(module (func (br 5)))`, ""},
		{"br_if depth", `(module (func (br_if 5 (i32.const 1))))`, ""},
		{"br_table depth", `(module (func (block (br_table 0 9 (i32.const 0)))))`, ""},
		{"br_table arity mix", `(module (func (result i32)
			(block (result i32) (block (i32.const 1) (br_table 0 1 (i32.const 0))))))`, ""},
		{"if without cond", `(module (func (if (then (nop)))))`, ""},
		{"if arm mismatch", `(module (func (result i32)
			(if (result i32) (i32.const 1) (then (i32.const 1)) (else (i64.const 2)))))`, ""},
		{"else missing value", `(module (func (result i32)
			(if (result i32) (i32.const 1) (then (i32.const 1)) (else (nop)))))`, ""},
		{"load without memory", `(module (func (result i32) (i32.load (i32.const 0))))`, ""},
		{"store without memory", `(module (func (i32.store (i32.const 0) (i32.const 1))))`, ""},
		{"memory.size without memory", `(module (func (result i32) (memory.size)))`, ""},
		{"load addr type", `(module (memory 1) (func (result i32) (i32.load (i64.const 0))))`, ""},
		{"select mismatch", `(module (func (result i32)
			(select (i32.const 1) (i64.const 2) (i32.const 1))))`, ""},
		{"call_indirect no table", `(module (type $v (func)) (func (call_indirect (type $v) (i32.const 0))))`, ""},
		{"drop empty", `(module (func (drop)))`, ""},
		{"unbalanced end", `(module (func (block (i32.const 1))))`, ""},
		{"unreachable then bad", `(module (func (unreachable) (i32.add (i64.const 1) (i32.const 2))))`, ""},
		{"memory.grow arg", `(module (memory 1) (func (result i32) (memory.grow (i64.const 1))))`, ""},
		{"tee type", `(module (func (local i64) (drop (local.tee 0 (i32.const 1)))))`, ""},
	}
	for _, c := range cases {
		bin, err := wat.Compile([]byte(c.src))
		if err != nil {
			// wat itself refused: the construct never reaches the validator,
			// which means the case needs rewriting — surface it loudly
			t.Errorf("%s: wat rejected the probe: %v", c.name, err)
			continue
		}
		if _, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin)); err == nil {
			t.Errorf("%s: validator accepted an invalid module", c.name)
		} else if c.msg != "" && !strings.Contains(err.Error(), c.msg) {
			t.Errorf("%s: error %q missing %q", c.name, err, c.msg)
		}
	}
}

// malformed binaries: every section reader must reject truncation and bad
// counts instead of panicking or over-allocating.
func TestMalformedBinaries(t *testing.T) {
	sec := func(id byte, payload ...byte) []byte {
		out := []byte{0, 'a', 's', 'm', 1, 0, 0, 0, id, byte(len(payload))}
		return append(out, payload...)
	}
	cases := []struct {
		name string
		bin  []byte
	}{
		{"type count bad", sec(1, 0x80)},
		{"import count bad", sec(2, 0x80)},
		{"function count bad", sec(3, 0x80)},
		{"table count bad", sec(4, 0x80)},
		{"memory count bad", sec(5, 0x80)},
		{"global count bad", sec(6, 0x80)},
		{"export count bad", sec(7, 0x80)},
		{"start index bad", sec(8, 0x80)},
		{"element count bad", sec(9, 0x80)},
		{"code count bad", sec(10, 0x80)},
		{"data count bad", sec(11, 0x80)},
		{"custom name len bad", sec(0, 0x80)},
		{"custom name past end", sec(0, 0x05, 'a')},
		{"section len past end", []byte{0, 'a', 's', 'm', 1, 0, 0, 0, 1, 0xff}},
		{"unknown section id", sec(13, 0x00)},
		{"truncated header", []byte{0, 'a', 's'}},
		{"bad magic", []byte{1, 2, 3, 4, 1, 0, 0, 0}},
		{"bad version", []byte{0, 'a', 's', 'm', 9, 0, 0, 0}},
	}
	for _, c := range cases {
		if _, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(c.bin)); err == nil {
			t.Errorf("%s: malformed binary accepted", c.name)
		}
	}
	// a benign custom section between real sections is skipped, not fatal
	ok := append(sec(0, 1, 'x'), sec(1, 0)[8:]...)
	if _, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(ok)); err != nil {
		t.Errorf("benign custom section rejected: %v", err)
	}
}
