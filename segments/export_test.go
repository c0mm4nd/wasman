package segments_test

import (
	"bytes"
	"errors"
	"reflect"
	"strconv"

	"testing"

	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
)

func TestReadExportDesc(t *testing.T) {
	t.Run("ng", func(t *testing.T) {
		buf := []byte{0x04}
		_, err := segments.ReadExportDesc(bytes.NewReader(buf))
		if !errors.Is(err, types.ErrInvalidTypeByte) {
			t.Log(err)
			t.Fail()
		}
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
			actual, err := segments.ReadExportDesc(bytes.NewReader(c.bytes))
			if err != nil {
				t.Fail()
			}
			if !reflect.DeepEqual(c.exp, actual) {
				t.Fail()
			}
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

	actual, err := segments.ReadExportSegment(bytes.NewReader(buf))
	if err != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(exp, actual) {
		t.Fail()
	}
}

func TestReadExportDesc_errors(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   string
	}{
		// truncated before the kind byte
		{bytes: []byte{}, exp: "read value kind: EOF"},
		// kind present but the index is missing
		{bytes: []byte{0x00}, exp: "read funcidx: EOF"},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := segments.ReadExportDesc(bytes.NewReader(c.bytes))
			if err == nil || err.Error() != c.exp {
				t.Errorf("got %v, want %q", err, c.exp)
			}
		})
	}
}

func TestReadExportSegment_errors(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   string
	}{
		// truncated before the export name
		{bytes: []byte{}, exp: "read name of export module: read size of name: EOF"},
		// name present but the description is missing
		{bytes: []byte{0x01, 'A'}, exp: "read export description: read value kind: EOF"},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := segments.ReadExportSegment(bytes.NewReader(c.bytes))
			if err == nil || err.Error() != c.exp {
				t.Errorf("got %v, want %q", err, c.exp)
			}
		})
	}
}
