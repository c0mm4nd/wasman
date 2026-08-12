package segments_test

import (
	"bytes"
	"reflect"
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
