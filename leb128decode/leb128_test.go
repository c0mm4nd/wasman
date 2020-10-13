package leb128decode_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c0mm4nd/wasman/leb128decode"
)

func TestDecodeUint32(t *testing.T) {
	for _, c := range []struct {
		bytes []byte
		exp   uint32
	}{
		{bytes: []byte{0x04}, exp: 4},
		{bytes: []byte{0x80, 0x7f}, exp: 16256},
		{bytes: []byte{0xe5, 0x8e, 0x26}, exp: 624485},
		{bytes: []byte{0x80, 0x80, 0x80, 0x4f}, exp: 165675008},
		{bytes: []byte{0x89, 0x80, 0x80, 0x80, 0x01}, exp: 268435465},
	} {
		actual, l, err := leb128decode.DecodeUint32(bytes.NewReader(c.bytes))
		require.NoError(t, err)
		assert.Equal(t, c.exp, actual)
		assert.Equal(t, uint64(len(c.bytes)), l)
	}
}

func TestDecodeUint64(t *testing.T) {
	for _, c := range []struct {
		bytes []byte
		exp   uint64
	}{
		{bytes: []byte{0x04}, exp: 4},
		{bytes: []byte{0x80, 0x7f}, exp: 16256},
		{bytes: []byte{0xe5, 0x8e, 0x26}, exp: 624485},
		{bytes: []byte{0x80, 0x80, 0x80, 0x4f}, exp: 165675008},
		{bytes: []byte{0x89, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x01}, exp: 9223372036854775817},
	} {
		actual, l, err := leb128decode.DecodeUint64(bytes.NewReader(c.bytes))
		require.NoError(t, err)
		assert.Equal(t, c.exp, actual)
		assert.Equal(t, uint64(len(c.bytes)), l)
	}
}

func TestDecodeInt32(t *testing.T) {
	for _, c := range []struct {
		bytes []byte
		exp   int32
	}{
		{bytes: []byte{0x00}, exp: 0},
		{bytes: []byte{0x04}, exp: 4},
		{bytes: []byte{0xFF, 0x00}, exp: 127},
		{bytes: []byte{0x81, 0x01}, exp: 129},
		{bytes: []byte{0x7f}, exp: -1},
		{bytes: []byte{0x81, 0x7f}, exp: -127},
		{bytes: []byte{0xFF, 0x7e}, exp: -129},
	} {
		actual, l, err := leb128decode.DecodeInt32(bytes.NewReader(c.bytes))
		require.NoError(t, err)
		assert.Equal(t, c.exp, actual)
		assert.Equal(t, uint64(len(c.bytes)), l)
	}
}

func TestDecodeInt33AsInt64(t *testing.T) {
	for _, c := range []struct {
		bytes []byte
		exp   int64
	}{
		{bytes: []byte{0x00}, exp: 0},
		{bytes: []byte{0x04}, exp: 4},
		{bytes: []byte{0x40}, exp: -64},
		{bytes: []byte{0x7f}, exp: -1},
		{bytes: []byte{0x7e}, exp: -2},
		{bytes: []byte{0x7d}, exp: -3},
		{bytes: []byte{0x7c}, exp: -4},
		{bytes: []byte{0xFF, 0x00}, exp: 127},
		{bytes: []byte{0x81, 0x01}, exp: 129},
		{bytes: []byte{0x7f}, exp: -1},
		{bytes: []byte{0x81, 0x7f}, exp: -127},
		{bytes: []byte{0xFF, 0x7e}, exp: -129},
	} {
		actual, l, err := leb128decode.DecodeInt33AsInt64(bytes.NewReader(c.bytes))
		require.NoError(t, err)
		assert.Equal(t, c.exp, actual)
		assert.Equal(t, uint64(len(c.bytes)), l)
	}
}

func TestDecodeInt64(t *testing.T) {
	for _, c := range []struct {
		bytes []byte
		exp   int64
	}{
		{bytes: []byte{0x00}, exp: 0},
		{bytes: []byte{0x04}, exp: 4},
		{bytes: []byte{0xFF, 0x00}, exp: 127},
		{bytes: []byte{0x81, 0x01}, exp: 129},
		{bytes: []byte{0x7f}, exp: -1},
		{bytes: []byte{0x81, 0x7f}, exp: -127},
		{bytes: []byte{0xFF, 0x7e}, exp: -129},
		{bytes: []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x7f},
			exp: -9223372036854775808},
	} {
		actual, l, err := leb128decode.DecodeInt64(bytes.NewReader(c.bytes))
		require.NoError(t, err)
		assert.Equal(t, c.exp, actual)
		assert.Equal(t, uint64(len(c.bytes)), l)
	}
}
