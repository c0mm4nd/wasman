package segments_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/c0mm4nd/wasman/segments"
)

func TestReadCodeSegment(t *testing.T) {
	buf := []byte{0x9, 0x1, 0x1, 0x1, 0x1, 0x1, 0x12, 0x3, 0x01, 0x0b}
	exp := &segments.CodeSegment{
		NumLocals: 0x01,
		Body:      []byte{0x1, 0x1, 0x12, 0x3, 0x01},
	}
	actual, err := segments.ReadCodeSegment(bytes.NewBuffer(buf))
	if err != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(exp, actual) {
		t.Fail()
	}
}
