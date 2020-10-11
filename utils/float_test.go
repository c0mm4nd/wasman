package utils_test

import (
	"bytes"
	"encoding/binary"
	"github.com/c0mm4nd/wasman/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func TestReadFloat32(t *testing.T) {
	var exp float32 = 3.1231231231
	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, math.Float32bits(exp))
	actual, err := utils.ReadFloat32(bytes.NewBuffer(bs))
	require.NoError(t, err)
	assert.Equal(t, exp, actual)
}

func TestReadFloat64(t *testing.T) {
	exp := 3.1231231231
	bs := make([]byte, 8)
	binary.LittleEndian.PutUint64(bs, math.Float64bits(exp))

	actual, err := utils.ReadFloat64(bytes.NewBuffer(bs))
	require.NoError(t, err)
	assert.Equal(t, exp, actual)
}
