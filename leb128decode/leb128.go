package leb128decode

import (
	"bytes"
	"errors"
	"fmt"
)

const (
	maxVarintLen32 = 5
	maxVarintLen64 = 10
)

var (
	// leb128 decoding errors
	errOverflow32 = errors.New("overflows a 32-bit integer")
	errOverflow33 = errors.New("overflows a 33-bit integer")
	errOverflow64 = errors.New("overflows a 64-bit integer")
)

// DecodeUint32 will decode a uint32 from io.Reader, returning it as the ret with the bytes length l which it read.
func DecodeUint32(r *bytes.Reader) (ret uint32, bytesRead uint64, err error) {
	// Derived from https://github.com/golang/go/blob/aafad20b617ee63d58fcd4f6e0d98fe27760678c/src/encoding/binary/varint.go
	// with the modification on the overflow handling tailored for 32-bits.
	var s uint32
	var b byte
	for i := 0; i < maxVarintLen32; i++ {
		b, err = r.ReadByte()
		if err != nil {
			return 0, 0, err
		}
		if b < 0x80 {
			// Unused bits must be all zero.
			if i == maxVarintLen32-1 && (b&0xf0) > 0 {
				return 0, 0, errOverflow32
			}
			return ret | uint32(b)<<s, uint64(i) + 1, nil
		}
		ret |= (uint32(b) & 0x7f) << s
		s += 7
	}
	return 0, 0, errOverflow32
}

// DecodeUint64 will decode a uint64 from io.Reader, returning it as the ret with the bytes length l which it read.
func DecodeUint64(r *bytes.Reader) (ret uint64, bytesRead uint64, err error) {
	// Derived from https://github.com/golang/go/blob/aafad20b617ee63d58fcd4f6e0d98fe27760678c/src/encoding/binary/varint.go
	var s uint64
	var b byte
	for i := 0; i < maxVarintLen64; i++ {
		b, err = r.ReadByte()
		if err != nil {
			return 0, 0, err
		}
		if b < 0x80 {
			// Unused bits (non first bit) must all be zero.
			if i == maxVarintLen64-1 && b > 1 {
				return 0, 0, errOverflow64
			}
			return ret | uint64(b)<<s, uint64(i) + 1, nil
		}
		ret |= (uint64(b) & 0x7f) << s
		s += 7
	}
	return 0, 0, errOverflow64
}

// DecodeInt32 will decode a int32 from io.Reader, returning it as the ret with the bytes length l which it read.
func DecodeInt32(r *bytes.Reader) (ret int32, bytesRead uint64, err error) {
	var shift int
	var b byte
	for {
		b, err = r.ReadByte()
		if err != nil {
			return 0, 0, fmt.Errorf("readByte failed: %w", err)
		}
		ret |= (int32(b) & 0x7f) << shift
		shift += 7
		bytesRead++
		if b&0x80 == 0 {
			if shift < 32 && (b&0x40) != 0 {
				ret |= ^0 << shift
			}
			// Over flow checks.
			// fixme: can be optimized.
			if bytesRead > 5 {
				return 0, 0, errOverflow32
			} else if unused := b & 0b00110000; bytesRead == 5 && ret < 0 && unused != 0b00110000 {
				return 0, 0, errOverflow32
			} else if bytesRead == 5 && ret >= 0 && unused != 0x00 {
				return 0, 0, errOverflow32
			}
			return
		}
	}
}

// DecodeInt33AsInt64 will decode a int33 from io.Reader, returning it as the int64 ret with the bytes length l which it read.
func DecodeInt33AsInt64(r *bytes.Reader) (ret int64, bytesRead uint64, err error) {
	const (
		int33Mask  int64 = 1 << 7
		int33Mask2       = ^int33Mask
		int33Mask3       = 1 << 6
		int33Mask4       = 8589934591 // 2^33-1
		int33Mask5       = 1 << 32
		int33Mask6       = int33Mask4 + 1 // 2^33
	)
	var shift int
	var b int64
	var rb byte
	for shift < 35 {
		rb, err = r.ReadByte()
		if err != nil {
			return 0, 0, fmt.Errorf("readByte failed: %w", err)
		}
		b = int64(rb)
		ret |= (b & int33Mask2) << shift
		shift += 7
		bytesRead++
		if b&int33Mask == 0 {
			break
		}
	}

	// fixme: can be optimized
	if shift < 33 && (b&int33Mask3) == int33Mask3 {
		ret |= int33Mask4 << shift
	}
	ret = ret & int33Mask4

	// if 33rd bit == 1, we translate it as a corresponding signed-33bit minus value
	if ret&int33Mask5 > 0 {
		ret = ret - int33Mask6
	}
	// Over flow checks.
	// fixme: can be optimized.
	if bytesRead > 5 {
		return 0, 0, errOverflow33
	} else if unused := b & 0b00100000; bytesRead == 5 && ret < 0 && unused != 0b00100000 {
		return 0, 0, errOverflow33
	} else if bytesRead == 5 && ret >= 0 && unused != 0x00 {
		return 0, 0, errOverflow33
	}
	return ret, bytesRead, nil
}

// DecodeInt64 will decode a int64 from io.Reader, returning it as the ret with the bytes length l which it read.
func DecodeInt64(r *bytes.Reader) (ret int64, bytesRead uint64, err error) {
	const (
		int64Mask3 = 1 << 6
		int64Mask4 = ^0
	)
	var shift int
	var b byte
	for {
		b, err = r.ReadByte()
		if err != nil {
			return 0, 0, fmt.Errorf("readByte failed: %w", err)
		}
		ret |= (int64(b) & 0x7f) << shift
		shift += 7
		bytesRead++
		if b&0x80 == 0 {
			if shift < 64 && (b&int64Mask3) == int64Mask3 {
				ret |= int64Mask4 << shift
			}
			// Over flow checks.
			// fixme: can be optimized.
			if bytesRead > 10 {
				return 0, 0, errOverflow64
			} else if unused := b & 0b00111110; bytesRead == 10 && ret < 0 && unused != 0b00111110 {
				return 0, 0, errOverflow64
			} else if bytesRead == 10 && ret >= 0 && unused != 0x00 {
				return 0, 0, errOverflow64
			}
			return
		}
	}
}
