package wat

import (
	"strings"
	"testing"
)

// wantErr asserts that err is non-nil and mentions substr.
func wantErr(t *testing.T, name string, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected error containing %q, got nil", name, substr)
		return
	}
	if !strings.Contains(err.Error(), substr) {
		t.Errorf("%s: error = %q, want it to contain %q", name, err, substr)
	}
}

// TestCompileErrors drives every malformed-input branch of the lexer, the
// grammar checker and the emitter through the public Compile entry point,
// asserting the exact diagnostic each one produces.
func TestCompileErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		// --- lexer / token errors ---
		{"invalid utf8 source", "\xff", "source is not valid UTF-8"},
		{"unbalanced close", `)`, "unbalanced ')'"},
		{"unbalanced open", `(`, "unbalanced '('"},
		{"lone semicolon", `;`, "unexpected ';'"},
		{"bare string at top level", `"x"`, "unexpected token at module level"},
		{"quote after token", `abc"x"`, "unexpected character after token"},
		{"unterminated string id", `$"abc`, "unterminated string literal"},
		{"char after string id", `$"a"b`, "unexpected character after id"},
		{"string id invalid utf8", `$"\ff"`, "id is not valid UTF-8"},
		{"control char in string", "\"\x01\"", "illegal control character"},
		{"unterminated after escape", "\"a\\t", "unterminated string literal"},
		{"backslash at eof", "\"a\\", "unterminated string literal"},
		{"u escape without brace", `"\ux"`, `malformed \u escape`},
		{"u escape bad digit", `"\u{1z}"`, `malformed \u escape`},
		{"u escape empty", `"\u{}"`, `malformed \u escape`},
		{"bad hex escape", `"\az"`, `malformed escape \a`},
		{"control char after escape", "\"\\t\x01\"", "illegal control character"},

		// --- module-level grammar errors (check.go) ---
		{"atom at module level", `(module foo)`, "unexpected token at module level"},
		{"duplicate type", `(module (type $t (func)) (type $t (func)))`, "duplicate type $t"},
		{"malformed type", `(module (type))`, "malformed type definition"},
		{"duplicate table", `(module (table $t 0 funcref) (table $t 0 funcref))`, "duplicate table $t"},
		{"duplicate memory", `(module (memory $m 1) (memory $m 1))`, "duplicate memory $m"},
		{"duplicate global", `(module (global $g i32 (i32.const 0)) (global $g i32 (i32.const 0)))`, "duplicate global $g"},
		{"duplicate import id", `(module (import "a" "b" (func $f)) (import "a" "c" (func $f)))`, "duplicate func $f"},
		{"multiple start", `(module (func $f) (start $f) (start $f))`, "multiple start sections"},
		{"bad result type", `(module (type (func (result x))))`, "expected value type"},
		{"junk in functype", `(module (type (func (local i32))))`, `unexpected "local" in function type`},
		{"named result", `(module (type (func (result $r i32))))`, "unexpected name"},
		{"malformed named param", `(module (type (func (param $p))))`, "malformed named parameter"},
		{"malformed type use", `(module (type (func)) (func (type 0 0)))`, "malformed type use"},
		{"unknown type id", `(module (func (type $nope)))`, "unknown type $nope"},
		{"type index overflow", `(module (func (type 4294967296)))`, "malformed type index"},
		{"type index not index", `(module (func (type abc)))`, "expected type index"},
		{"func index overflow", `(module (export "e" (func 4294967296)))`, "malformed function index"},
		{"func index keyword", `(module (export "e" (func abc)))`, `expected function index, got "abc"`},
		{"table missing limits", `(module (table))`, "expected limits"},
		{"memory missing limits", `(module (memory))`, "expected limits"},
		{"memory extra token", `(module (memory 1 2 3))`, "unexpected token after memory limits"},
		{"memory min overflow", `(module (memory 4294967296))`, "i32 constant out of range"},
		{"memory max overflow", `(module (memory 1 4294967296))`, "i32 constant out of range"},
		{"limits with sign", `(module (memory +1))`, "i32 constant out of range"},
		{"export name not string", `(module (export abc (func 0)))`, "expected a name string"},
		{"export name bad utf8", `(module (export "\ff" (func 0)))`, "malformed UTF-8 encoding in name"},
		{"malformed inline export", `(module (func (export "a" "b")))`, "malformed inline export"},
		{"inline export bad name", `(module (func (export "\ff")))`, "malformed UTF-8 encoding in name"},
		{"malformed inline import", `(module (func (import "m")))`, "malformed inline import"},
		{"inline import bad module name", `(module (func (import "\ff" "n")))`, "malformed UTF-8 encoding in name"},
		{"inline import bad name", `(module (func (import "m" "\ff")))`, "malformed UTF-8 encoding in name"},
		{"inline import after def", `(module (func) (func (import "m" "n")))`, "import after function definition"},
		{"body on imported func", `(module (func (import "m" "n") nop))`, "unexpected body on imported function"},
		{"duplicate param name", `(module (func (param $x i32) (param $x i32)))`, "duplicate local $x"},
		{"duplicate local name", `(module (func (local $x i32) (local $x i32)))`, "duplicate local $x"},
		{"malformed named local", `(module (func (local $x)))`, "malformed named local"},
		{"bad local type", `(module (func (local abc)))`, "expected value type in local"},
		{"table inline import malformed", `(module (table (import "m") 1 funcref))`, "malformed inline import"},
		{"elems on imported table", `(module (table (import "m" "n") funcref (elem)))`, "inline elements on imported table"},
		{"malformed inline table elems", `(module (table funcref))`, "malformed inline table elements"},
		{"memory inline import malformed", `(module (memory (import "m") 1))`, "malformed inline import"},
		{"data on imported memory", `(module (memory (import "m" "n") (data "x")))`, "inline data on imported memory"},
		{"bad inline data", `(module (memory (data 5)))`, "expected string in inline data"},
		{"global inline import malformed", `(module (global (import "m") i32))`, "malformed inline import"},
		{"missing global type", `(module (global))`, "expected global type"},
		{"init on imported global", `(module (global (import "m" "n") i32 (i32.const 0)))`, "unexpected init on imported global"},
		{"malformed import", `(module (import "m" "n"))`, "malformed import"},
		{"import bad module name", `(module (import "\ff" "n" (func)))`, "malformed UTF-8 encoding in name"},
		{"import bad name", `(module (import "m" "\ff" (func)))`, "malformed UTF-8 encoding in name"},
		{"func import bad typeuse", `(module (import "m" "n" (func (type $nope))))`, "unknown type $nope"},
		{"junk in func import", `(module (import "m" "n" (func nop)))`, "unexpected token in func import"},
		{"table import after def", `(module (table 0 funcref) (import "m" "n" (table 0 funcref)))`, "import after table definition"},
		{"memory import after def", `(module (memory 1) (import "m" "n" (memory 1)))`, "import after memory definition"},
		{"global import after def", `(module (global i32 (i32.const 0)) (import "m" "n" (global i32)))`, "import after global definition"},
		{"malformed global import", `(module (import "m" "n" (global)))`, "malformed global import"},
		{"bad import desc", `(module (import "m" "n" (foo)))`, `unexpected import description "foo"`},
		{"malformed export", `(module (export "a"))`, "malformed export"},
		{"malformed export desc", `(module (export "a" (func)))`, "malformed export description"},
		{"bad export desc kind", `(module (export "a" (foo 0)))`, `unexpected export description "foo"`},
		{"malformed start", `(module (start))`, "malformed start"},
		{"malformed table use", `(module (elem (table 0 0) (i32.const 0)))`, "malformed table use"},
		{"unknown table in use", `(module (elem (table $t) (i32.const 0)))`, "unknown table $t"},
		{"bad legacy table index", `(module (elem 4294967296 (i32.const 0)))`, "malformed table index"},
		{"missing elem offset", `(module (elem))`, "expected offset expression"},
		{"bad elem offset", `(module (elem (bogus)))`, `unknown operator "bogus"`},
		{"elem funcref non-list", `(module (elem funcref 0))`, "expected element expression"},
		{"malformed ref.func", `(module (elem funcref (ref.func)))`, "malformed ref.func"},
		{"unknown ref.func target", `(module (elem funcref (ref.func $f)))`, "unknown function $f"},
		{"unknown elem func", `(module (elem func $f))`, "unknown function $f"},
		{"malformed memory use", `(module (data (memory 0 0) (i32.const 0)))`, "malformed memory use"},
		{"bad data offset", `(module (data (bogus) "x"))`, `unknown operator "bogus"`},
		{"data non-string", `(module (data (i32.const 0) 5))`, "expected string in data segment"},

		// --- instruction grammar errors (instr.go) ---
		{"label wrong kind", `(module (func br abc))`, `expected label, got "abc"`},
		{"number in body", `(module (func 5))`, `unexpected token "5"`},
		{"reserved token in body", `(module (func $ nop))`, `unexpected token "$"`},
		{"plain unknown op", `(module (func bogus))`, `unknown operator "bogus"`},
		{"br missing label", `(module (func br))`, "expected label"},
		{"br_table unknown label", `(module (func br_table $x))`, "unknown label $x"},
		{"br_table no labels", `(module (func br_table))`, "expected label"},
		{"call missing index", `(module (func call))`, "expected function index"},
		{"local.get missing index", `(module (func local.get))`, "expected local index"},
		{"global.get missing index", `(module (func global.get))`, "expected global index"},
		{"call_indirect unknown table", `(module (func call_indirect $t))`, "unknown table $t"},
		{"call_indirect bad typeuse", `(module (func call_indirect (type $nope)))`, "unknown type $nope"},
		{"offset overflow", `(module (memory 1) (func i32.load offset=4294967296))`, "offset out of range"},
		{"align overflow", `(module (memory 1) (func i32.load align=4294967296))`, "align out of range"},
		{"offset after align", `(module (memory 1) (func i32.load align=4 offset=8))`, "unexpected offset="},
		{"int const wrong token", `(module (func i32.const abc))`, `unexpected token "abc"`},
		{"int const missing", `(module (func i32.const))`, "expected integer constant"},
		{"float const missing", `(module (func f32.const))`, "expected float constant"},
		{"float const keyword", `(module (func f32.const abc))`, `unexpected token "abc"`},
		{"folded block bad blocktype", `(module (func (block (type $nope))))`, "unknown type $nope"},
		{"plain end label mismatch", `(module (func block $a nop end $b))`, "mismatching label"},
		{"plain block bad blocktype", `(module (func block (type $nope) end))`, "unknown type $nope"},
		{"unclosed block", `(module (func block nop))`, "unclosed block: expected end"},
		{"else outside if", `(module (func block else end))`, "unexpected else"},
		{"else label mismatch", `(module (func if $a nop else $b end))`, "mismatching label"},
		{"bad instr inside block", `(module (func block bogus end))`, `unknown operator "bogus"`},
		{"empty folded expr", `(module (func ()))`, "empty expression"},
		{"folded non-operator", `(module (func (5)))`, "expected an operator"},
		{"folded if bad blocktype", `(module (func (if (type $nope) (then))))`, "unknown type $nope"},
		{"folded if bad condition", `(module (func (if (bogus) (then))))`, `unknown operator "bogus"`},
		{"folded if missing then", `(module (func (if)))`, "expected (then ...) in folded if"},
		{"folded if bad then body", `(module (func (if (then bogus))))`, `unknown operator "bogus"`},
		{"folded if not else", `(module (func (if (then) (nop))))`, "unexpected token in folded if"},
		{"folded if bad else body", `(module (func (if (then) (else bogus))))`, `unknown operator "bogus"`},
		{"folded if trailing item", `(module (func (if (then) (else) (nop))))`, "unexpected token after folded if"},
		{"stray then", `(module (func (then)))`, `unexpected "then"`},

		// --- literal errors surfaced through instructions ---
		{"float as int const", `(module (func i32.const 1.5))`, "not an integer literal"},
		{"i32 negative overflow", `(module (func i32.const -2147483649))`, "constant out of range"},
		{"nan payload too wide", `(module (func f32.const nan:0x11111111111111111))`, "constant out of range"},
		{"reserved dec float token", `(module (func i32.const 1.x))`, `unexpected token "1.x"`},
		{"reserved exponent token", `(module (func i32.const 1e+))`, `unexpected token "1e+"`},
		{"reserved hex float token", `(module (func i32.const 0x1.z))`, `unexpected token "0x1.z"`},

		// --- emit-time errors: forms the grammar checker leaves for the
		// binary validator (out-of-range numeric type indices, named params
		// on imported functions) ---
		{"func unknown numeric type", `(module (func (type 5)))`, "unknown type 5"},
		{"import unknown numeric type", `(module (import "m" "n" (func (type 5))))`, "unknown type 5"},
		{"named param on imported func", `(module (func (import "m" "n") (param $x i32)))`, "unexpected name"},
		{"named param with typeuse on import", `(module (type (func (param i32))) (func (import "m" "n") (type 0) (param $x i32)))`, "unexpected name"},
		{"plain block unknown type", `(module (func block (type 5) end))`, "unknown type 5"},
		{"call_indirect unknown type", `(module (func call_indirect (type 5)))`, "unknown type 5"},
		{"bad instr in plain block", `(module (func block call_indirect (type 5) end))`, "unknown type 5"},
		{"folded block unknown type", `(module (func (block (type 5))))`, "unknown type 5"},
		{"folded block bad body", `(module (func (block (call_indirect (type 5)))))`, "unknown type 5"},
		{"folded if bad cond emit", `(module (func (if (call_indirect (type 5)) (then))))`, "unknown type 5"},
		{"folded if unknown type", `(module (func (if (type 5) (i32.const 1) (then))))`, "unknown type 5"},
		{"folded if bad then emit", `(module (func (if (i32.const 1) (then (call_indirect (type 5))))))`, "unknown type 5"},
		{"folded if bad else emit", `(module (func (if (i32.const 1) (then) (else (call_indirect (type 5))))))`, "unknown type 5"},
		{"folded operand emit error", `(module (func (drop (call_indirect (type 5)))))`, "unknown type 5"},
		{"global init emit error", `(module (global i32 (block (type 5))))`, "unknown type 5"},
		{"elem offset emit error", `(module (table 1 funcref) (elem (block (type 5)) func))`, "unknown type 5"},
		{"data offset emit error", `(module (memory 1) (data (block (type 5)) "x"))`, "unknown type 5"},
		{"data non-zero memory", `(module (memory 1) (data (memory 1) (i32.const 0) "x"))`, "unsupported data form (non-zero memory index)"},
	}

	for _, c := range cases {
		_, err := Compile([]byte(c.src))
		wantErr(t, c.name, err, c.want)
	}
}

