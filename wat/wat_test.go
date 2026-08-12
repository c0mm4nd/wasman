package wat

import "testing"

func TestValidateText(t *testing.T) {
	cases := []struct {
		name   string
		src    string
		accept bool
	}{
		// --- well-formed modules that must be accepted ---
		{"empty module", `(module)`, true},
		{"bare fields", `(func) (memory 1)`, true},
		{"multiple inline exports", `(module (func (export "a") (export "b")))`, true},
		{"inline import", `(module (import "m" "n" (func)))`, true},
		{"folded nested", `(module (func (result i32) (i32.add (i32.const 1) (i32.mul (i32.const 2) (i32.const 3)))))`, true},
		{"plain block/loop/if", `(module (func (block (loop (if (i32.const 1) (then (nop)) (else (nop)))))))`, true},
		{"blocktype by index", `(module (type $s (func (param i32) (result i32))) (func (i32.const 0) (block (type $s) (drop) (i32.const 0))))`, true},
		{"br_table many labels", `(module (func (block (block (block (i32.const 0) (br_table 0 1 2 0))))))`, true},
		{"hex float const", `(module (global f64 (f64.const 0x1.921fb6p+2)))`, true},
		{"nan payload const", `(module (global f32 (f32.const nan:0x400000)))`, true},
		{"unicode name escape", `(module (func (export "\u{2764}")))`, true},
		{"multi-string data", `(module (memory 1) (data (i32.const 0) "a" "b" "c"))`, true},
		{"passive elem func", `(module (func) (elem func 0))`, true},
		{"passive elem funcref", `(module (func) (elem funcref (ref.func 0)))`, true},
		{"declarative elem", `(module (func) (elem declare func 0))`, true},
		{"active elem legacy idx", `(module (func) (table 1 funcref) (elem 0 (i32.const 0) 0))`, true},
		{"mut global", `(module (global (mut i32) (i32.const 0)))`, true},
		{"table/memory import", `(module (import "" "" (table 0 funcref)) (import "" "" (memory 0)))`, true},
		{"sign extension ops", `(module (func (result i32) (i32.const 0) (i32.extend8_s)))`, true},
		{"select/memory ops", `(module (memory 1) (func (result i32) (memory.size) (memory.grow) (drop) (i32.const 0) (i32.const 1) (i32.const 0) (select)))`, true},
		{"numeric type match", `(module (type (func (result i32))) (func (type 0) (result i32) (unreachable)))`, true},

		// --- malformed modules that must be rejected ---
		{"unterminated string", `(module (func (export "abc)))`, false},
		{"unterminated comment", `(module (; nested (; x`, false},
		{"string adjacency", `(module (func (export "a"$x)))`, false},
		{"surrogate escape", `(module (func (export "\u{D800}")))`, false},
		{"unicode out of range", `(module (func (export "\u{110000}")))`, false},
		{"duplicate func id", `(module (func $f) (func $f))`, false},
		{"unknown local", `(module (func (local.get $x)))`, false},
		{"unknown label", `(module (func (br $l)))`, false},
		{"param after result", `(module (type (func (result i32) (param i32))))`, false},
		{"result before param typeuse", `(module (func (result i32) (param i32) (i32.const 0)))`, false},
		{"align not power of two", `(module (memory 1) (func (i32.const 0) (i32.load align=3) (drop)))`, false},
		{"align over natural", `(module (memory 1) (func (i32.const 0) (i32.load8_u align=2) (drop)))`, false},
		{"i32 const overflow", `(module (func (i32.const 4294967296) (drop)))`, false},
		{"nan payload zero", `(module (global f32 (f32.const nan:0x0)))`, false},
		{"f32 nan payload overflow", `(module (global f32 (f32.const nan:0x800000)))`, false},
		{"underscore misplaced", `(module (global i32 (i32.const _100)))`, false},
		{"mismatched end label", `(module (func (block $a (nop) end $b)))`, false},
		{"import after def", `(module (func) (import "" "" (func)))`, false},
		{"numeric type mismatch", `(module (type (func (result i32))) (func (type 0) (result i64) (unreachable)))`, false},
		{"named type mismatch", `(module (type $s (func (result i32))) (func (type $s) (result i64) (unreachable)))`, false},
	}

	for _, c := range cases {
		err := ValidateText([]byte(c.src))
		if c.accept && err != nil {
			t.Errorf("%s: rejected a valid module: %v", c.name, err)
		}
		if !c.accept && err == nil {
			t.Errorf("%s: accepted a malformed module", c.name)
		}
	}
}
