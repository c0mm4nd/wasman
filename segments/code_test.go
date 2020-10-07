package segments_test

import (
	"bytes"
	"github.com/c0mm4nd/wasman/segments"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReadCodeSegment(t *testing.T) {
	buf := []byte{0x9, 0x1, 0x1, 0x1, 0x1, 0x1, 0x12, 0x3, 0x01, 0x0b}
	exp := &segments.CodeSegment{
		NumLocals: 0x01,
		Body:      []byte{0x1, 0x1, 0x12, 0x3, 0x01},
	}
	actual, err := segments.ReadCodeSegment(bytes.NewBuffer(buf))
	require.NoError(t, err)
	assert.Equal(t, exp, actual)
}
