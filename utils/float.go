package utils

import (
	"encoding/binary"
	"io"
	"math"
)

// ReadFloat32 reads a float32 following IEEE 754
func ReadFloat32(r io.Reader) (float32, error) {
	buf := make([]byte, 4)

	_, err := io.ReadFull(r, buf)
	if err != nil {
		return 0, err
	}

	raw := binary.LittleEndian.Uint32(buf)
	return math.Float32frombits(raw), nil
}

// ReadFloat64 reads a float64 following IEEE 754
func ReadFloat64(r io.Reader) (float64, error) {
	buf := make([]byte, 8)

	_, err := io.ReadFull(r, buf)
	if err != nil {
		return 0, err
	}

	raw := binary.LittleEndian.Uint64(buf)
	return math.Float64frombits(raw), nil
}