func TestValidateTextParseError(t *testing.T) {
	wantErr(t, "unbalanced open", ValidateText([]byte(`(`)), "unbalanced '('")
}

func TestScriptModules(t *testing.T) {
	if _, err := ScriptModules([]byte("\xff")); err == nil {
		t.Error("invalid UTF-8 script accepted")
	}
	if _, err := ScriptModules([]byte(`(`)); err == nil {
		t.Error("unbalanced script accepted")
	}

	// binary/quote payloads and non-module commands are skipped; only the
	// one plain module counts
	checked, err := ScriptModules([]byte(`
	(module (func))
	(module binary "\00asm")
	(module quote "(module)")
	(assert_return (invoke "x"))`))
	if err != nil {
		t.Fatalf("valid script rejected: %v", err)
	}
	if checked != 1 {
		t.Fatalf("checked = %d, want 1", checked)
	}

	checked, err = ScriptModules([]byte(`(module (funk))`))
	wantErr(t, "invalid module in script", err, "valid module rejected")
	if checked != 0 {
		t.Fatalf("checked = %d, want 0", checked)
	}
}

// mustParse builds an S-expression forest for unit tests of internals.
func mustParse(t *testing.T, src string) []node {
	t.Helper()
	toks, err := lex([]byte(src))
	if err != nil {
		t.Fatalf("lex(%q): %v", src, err)
	}
	nodes, err := parseSExprs(toks)
	if err != nil {
		t.Fatalf("parse(%q): %v", src, err)
	}
	return nodes
}

