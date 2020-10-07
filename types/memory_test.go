package types_test

import (
	"bytes"
	"github.com/c0mm4nd/wasman/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func TestReadMemoryType(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   *types.MemoryType
	}{
		{bytes: []byte{0x00, 0xa}, exp: &types.MemoryType{Min: 10}},
		{bytes: []byte{0x01, 0xa, 0xa}, exp: &types.MemoryType{Min: 10, Max: uint32Ptr(10)}},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual, err := types.ReadMemoryType(bytes.NewBuffer(c.bytes))
			require.NoError(t, err)
			assert.Equal(t, c.exp, actual)
		})
	}
}
