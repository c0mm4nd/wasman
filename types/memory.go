package types

import (
	"io"
)

type MemoryType = Limits

func ReadMemoryType(r io.Reader) (*MemoryType, error) {
	return ReadLimits(r)
}
