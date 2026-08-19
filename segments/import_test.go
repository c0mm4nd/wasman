package segments_test

import (
	"bytes"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
	"github.com/c0mm4nd/wasman/utils"
)

func TestReadImportDesc(t *testing.T) {
	t.Run("ng", func(t *testing.T) {
		buf := []byte{0x04}
		_, err := segments.ReadImportDesc(bytes.NewReader(buf))
		if !errors.Is(err, types.ErrInvalidTypeByte) {
			t.Log(err)
			t.Fail()
		}
	})

	for i, c := range []struct {
		bytes []byte
		exp   *segments.ImportDesc
	}{
		{
			bytes: []byte{0x00, 0x0a},
			exp: &segments.ImportDesc{
				Kind:         0,
				TypeIndexPtr: utils.Uint32Ptr(10),
			},
		},
		{
			bytes: []byte{0x01, 0x70, 0x0, 0x0a},
			exp: &segments.ImportDesc{
				Kind: 1,
				TableTypePtr: &types.TableType{
					Elem:   0x70,
					Limits: &types.Limits{Min: 10},
				},
			},
		},
		{
			bytes: []byte{0x02, 0x0, 0x0a},
			exp: &segments.ImportDesc{
				Kind:       2,
				MemTypePtr: &types.MemoryType{Min: 10},
			},
		},
		{
			bytes: []byte{0x03, 0x7e, 0x01},
			exp: &segments.ImportDesc{
				Kind:          3,
				GlobalTypePtr: &types.GlobalType{ValType: types.ValueTypeI64, Mutable: true},
			},
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual, err := segments.ReadImportDesc(bytes.NewReader(c.bytes))
			if err != nil {
				t.Fail()
			}
			if !reflect.DeepEqual(c.exp, actual) {
				t.Fail()
			}
		})

	}
}

func TestReadImportSegment(t *testing.T) {
	exp := &segments.ImportSegment{
		Module: "abc",
		Name:   "ABC",
		Desc:   &segments.ImportDesc{Kind: 0, TypeIndexPtr: utils.Uint32Ptr(10)},
	}

	buf := []byte{byte(len(exp.Module))}
	buf = append(buf, exp.Module...)
	buf = append(buf, byte(len(exp.Name)))
	buf = append(buf, exp.Name...)
	buf = append(buf, 0x00, 0x0a)

	actual, err := segments.ReadImportSegment(bytes.NewReader(buf))
	if err != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(exp, actual) {
		t.Fail()
	}
}

func TestReadImportDesc_errors(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   string
	}{
		// truncated before the kind byte
		{bytes: []byte{}, exp: "read value kind: EOF"},
		// func import without its type index
		{bytes: []byte{0x00}, exp: "read typeindex: EOF"},
		// table import without its table type
		{bytes: []byte{0x01}, exp: "read table type: read leading byte: EOF"},
		// mem import without its memory type
		{bytes: []byte{0x02}, exp: "read table type: read leading byte: EOF"},
		// global import without its global type
		{bytes: []byte{0x03}, exp: "read global type: read value type: EOF"},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := segments.ReadImportDesc(bytes.NewReader(c.bytes))
			if err == nil || err.Error() != c.exp {
				t.Errorf("got %v, want %q", err, c.exp)
			}
		})
	}
}

func TestReadImportSegment_errors(t *testing.T) {
	for i, c := range []struct {
		bytes []byte
		exp   string
	}{
		// truncated before the module name
		{bytes: []byte{}, exp: "read name of imported module: read size of name: EOF"},
		// module name present but the component name is missing
		{bytes: []byte{0x01, 'a'}, exp: "read name of imported module component: read size of name: EOF"},
		// both names present but the description is missing
		{bytes: []byte{0x01, 'a', 0x01, 'A'}, exp: "read import description : read value kind: EOF"},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := segments.ReadImportSegment(bytes.NewReader(c.bytes))
			if err == nil || err.Error() != c.exp {
				t.Errorf("got %v, want %q", err, c.exp)
			}
		})
	}
}
