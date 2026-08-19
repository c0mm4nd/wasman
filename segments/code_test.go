package segments_test

import (
	"bytes"
	"reflect"
	"strconv"
	"testing"

	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
)

func TestReadCodeSegment(t *testing.T) {
	// size 9: locals vec (1 entry: 1 x i32), body 0x1 0x1 0x12 0x3 0x01, end
	buf := []byte{0x9, 0x1, 0x1, 0x7f, 0x1, 0x1, 0x12, 0x3, 0x01, 0x0b}
	exp := &segments.CodeSegment{
		NumLocals:  0x01,
		LocalDecls: []segments.LocalDecl{{Count: 1, Type: types.ValueTypeI32}},
		Body:       []byte{0x1, 0x1, 0x12, 0x3, 0x01},
	}
	actual, err := segments.ReadCodeSegment(bytes.NewReader(buf))
	if err != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(exp, actual) {
		t.Fail()
	}
}

func TestReadCodeSegment_errors(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   string
	}{
		// truncated before the segment size
		{bytes: []byte{}, exp: "get the size of code segment: EOF"},
		// truncated before the locals vector size
		{bytes: []byte{0x01}, exp: "get the size locals: EOF"},
		// size 0 cannot even hold the locals vector size byte
		{bytes: []byte{0x00, 0x00}, exp: "EOF"},
		// locals vector declares one entry but the count is missing
		{bytes: []byte{0x02, 0x01}, exp: "read n of locals: EOF"},
		// size 1 is used up by the locals vector size, no room for the entry
		{bytes: []byte{0x01, 0x01, 0x01}, exp: "EOF"},
		// two entries of 0xFFFFFFFF locals overflow the u32 local count
		{bytes: []byte{0x14, 0x02, 0xff, 0xff, 0xff, 0xff, 0x0f, 0x7f, 0xff, 0xff, 0xff, 0xff, 0x0f},
			exp: "too many locals: 8589934590"},
		// local count present but its value type is missing
		{bytes: []byte{0x05, 0x01, 0x01}, exp: "read type of local"},
		// 0x22 is not a value type
		{bytes: []byte{0x05, 0x01, 0x01, 0x22}, exp: "invalid local type: 0x22"},
		// nothing left for the body, not even the end opcode
		{bytes: []byte{0x01, 0x00}, exp: "code segment body is empty"},
		// body declared as 2 bytes but only 1 remains
		{bytes: []byte{0x03, 0x00, 0x0b}, exp: "read body: unexpected EOF"},
		// body does not end with the end opcode
		{bytes: []byte{0x02, 0x00, 0x01}, exp: "expr not end with opcodes.OpCodeEnd"},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := segments.ReadCodeSegment(bytes.NewReader(c.bytes))
			if err == nil || err.Error() != c.exp {
				t.Errorf("got %v, want %q", err, c.exp)
			}
		})
	}
}