func TestLiteralEdgeCases(t *testing.T) {
	for _, s := range []string{"1.x", "1e+", "0x1.z"} {
		if isNumToken(s) {
			t.Errorf("isNumToken(%q) = true, want false", s)
		}
	}
	wantErr(t, "malformed nan payload", checkFloat("nan:0xzz", 32), "malformed nan payload")
	wantErr(t, "not a float literal", checkFloat("1.x", 32), "not a float literal")
	_, err := parseUint("-1", 32)
	wantErr(t, "signed uint", err, "unexpected sign")
	_, err = parseFloatBits("abc", 32)
	wantErr(t, "parseFloatBits bad literal", err, "not a float literal")
}

// TestCheckerInternalDefault covers the defensive default arm of
// checkImmediates that a well-formed instrTable can never reach.
func TestCheckerInternalDefault(t *testing.T) {
	c := &checker{}
	_, err := c.checkImmediates(nil, 0, instrInfo{imm: immKind(99)}, &instrCtx{})
	wantErr(t, "checkImmediates default", err, "internal: unhandled immediate kind")
}

// TestCompilerCollectErrors unit-tests error branches of the collect pass
// that the grammar checker normally rejects first.
func TestCompilerCollectErrors(t *testing.T) {
	newC := func() *compiler { return newCompiler() }

	c := newC()
	_, _, _, err := c.resolveTypeuse(mustParse(t, `(type 99999999999999999999)`), false)
	wantErr(t, "resolveTypeuse overflow", err, "constant out of range")

	wantErr(t, "collect named result",
		newC().collect(mustParse(t, `(type (func (result $r i32)))`)), "unexpected name")

	c = newC()
	imp := mustParse(t, `(import "m" "n" (table abc funcref))`)
	wantErr(t, "import bad table limits", c.collectImport(imp[0].list[1:]), "unsupported limits form")

	c = newC()
	imp = mustParse(t, `(import "m" "n" (memory abc))`)
	wantErr(t, "import bad memory limits", c.collectImport(imp[0].list[1:]), "unsupported limits form")

	wantErr(t, "collectTable bad limits",
		newC().collectTable(mustParse(t, `abc funcref`)), "unsupported limits form")
	wantErr(t, "collectMemory bad limits",
		newC().collectMemory(mustParse(t, `abc`)), "unsupported limits form")
	wantErr(t, "collectData unknown memory",
		newC().collectData(mustParse(t, `(memory $nope) (i32.const 0) "x"`)), "unknown index $nope")
	wantErr(t, "collectData non-string",
		newC().collectData(mustParse(t, `(i32.const 0) 5`)), "unsupported data form")

	_, err = encodeLimits(mustParse(t, `4294967296`))
	wantErr(t, "encodeLimits min overflow", err, "constant out of range")
	_, err = encodeLimits(mustParse(t, `1 4294967296`))
	wantErr(t, "encodeLimits max overflow", err, "constant out of range")
}

