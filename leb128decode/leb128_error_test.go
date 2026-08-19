package leb128decode_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/c0mm4nd/wasman/leb128decode"
)

const (
	errMsgOverflow32 = "overflows a 32-bit integer"
	errMsgOverflow33 = "overflows a 33-bit integer"
	errMsgOverflow64 = "overflows a 64-bit integer"
)

func TestDecodeUint32Boundary(t *testing.T) {
	// max-length (5 bytes) encoding of math.MaxUint32
	actual, l, err := leb128decode.DecodeUint32(bytes.NewReader([]byte{0xff, 0xff, 0xff, 0xff, 0x0f}))
	if err != nil {
		t.Fail()
	}
	if actual != math.MaxUint32 {
		t.Fail()
	}
	if l != 5 {
		t.Fail()
	}
}

func TestDecodeUint32Error(t *testing.T) {
	// empty input
	if _, _, err := leb128decode.DecodeUint32(bytes.NewReader(nil)); !errors.Is(err, io.EOF) {
		t.Fail()
	}

	// truncated in the middle of a sequence
	if _, _, err := leb128decode.DecodeUint32(bytes.NewReader([]byte{0x80})); !errors.Is(err, io.EOF) {
		t.Fail()
	}

	for _, c := range [][]byte{
		// unused bits of the 5th byte are not zero
		{0x80, 0x80, 0x80, 0x80, 0x10},
		// more than 5 bytes (5th byte still has the continuation bit)
		{0x80, 0x80, 0x80, 0x80, 0x80},
	} {
		_, _, err := leb128decode.DecodeUint32(bytes.NewReader(c))
		if err == nil || err.Error() != errMsgOverflow32 {
			t.Errorf("DecodeUint32(% x): expected overflow error, got %v", c, err)
		}
	}
}

func TestDecodeUint64Boundary(t *testing.T) {
	// max-length (10 bytes) encoding of math.MaxUint64
	actual, l, err := leb128decode.DecodeUint64(bytes.NewReader(
		[]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}))
	if err != nil {
		t.Fail()
	}
	if actual != math.MaxUint64 {
		t.Fail()
	}
	if l != 10 {
		t.Fail()
	}
}

func TestDecodeUint64Error(t *testing.T) {
	// empty input
	if _, _, err := leb128decode.DecodeUint64(bytes.NewReader(nil)); !errors.Is(err, io.EOF) {
		t.Fail()
	}

	// truncated in the middle of a sequence
	if _, _, err := leb128decode.DecodeUint64(bytes.NewReader([]byte{0x80})); !errors.Is(err, io.EOF) {
		t.Fail()
	}

	for _, c := range [][]byte{
		// unused bits of the 10th byte are not zero (b > 1)
		{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x02},
		// more than 10 bytes (10th byte still has the continuation bit)
		{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80},
	} {
		_, _, err := leb128decode.DecodeUint64(bytes.NewReader(c))
		if err == nil || err.Error() != errMsgOverflow64 {
			t.Errorf("DecodeUint64(% x): expected overflow error, got %v", c, err)
		}
	}
}

func TestDecodeInt32Boundary(t *testing.T) {
	for _, c := range []struct {
		bytes []byte
		exp   int32
	}{
		// max-length (5 bytes) encoding of math.MaxInt32
		{bytes: []byte{0xff, 0xff, 0xff, 0xff, 0x07}, exp: math.MaxInt32},
		// max-length (5 bytes) encoding of math.MinInt32
		{bytes: []byte{0x80, 0x80, 0x80, 0x80, 0x78}, exp: math.MinInt32},
		// max-length (5 bytes) encoding of -1
		{bytes: []byte{0xff, 0xff, 0xff, 0xff, 0x7f}, exp: -1},
	} {
		actual, l, err := leb128decode.DecodeInt32(bytes.NewReader(c.bytes))
		if err != nil {
			t.Fail()
		}
		if actual != c.exp {
			t.Fail()
		}
		if l != uint64(len(c.bytes)) {
			t.Fail()
		}
	}
}

