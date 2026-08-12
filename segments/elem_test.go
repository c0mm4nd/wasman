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
