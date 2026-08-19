package types_test

import (
	"bytes"
	"errors"
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

func TestReadLimitsType_errors(t *testing.T) {
	t.Run("ng", func(t *testing.T) {
		// 0x02 is not a valid limits flag
		buf := []byte{0x02}
		_, err := types.ReadLimits(bytes.NewReader(buf))
		if !errors.Is(err, types.ErrInvalidTypeByte) {
			t.Log(err)
			t.Fail()
		}
		exp := "invalid byte for limits: 0x2 != 0x00 or 0x01"
		if err == nil || err.Error() != exp {
			t.Errorf("got %v, want %q", err, exp)
		}
	})

	for i, c := range []struct {
		bytes []byte
		exp   string
	}{
		// truncated before the flag byte
		{bytes: []byte{}, exp: "read leading byte: EOF"},
		// flag 0x00 but the min is missing
		{bytes: []byte{0x00}, exp: "read min of limit: EOF"},
		// flag 0x01 but the min is missing
		{bytes: []byte{0x01}, exp: "read min of limit: EOF"},
		// flag 0x01 but the max is missing
		{bytes: []byte{0x01, 0x0a}, exp: "read min of limit: EOF"},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := types.ReadLimits(bytes.NewReader(c.bytes))
			if err == nil || err.Error() != c.exp {
				t.Errorf("got %v, want %q", err, c.exp)
			}
		})
	}
}
