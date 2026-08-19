package wasm

import (
	"strings"
	"testing"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/stacks"
	"github.com/c0mm4nd/wasman/types"
	"github.com/c0mm4nd/wasman/utils"
)

// isFloatOp gates DisableFloatPoint: every float opcode class must be
// recognized and every integer neighbor must not.
func TestIsFloatOp(t *testing.T) {
	floats := []byte{
		0x2a, 0x2b, 0x38, 0x39, 0x43, 0x44, // loads / stores / consts
		0x5b, 0x60, 0x66, // comparisons (range ends + middle)
		0x8b, 0x99, 0xa6, // arithmetic
		0xa8, 0xb1, 0xb2, 0xbb, 0xbe, 0xbf, // trunc/convert/reinterpret
	}
	for _, b := range floats {
		if !isFloatOp(expr.OpCode(b)) {
			t.Errorf("opcode %#x should be a float op", b)
		}
	}
	ints := []byte{0x28, 0x36, 0x41, 0x42, 0x45, 0x6a, 0x7c,
		0xac, 0xad, // int extends inside the conversion range
		0xa7, 0xc0, 0xc4}
	for _, b := range ints {
		if isFloatOp(expr.OpCode(b)) {
			t.Errorf("opcode %#x should NOT be a float op", b)
		}
	}
}

func TestHasFloatType(t *testing.T) {
	cases := []struct {
		in, out []types.ValueType
		want    bool
	}{
		{nil, nil, false},
		{[]types.ValueType{types.ValueTypeI32}, []types.ValueType{types.ValueTypeI64}, false},
		{[]types.ValueType{types.ValueTypeF32}, nil, true},
		{[]types.ValueType{types.ValueTypeF64}, nil, true},
		{nil, []types.ValueType{types.ValueTypeF32}, true},
		{nil, []types.ValueType{types.ValueTypeF64}, true},
	}
	for i, c := range cases {
		if got := hasFloatType(&types.FuncType{InputTypes: c.in, ReturnTypes: c.out}); got != c.want {
			t.Errorf("case %d: got %v want %v", i, got, c.want)
		}
	}
}

