package expr_test

import (
	"bytes"
	"reflect"
	"strconv"
	"testing"

	"github.com/c0mm4nd/wasman/expr"
)

func TestReadExpr(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		for _, b := range [][]byte{
			{}, {0xaa}, {0x41, 0x1}, {0x41, 0x01, 0x41}, // all invalid
		} {
			_, err := expr.ReadExpression(bytes.NewReader(b))
			t.Log(err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		for _, c := range []struct {
			bytes []byte
			exp   *expr.Expression
		}{
			{
				bytes: []byte{0x42, 0x01, 0x0b},
				exp:   &expr.Expression{OpCode: expr.OpCodeI64Const, Data: []byte{0x01}},
			},
			{
				bytes: []byte{0x43, 0x40, 0xe1, 0x47, 0x40, 0x0b},
				exp:   &expr.Expression{OpCode: expr.OpCodeF32Const, Data: []byte{0x40, 0xe1, 0x47, 0x40}},
			},
			{
				bytes: []byte{0x23, 0x01, 0x0b},
				exp:   &expr.Expression{OpCode: expr.OpCodeGlobalGet, Data: []byte{0x01}},
			},
		} {
			actual, err := expr.ReadExpression(bytes.NewReader(c.bytes))
			if err != nil {
				t.Fail()
			}
			if !reflect.DeepEqual(c.exp, actual) {
				t.Fail()
			}
		}
	})
}

func TestReadExpr_extended(t *testing.T) {
	// extended constant expression: i32.const 1, i32.const 2, i32.add, end
	buf := []byte{0x41, 0x01, 0x41, 0x02, 0x6a, 0x0b}
	exp := &expr.Expression{
		OpCode:   expr.OpCodeI32Const,
		Data:     []byte{0x01},
		Extended: true,
		Raw:      []byte{0x41, 0x01, 0x41, 0x02, 0x6a},
	}
	actual, err := expr.ReadExpression(bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(exp, actual) {
		t.Errorf("got %#v, want %#v", actual, exp)
	}
}

func TestReadExpr_f64Const(t *testing.T) {
	// f64.const 1.0 (IEEE 754 little-endian), end
	buf := []byte{0x44, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0, 0x3f, 0x0b}
	exp := &expr.Expression{
		OpCode: expr.OpCodeF64Const,
		Data:   []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0, 0x3f},
	}
	actual, err := expr.ReadExpression(bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(exp, actual) {
		t.Errorf("got %#v, want %#v", actual, exp)
	}
}

func TestReadExpr_errors(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   string
	}{
		// truncated before the first opcode
		{bytes: []byte{}, exp: "read opcode: EOF"},
		// 0xaa is not a constant-expression opcode
		{bytes: []byte{0xaa}, exp: "invalid byte for opcodes.OpCode: 0xaa"},
		// i32.const without its immediate
		{bytes: []byte{0x41}, exp: "read value: readByte failed: EOF"},
		// f32.const with a truncated immediate
		{bytes: []byte{0x43, 0x00}, exp: "read value: unexpected EOF"},
		// f64.const without its immediate
		{bytes: []byte{0x44}, exp: "read value: EOF"},
		// global.get without its index
		{bytes: []byte{0x23}, exp: "read value: EOF"},
		// bare end opcode: no instruction at all
		{bytes: []byte{0x0b}, exp: "empty constant expression"},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := expr.ReadExpression(bytes.NewReader(c.bytes))
			if err == nil || err.Error() != c.exp {
				t.Errorf("got %v, want %q", err, c.exp)
			}
		})
	}
}

func TestGetOpCodeName(t *testing.T) {
	// first call initializes the name table lazily
	for _, c := range []struct {
		op  expr.OpCode
		exp string
	}{
		{op: expr.OpCodeUnreachable, exp: "Unreachable"},
		{op: expr.OpCodeEnd, exp: "End"},
		{op: expr.OpCodeI32Const, exp: "I32Const"},
		{op: expr.OpCodeI32Add, exp: "I32Add"},
		{op: expr.OpCodeMiscPrefix, exp: "MiscPrefix"},
		{op: expr.OpCode(0xff), exp: ""}, // unassigned opcode has no name
	} {
		if actual := expr.GetOpCodeName(c.op); actual != c.exp {
			t.Errorf("GetOpCodeName(%#x) = %q, want %q", byte(c.op), actual, c.exp)
		}
	}
}
