package types_test

import (
	"bytes"
	"reflect"
	"strconv"
	"testing"

	"github.com/c0mm4nd/wasman/utils"

	"github.com/c0mm4nd/wasman/types"
)

func TestReadMemoryType(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   *types.MemoryType
	}{
		{bytes: []byte{0x00, 0xa}, exp: &types.MemoryType{Min: 10}},
		{bytes: []byte{0x01, 0xa, 0xa}, exp: &types.MemoryType{Min: 10, Max: utils.Uint32Ptr(10)}},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual, err := types.ReadMemoryType(bytes.NewReader(c.bytes))
			if err != nil {
				t.Fail()
			}
			if !reflect.DeepEqual(c.exp, actual) {
				t.Fail()
			}
		})
	}
}