// validateConstExpr type-checks global/element/data initializers: every
// accept and reject branch, simple and extended.
func TestValidateConstExpr(t *testing.T) {
	imm := &types.GlobalType{ValType: types.ValueTypeI32}
	mut := &types.GlobalType{ValType: types.ValueTypeI32, Mutable: true}
	globals := []*types.GlobalType{imm, mut, nil}

	simple := func(op expr.OpCode, data []byte) *expr.Expression {
		return &expr.Expression{OpCode: op, Data: data}
	}
	ext := func(raw []byte) *expr.Expression {
		return &expr.Expression{Extended: true, Raw: raw}
	}
	i32 := types.ValueTypeI32

	cases := []struct {
		name string
		e    *expr.Expression
		want types.ValueType
		msg  string // "" = must pass
	}{
		{"nil", nil, i32, "missing constant expression"},
		{"i32 ok", simple(expr.OpCodeI32Const, []byte{5}), i32, ""},
		{"i64 ok", simple(expr.OpCodeI64Const, []byte{5}), types.ValueTypeI64, ""},
		{"f32 ok", simple(expr.OpCodeF32Const, []byte{0, 0, 0, 0}), types.ValueTypeF32, ""},
		{"f64 ok", simple(expr.OpCodeF64Const, make([]byte, 8)), types.ValueTypeF64, ""},
		{"global ok", simple(expr.OpCodeGlobalGet, []byte{0}), i32, ""},
		{"global bad index enc", simple(expr.OpCodeGlobalGet, []byte{0x80}), i32, "read global index"},
		{"global unknown", simple(expr.OpCodeGlobalGet, []byte{9}), i32, "unknown global"},
		{"global nil type", simple(expr.OpCodeGlobalGet, []byte{2}), i32, "unknown global type"},
		{"global mutable", simple(expr.OpCodeGlobalGet, []byte{1}), i32, "must not use a mutable global"},
		{"non-const op", simple(expr.OpCodeI32Add, nil), i32, "non-constant opcode"},
		{"type mismatch", simple(expr.OpCodeI64Const, []byte{5}), i32, "type mismatch"},
		// extended forms
		{"ext ok", ext([]byte{0x41, 2, 0x41, 3, 0x6a}), i32, ""},
		{"ext i64 ok", ext([]byte{0x42, 2, 0x42, 3, 0x7c}), types.ValueTypeI64, ""},
		{"ext f32 const", ext([]byte{0x43, 0, 0, 0, 0}), types.ValueTypeF32, ""},
		{"ext f64 const", ext([]byte{0x44, 0, 0, 0, 0, 0, 0, 0, 0}), types.ValueTypeF64, ""},
		{"ext global", ext([]byte{0x23, 0}), i32, ""},
		{"ext i32 trunc", ext([]byte{0x41}), i32, "EOF"},
		{"ext i64 trunc", ext([]byte{0x42}), i32, "EOF"},
		{"ext f32 trunc", ext([]byte{0x43, 0}), i32, "EOF"},
		{"ext f64 trunc", ext([]byte{0x44, 0}), i32, "EOF"},
		{"ext global trunc", ext([]byte{0x23}), i32, "EOF"},
		{"ext global mutable", ext([]byte{0x23, 1}), i32, "mutable"},
		{"ext arith underflow", ext([]byte{0x41, 1, 0x6a}), i32, "type mismatch in arithmetic"},
		{"ext arith type", ext([]byte{0x41, 1, 0x42, 2, 0x6a}), i32, "type mismatch in arithmetic"},
		{"ext bad op", ext([]byte{0x0b}), i32, "opcode"},
		{"ext leftovers", ext([]byte{0x41, 1, 0x41, 2}), i32, "type mismatch"},
		{"ext result mismatch", ext([]byte{0x42, 1}), i32, "type mismatch"},
	}
	for _, c := range cases {
		err := validateConstExpr(c.e, c.want, globals)
		if c.msg == "" {
			if err != nil {
				t.Errorf("%s: unexpected error %v", c.name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: expected error", c.name)
		} else if !strings.Contains(err.Error(), c.msg) {
			t.Errorf("%s: error %q missing %q", c.name, err, c.msg)
		}
	}
}

// brTable's slow path decodes the target list from the raw body when no
// pre-decoded plan exists (directly-constructed frames).
func TestBrTableSlowPath(t *testing.T) {
	mk := func(body []byte, labels int) *Instance {
		ls := stacks.NewLabelStack()
		for i := 0; i < labels; i++ {
			ls.Push(stacks.Label{ContinuationPC: uint64(100 + i)})
		}
		ins := &Instance{
			OperandStack: stacks.NewOperandStack(),
			Active:       &Frame{Func: &wasmFunc{body: body}, LabelStack: ls},
		}
		return ins
	}
	// br_table with targets [1,0] default 0; selector picks target 1 -> label 0
	body := []byte{0x0e, 0x02, 0x01, 0x00, 0x00}
	ins := mk(body, 2)
	ins.OperandStack.Push(0) // selector -> lis[0] = 1
	if err := brTable(ins); err != nil {
		t.Fatal(err)
	}
	// selector beyond the list -> default label
	ins = mk(body, 2)
	ins.OperandStack.Push(9)
	if err := brTable(ins); err != nil {
		t.Fatal(err)
	}
	// decode errors: truncated count, truncated target, truncated default
	for _, bad := range [][]byte{{0x0e}, {0x0e, 0x02, 0x01}, {0x0e, 0x01, 0x00}} {
		ins = mk(bad, 2)
		ins.OperandStack.Push(0)
		if err := brTable(ins); err == nil {
			t.Errorf("body % x: expected decode error", bad)
		}
	}
	// label index out of range
	ins = mk([]byte{0x0e, 0x00, 0x05}, 1)
	ins.OperandStack.Push(0)
	if err := brTable(ins); err != ErrLabelNotFound {
		t.Errorf("want ErrLabelNotFound, got %v", err)
	}
}

// parseNameSection tolerates malformed custom sections (names are debug
// info; a bad section must not fail the module).
func TestParseNameSection(t *testing.T) {
	m := &Module{}
	// well-formed: function-name subsection with two entries
	body := []byte{
		1, 8, // id=1 (function names), size
		2,         // count
		0, 1, 'a', // idx 0 -> "a"
		1, 2, 'b', 'c', // idx 1 -> "bc"
	}
	m.parseNameSection(body)
	if m.FunctionNames[0] != "a" || m.FunctionNames[1] != "bc" {
		t.Fatalf("names = %v", m.FunctionNames)
	}
	// non-function subsections are skipped, then a valid one still parses
	m2 := &Module{}
	pre := []byte{0, 1, 0xff} // id=0 (module name), skipped
	m2.parseNameSection(append(pre, body...))
	if m2.FunctionNames[0] != "a" {
		t.Fatalf("names after skip = %v", m2.FunctionNames)
	}
	// malformed variants must return silently without names
	bads := [][]byte{
		{1},                // truncated size
		{1, 0x80},          // bad size LEB
		{1, 200, 0},        // size beyond remaining
		{1, 1, 0x80},       // bad count
		{1, 2, 1, 0x80},    // bad index
		{1, 3, 1, 0, 0x80}, // bad name
	}
	for _, b := range bads {
		m3 := &Module{}
		m3.parseNameSection(b)
		if len(m3.FunctionNames) != 0 {
			t.Errorf("malformed % x produced names %v", b, m3.FunctionNames)
		}
	}
}

// utils reference keeps the import used when cases above change shape.
var _ = utils.Uint32Ptr
