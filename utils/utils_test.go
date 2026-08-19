package utils_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/c0mm4nd/wasman/utils"
)

func TestCalcPageSize(t *testing.T) {
	for _, c := range []struct {
		contentLen int
		pageSize   int
		exp        int
	}{
		{contentLen: 0, pageSize: 100, exp: 0},
		{contentLen: 1, pageSize: 100, exp: 1},
		{contentLen: 99, pageSize: 100, exp: 1},
		{contentLen: 100, pageSize: 100, exp: 1}, // exact multiple, no extra page
		{contentLen: 101, pageSize: 100, exp: 2}, // remainder needs one more page
		{contentLen: 65536 * 3, pageSize: 65536, exp: 3},
		{contentLen: 65536*3 + 1, pageSize: 65536, exp: 4},
	} {
		if actual := utils.CalcPageSize(c.contentLen, c.pageSize); actual != c.exp {
			t.Errorf("CalcPageSize(%d, %d) = %d, expected %d", c.contentLen, c.pageSize, actual, c.exp)
		}
	}
}

func TestUint32Ptr(t *testing.T) {
	p := utils.Uint32Ptr(42)
	if p == nil || *p != 42 {
		t.Fail()
	}

	// each call must return a distinct pointer
	q := utils.Uint32Ptr(42)
	if p == q {
		t.Fail()
	}
	*q = 7
	if *p != 42 || *q != 7 {
		t.Fail()
	}
}

func TestReadFloat32Error(t *testing.T) {
	// empty reader
	if _, err := utils.ReadFloat32(bytes.NewReader(nil)); !errors.Is(err, io.EOF) {
		t.Fail()
	}

	// truncated input (less than 4 bytes)
	if _, err := utils.ReadFloat32(bytes.NewReader([]byte{0x01, 0x02})); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fail()
	}
}

func TestReadFloat64Error(t *testing.T) {
	// empty reader
	if _, err := utils.ReadFloat64(bytes.NewReader(nil)); !errors.Is(err, io.EOF) {
		t.Fail()
	}

	// truncated input (less than 8 bytes)
	if _, err := utils.ReadFloat64(bytes.NewReader([]byte{0x01, 0x02, 0x03, 0x04})); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fail()
	}
}