func TestDecodeInt32Error(t *testing.T) {
	// empty input (the error wraps io.EOF)
	if _, _, err := leb128decode.DecodeInt32(bytes.NewReader(nil)); !errors.Is(err, io.EOF) {
		t.Fail()
	}

	// truncated in the middle of a sequence
	if _, _, err := leb128decode.DecodeInt32(bytes.NewReader([]byte{0x80})); !errors.Is(err, io.EOF) {
		t.Fail()
	}

	for _, c := range [][]byte{
		// more than 5 bytes
		{0x80, 0x80, 0x80, 0x80, 0x80, 0x00},
		// 5 bytes, negative result but unused bits are not all one
		{0xff, 0xff, 0xff, 0xff, 0x4f},
		// 5 bytes, non-negative result but unused bits are not zero
		{0x80, 0x80, 0x80, 0x80, 0x10},
	} {
		_, _, err := leb128decode.DecodeInt32(bytes.NewReader(c))
		if err == nil || err.Error() != errMsgOverflow32 {
			t.Errorf("DecodeInt32(% x): expected overflow error, got %v", c, err)
		}
	}
}

func TestDecodeInt33AsInt64Boundary(t *testing.T) {
	for _, c := range []struct {
		bytes []byte
		exp   int64
	}{
		// max-length (5 bytes) encoding of 2^32-1 (the int33 maximum)
		{bytes: []byte{0xff, 0xff, 0xff, 0xff, 0x0f}, exp: 4294967295},
		// max-length (5 bytes) encoding of -2^32 (the int33 minimum)
		{bytes: []byte{0x80, 0x80, 0x80, 0x80, 0x70}, exp: -4294967296},
		// max-length (5 bytes) encoding of -1
		{bytes: []byte{0xff, 0xff, 0xff, 0xff, 0x7f}, exp: -1},
	} {
		actual, l, err := leb128decode.DecodeInt33AsInt64(bytes.NewReader(c.bytes))
		if err != nil {
			t.Fail()
		}
		if actual != c.exp {
			t.Fail()
		}
		if l != uint64(len(c.bytes)) {
			t.Fail()
		}
	}
}

func TestDecodeInt33AsInt64Error(t *testing.T) {
	// empty input (the error wraps io.EOF)
	if _, _, err := leb128decode.DecodeInt33AsInt64(bytes.NewReader(nil)); !errors.Is(err, io.EOF) {
		t.Fail()
	}

	// truncated in the middle of a sequence
	if _, _, err := leb128decode.DecodeInt33AsInt64(bytes.NewReader([]byte{0x80})); !errors.Is(err, io.EOF) {
		t.Fail()
	}

	for _, c := range [][]byte{
		// 5 bytes, negative result but unused bit is not one
		{0xff, 0xff, 0xff, 0xff, 0x10},
		// 5 bytes, non-negative result but unused bit is not zero
		{0x80, 0x80, 0x80, 0x80, 0x20},
	} {
		_, _, err := leb128decode.DecodeInt33AsInt64(bytes.NewReader(c))
		if err == nil || err.Error() != errMsgOverflow33 {
			t.Errorf("DecodeInt33AsInt64(% x): expected overflow error, got %v", c, err)
		}
	}
}

func TestDecodeInt64Boundary(t *testing.T) {
	// max-length (10 bytes) encoding of math.MaxInt64
	actual, l, err := leb128decode.DecodeInt64(bytes.NewReader(
		[]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x00}))
	if err != nil {
		t.Fail()
	}
	if actual != math.MaxInt64 {
		t.Fail()
	}
	if l != 10 {
		t.Fail()
	}
}

func TestDecodeInt64Error(t *testing.T) {
	// empty input (the error wraps io.EOF)
	if _, _, err := leb128decode.DecodeInt64(bytes.NewReader(nil)); !errors.Is(err, io.EOF) {
		t.Fail()
	}

	// truncated in the middle of a sequence
	if _, _, err := leb128decode.DecodeInt64(bytes.NewReader([]byte{0x80})); !errors.Is(err, io.EOF) {
		t.Fail()
	}

	for _, c := range [][]byte{
		// more than 10 bytes
		{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x00},
		// 10 bytes, negative result but unused bits are not all one
		{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x41},
		// 10 bytes, non-negative result but unused bits are not zero
		{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x02},
	} {
		_, _, err := leb128decode.DecodeInt64(bytes.NewReader(c))
		if err == nil || err.Error() != errMsgOverflow64 {
			t.Errorf("DecodeInt64(% x): expected overflow error, got %v", c, err)
		}
	}
}
