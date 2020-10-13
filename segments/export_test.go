package segments_test

import (
	"bytes"
	"errors"
	"strconv"

	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReadExportDesc(t *testing.T) {
	t.Run("ng", func(t *testing.T) {
		buf := []byte{0x04}
		_, err := segments.ReadExportDesc(bytes.NewBuffer(buf))
		require.True(t, errors.Is(err, types.ErrInvalidTypeByte))
		t.Log(err)
	})

	for i, c := range []struct {
		bytes []byte
		exp   *segments.ExportDesc
	}{
		{
			bytes: []byte{0x00, 0x0a},
			exp:   &segments.ExportDesc{Kind: 0, Index: 10},
		},
		{
			bytes: []byte{0x01, 0x05},
			exp:   &segments.ExportDesc{Kind: 1, Index: 5},
		},
		{
			bytes: []byte{0x02, 0x01},
			exp:   &segments.ExportDesc{Kind: 2, Index: 1},
		},
		{
			bytes: []byte{0x03, 0x0b},
			exp:   &segments.ExportDesc{Kind: 3, Index: 11},
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual, err := segments.ReadExportDesc(bytes.NewBuffer(c.bytes))
			require.NoError(t, err)
			assert.Equal(t, c.exp, actual)
		})

	}
}

func TestReadExportSegment(t *testing.T) {
	exp := &segments.ExportSegment{
		Name: "ABC",
		Desc: &segments.ExportDesc{Kind: 0, Index: 10},
	}

	buf := []byte{byte(len(exp.Name))}
	buf = append(buf, exp.Name...)
	buf = append(buf, 0x00, 0x0a)

	actual, err := segments.ReadExportSegment(bytes.NewBuffer(buf))
	require.NoError(t, err)
	assert.Equal(t, exp, actual)
}
