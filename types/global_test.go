package types_test

import (
	"bytes"
	"errors"
	"strconv"
	"testing"

	"github.com/c0mm4nd/wasman/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadGlobalType(t *testing.T) {
	t.Run("ng", func(t *testing.T) {
		buf := []byte{0x7e, 0x3}
		_, err := types.ReadGlobalType(bytes.NewBuffer(buf))
		require.True(t, errors.Is(err, types.ErrInvalidTypeByte))
		t.Log(err)
	})

	for i, c := range []struct {
		bytes []byte
		exp   *types.GlobalType
	}{
		{bytes: []byte{0x7e, 0x00}, exp: &types.GlobalType{ValType: types.ValueTypeI64, Mutable: false}},
		{bytes: []byte{0x7e, 0x01}, exp: &types.GlobalType{ValType: types.ValueTypeI64, Mutable: true}},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual, err := types.ReadGlobalType(bytes.NewBuffer(c.bytes))
			require.NoError(t, err)
			assert.Equal(t, c.exp, actual)
		})
	}
}
