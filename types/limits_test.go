package types_test

import (
	"bytes"
	"reflect"
	"strconv"
	"testing"

	"github.com/c0mm4nd/wasman/utils"

	"github.com/c0mm4nd/wasman/types"
)

func TestReadLimitsType(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   *types.Limits
	}{
		{bytes: []byte{0x00, 0xa}, exp: &types.Limits{Min: 10}},
		{bytes: []byte{0x01, 0xa, 0xa}, exp: &types.Limits{Min: 10, Max: utils.Uint32Ptr(10)}},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual, err := types.ReadLimits(bytes.NewReader(c.bytes))
			if err != nil {
				t.Fail()
			}
			if !reflect.DeepEqual(c.exp, actual) {
				t.Fail()
			}
		})
	}
}