// TestEmitterErrors unit-tests emit-time error branches that the checker
// normally rejects first (the emitter re-validates defensively).
func TestEmitterErrors(t *testing.T) {
	newEm := func() *emitter {
		return &emitter{c: newCompiler(), def: &funcDef{localNames: map[string]int{}}}
	}

	_, err := newEm().emitOp("bogus", instrInfo{}, nil, 0)
	wantErr(t, "no opcode", err, `no opcode for "bogus"`)

	_, err = newEm().emitOp("nop", instrInfo{imm: immKind(99)}, nil, 0)
	wantErr(t, "emitOp default", err, "internal: unhandled immediate kind")

	_, err = newEm().scanImmediates(nil, 0, instrInfo{imm: immKind(99)})
	wantErr(t, "scanImmediates default", err, "internal: unhandled immediate kind")

	ops := []struct {
		name, op, items, want string
	}{
		{"br unknown label", "br", `$x`, "unknown label $x"},
		{"br_table unknown label", "br_table", `$x`, "unknown label $x"},
		{"call unknown func", "call", `$f`, "unknown index $f"},
		{"local index overflow", "local.get", `4294967296`, "constant out of range"},
		{"unknown local", "local.get", `$x`, "unknown local $x"},
		{"unknown global", "global.get", `$g`, "unknown index $g"},
		{"call_indirect unknown table", "call_indirect", `$t`, "unknown index $t"},
		{"memarg offset overflow", "i32.load", `offset=4294967296`, "constant out of range"},
		{"memarg align overflow", "i32.load", `align=4294967296`, "constant out of range"},
		{"i32.const bad literal", "i32.const", `abc`, "not an integer literal"},
		{"i64.const bad literal", "i64.const", `abc`, "not an integer literal"},
		{"f32.const bad literal", "f32.const", `abc`, "not a float literal"},
		{"f64.const bad literal", "f64.const", `abc`, "not a float literal"},
	}
	for _, c := range ops {
		_, err := newEm().emitOp(c.op, instrTable[c.op], mustParse(t, c.items), 0)
		wantErr(t, c.name, err, c.want)
	}

	_, err = newEm().emitOneInstr(mustParse(t, `bogus`), 0)
	wantErr(t, "plain unknown op", err, `unknown operator "bogus"`)

	folded := mustParse(t, `(bogus)`)
	wantErr(t, "folded unknown op", newEm().emitFolded(&folded[0]), `unknown operator "bogus"`)

	// an instrTable entry with a corrupt immediate kind must surface the
	// scanImmediates diagnostic from emitFolded as well
	instrTable["zzz.test"] = instrInfo{imm: immKind(99)}
	defer delete(instrTable, "zzz.test")
	folded = mustParse(t, `(zzz.test)`)
	wantErr(t, "folded scanImmediates error", newEm().emitFolded(&folded[0]), "internal: unhandled immediate kind")
}

