package types_test

import (
	"bytes"
	"github.com/c0mm4nd/wasman/utils"
	"strconv"
	"testing"

	"github.com/c0mm4nd/wasman/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			actual, err := types.ReadLimits(bytes.NewBuffer(c.bytes))
			require.NoError(t, err)
			assert.Equal(t, c.exp, actual)
		})
	}
}
