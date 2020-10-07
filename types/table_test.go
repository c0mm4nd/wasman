package types_test

import (
	"bytes"
	"errors"
	"github.com/c0mm4nd/wasman/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func TestReadTableType(t *testing.T) {
	t.Run("ng", func(t *testing.T) {
		buf := []byte{0x00}
		_, err := types.ReadTableType(bytes.NewBuffer(buf))
		require.True(t, errors.Is(err, types.ErrInvalidByte))
		t.Log(err)
	})

	for i, c := range []struct {
		bytes []byte
		exp   *types.TableType
	}{
		{
			bytes: []byte{0x70, 0x00, 0xa},
			exp: &types.TableType{
				Elem:   0x70,
				Limits: &types.Limits{Min: 10},
			},
		},
		{
			bytes: []byte{0x70, 0x01, 0x01, 0xa},
			exp: &types.TableType{
				Elem:   0x70,
				Limits: &types.Limits{Min: 1, Max: uint32Ptr(10)},
			},
		},
	} {
		c := c
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual, err := types.ReadTableType(bytes.NewBuffer(c.bytes))
			require.NoError(t, err)
			assert.Equal(t, c.exp, actual)
		})
	}
}