// TestEmitResolutionErrors unit-tests emit()'s index-resolution error
// branches for exports, start, and element segments; through Compile the
// checker guarantees these ids exist, so the branches are defensive.
func TestEmitResolutionErrors(t *testing.T) {
	badIdx := func() *node { return &node{atom: &token{kind: tokID, text: "$nope"}} }

	c := newCompiler()
	c.exports = append(c.exports, exportEntry{name: []byte("x"), kind: 0x00, idx: badIdx()})
	_, err := c.emit()
	wantErr(t, "export unknown index", err, "unknown index $nope")

	c = newCompiler()
	c.start = badIdx()
	_, err = c.emit()
	wantErr(t, "start unknown index", err, "unknown index $nope")

	c = newCompiler()
	c.elems = append(c.elems, elemDef{table: badIdx(), offset: []node{*numConstNode(0)}})
	_, err = c.emit()
	wantErr(t, "elem unknown table", err, "unknown index $nope")

	c = newCompiler()
	c.elems = append(c.elems, elemDef{offset: []node{*numConstNode(0)}, funcs: []node{*badIdx()}})
	_, err = c.emit()
	wantErr(t, "elem unknown func", err, "unknown index $nope")
}

// TestUnreachablePanics pins the invariant that the byte-mapping helpers
// only ever see checker-approved names.
func TestUnreachablePanics(t *testing.T) {
	for name, fn := range map[string]func(){
		"valTypeByte":    func() { valTypeByte("bogus") },
		"exportKindByte": func() { exportKindByte("bogus") },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s(bogus): expected panic", name)
				}
			}()
			fn()
		}()
	}
}
