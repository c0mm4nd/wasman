package utils_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"testing"

	"github.com/c0mm4nd/wasman/utils"
)

func TestReadFloat32(t *testing.T) {
	var exp float32 = 3.1231231231
	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, math.Float32bits(exp))
	actual, err := utils.ReadFloat32(bytes.NewReader(bs))
	if err != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(exp, actual) {
		t.Fail()
	}
}

func TestReadFloat64(t *testing.T) {
	exp := 3.1231231231
	bs := make([]byte, 8)
	binary.LittleEndian.PutUint64(bs, math.Float64bits(exp))

	actual, err := utils.ReadFloat64(bytes.NewReader(bs))
	if err != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(exp, actual) {
		t.Fail()
	}
}
