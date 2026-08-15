package wat

import "testing"

// FuzzValidateText: arbitrary text must be either accepted or rejected
// with an error — never a panic. Downstream embedders run this reader on
// adversarial contract source.
func FuzzValidateTextLiterals(f *testing.F) {
	// literal/token edge cases Codex flagged: numeric overflow, lone signs,
	// huge exponents, malformed hex, unterminated strings/comments,
	// deep nesting.
	for _, s := range []string{
		`(module (func (result i32) i32.const 999999999999999999999999))`,
		`(module (func (result i32) i32.const 0x))`,
		`(module (func (result i32) i32.const +))`,
		`(module (func (result f64) f64.const 1e999999))`,
		`(module (func (result f64) f64.const 0x1.fp+))`,
		`(module (data "\)`,
		`(module (; unterminated`,
		`(module (func (result i64) i64.const -0x8000000000000000))`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		_ = ValidateText([]byte(src))
	})
}

// FuzzDeepNesting checks that deeply nested S-expressions error rather than
// crash (a fatal stack overflow is not a recoverable error).
func FuzzDeepNesting(f *testing.F) {
	f.Add(200)
	f.Fuzz(func(t *testing.T, depth int) {
		if depth < 0 || depth > 100000 {
			return
		}
		src := make([]byte, 0, depth*2+16)
		src = append(src, []byte("(module ")...)
		for i := 0; i < depth; i++ {
			src = append(src, '(')
		}
		for i := 0; i < depth; i++ {
			src = append(src, ')')
		}
		src = append(src, ')')
		_ = ValidateText(src) // must not fatally overflow the goroutine stack
	})
}

func FuzzValidateText(f *testing.F) {
	f.Add("(module)")
	f.Add(`(module (func (export "f") (result i32) i32.const 1))`)
	f.Add(`(import "m" "n" ())`)
	f.Add(`(module (import "m" "n" ()))`)
	f.Add(`(module (memory 1) (data (i32.const 0) "abc"))`)
	f.Add(`(module (table 1 funcref) (elem (i32.const 0) $f) (func $f))`)
	f.Add(`(module (global (mut i32) (i32.const 0)))`)
	f.Add(`(module (func (block (result i32) (i32.const 1))))`)
	f.Add("(module (func \x00))")
	f.Fuzz(func(t *testing.T, src string) {
		_ = ValidateText([]byte(src)) // must not panic
	})
}
