package segments_test

import (
	"bytes"
	"reflect"
	"strconv"
	"testing"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/segments"
)

func TestReadElementSegment(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   *segments.ElemSegment
	}{
		{
			// flag 0: active, table 0, offset expr, funcidx vec
			bytes: []byte{0x00, 0x41, 0x1, 0x0b, 0x02, 0x05, 0x07},
			exp: &segments.ElemSegment{
				TableIndex: 0,
				OffsetExpr: &expr.Expression{
					OpCode: expr.OpCodeI32Const,
					Data:   []byte{0x01},
				},
				Init: []uint32{5, 7},
			},
		},
		{
			// flag 2: active, explicit tableidx 3, offset expr, elemkind, funcidx vec
			bytes: []byte{0x02, 0x03, 0x41, 0x04, 0x0b, 0x00, 0x01, 0x0a},
			exp: &segments.ElemSegment{
				TableIndex: 3,
				OffsetExpr: &expr.Expression{
					OpCode: expr.OpCodeI32Const,
					Data:   []byte{0x04},
				},
				Init: []uint32{10},
			},
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual, err := segments.ReadElemSegment(bytes.NewReader(c.bytes))
			if err != nil {
				t.Fail()
			}
			if !reflect.DeepEqual(c.exp, actual) {
				t.Fail()
			}
		})
	}
}

func TestReadElemSegment_refNull(t *testing.T) {
	// flag 5: passive, reftype funcref, expr vec: [ref.null func end, ref.func 2 end]
	buf := []byte{
		0x05,             // flag
		0x70,             // reftype funcref
		0x02,             // vec len
		0xd0, 0x70, 0x0b, // ref.null func, end
		0xd2, 0x02, 0x0b, // ref.func 2, end
	}
	seg, err := segments.ReadElemSegment(bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if !seg.Passive {
		t.Error("flag 5 must be passive")
	}
	if len(seg.Init) != 2 {
		t.Fatalf("Init len=%d want 2", len(seg.Init))
	}
	// a null reference must NOT be folded into function index 0
	if seg.Init[0] != segments.NullElem {
		t.Errorf("Init[0]=%#x want NullElem", seg.Init[0])
	}
	if seg.Init[1] != 2 {
		t.Errorf("Init[1]=%d want 2", seg.Init[1])
	}
}

func TestReadElemSegment_flags(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   *segments.ElemSegment
	}{
		{
			// flag 1: passive, elemkind, funcidx vec
			bytes: []byte{0x01, 0x00, 0x02, 0x05, 0x07},
			exp: &segments.ElemSegment{
				Passive: true,
				Init:    []uint32{5, 7},
			},
		},
		{
			// flag 3: declarative, elemkind, funcidx vec
			bytes: []byte{0x03, 0x00, 0x01, 0x02},
			exp: &segments.ElemSegment{
				Passive: true,
				Init:    []uint32{2},
			},
		},
		{
			// flag 4: active, table 0, offset expr, expr vec (ref.func 5)
			bytes: []byte{0x04, 0x41, 0x01, 0x0b, 0x01, 0xd2, 0x05, 0x0b},
			exp: &segments.ElemSegment{
				OffsetExpr: &expr.Expression{
					OpCode: expr.OpCodeI32Const,
					Data:   []byte{0x01},
				},
				Init: []uint32{5},
			},
		},
		{
			// flag 6: active, explicit tableidx 2, offset expr, reftype, expr vec (ref.null)
			bytes: []byte{0x06, 0x02, 0x41, 0x00, 0x0b, 0x70, 0x01, 0xd0, 0x70, 0x0b},
			exp: &segments.ElemSegment{
				TableIndex: 2,
				OffsetExpr: &expr.Expression{
					OpCode: expr.OpCodeI32Const,
					Data:   []byte{0x00},
				},
				Init: []uint32{segments.NullElem},
			},
		},
		{
			// flag 7: declarative, reftype, expr vec (ref.func 3)
			bytes: []byte{0x07, 0x70, 0x01, 0xd2, 0x03, 0x0b},
			exp: &segments.ElemSegment{
				Passive: true,
				Init:    []uint32{3},
			},
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual, err := segments.ReadElemSegment(bytes.NewReader(c.bytes))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(c.exp, actual) {
				t.Errorf("got %#v, want %#v", actual, c.exp)
			}
		})
	}
}

func TestReadElemSegment_errors(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   string
	}{
		// truncated before the flag
		{bytes: []byte{}, exp: "read element segment flag: EOF"},
		// 8 is not a valid element segment flag
		{bytes: []byte{0x08}, exp: "invalid element segment flag: 8"},
		// flag 2 but the table index is missing
		{bytes: []byte{0x02}, exp: "read table index: EOF"},
		// flag 0 but the offset expression is missing
		{bytes: []byte{0x00}, exp: "read expr for offset: read opcode: EOF"},
		// i64.const is not a valid offset expression
		{bytes: []byte{0x00, 0x42, 0x01, 0x0b}, exp: "offset expression must be i32.const or global.get but got 0x42"},
		// flag 1 but the elemkind byte is missing
		{bytes: []byte{0x01}, exp: "read elemkind/reftype: EOF"},
		// 0x33 is neither elemkind 0x00 nor reftype funcref 0x70
		{bytes: []byte{0x01, 0x33}, exp: "unsupported elemkind/reftype: 0x33"},
		// truncated before the vector size
		{bytes: []byte{0x01, 0x00}, exp: "get size of vector: EOF"},
		// funcidx vec declares one entry but it is missing
		{bytes: []byte{0x00, 0x41, 0x00, 0x0b, 0x01}, exp: "read function index: EOF"},
		// expr vec declares one entry but it is missing
		{bytes: []byte{0x05, 0x70, 0x01}, exp: "read element expression: EOF"},
		// ref.func without its function index
		{bytes: []byte{0x05, 0x70, 0x01, 0xd2}, exp: "read element expression: read ref.func index: EOF"},
		// ref.null without its reference type
		{bytes: []byte{0x05, 0x70, 0x01, 0xd0}, exp: "read element expression: read ref.null type: EOF"},
		// i32.const is not a valid element expression
		{bytes: []byte{0x05, 0x70, 0x01, 0x41}, exp: "read element expression: unsupported element expression opcode: 0x41"},
		// ref.func without the terminating end opcode
		{bytes: []byte{0x05, 0x70, 0x01, 0xd2, 0x01}, exp: "read element expression: read element expression end: EOF"},
		// element expression terminated by something other than end
		{bytes: []byte{0x05, 0x70, 0x01, 0xd2, 0x01, 0x0c}, exp: "read element expression: element expression not terminated: 0xc"},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := segments.ReadElemSegment(bytes.NewReader(c.bytes))
			if err == nil || err.Error() != c.exp {
				t.Errorf("got %v, want %q", err, c.exp)
			}
		})
	}
}
