package wasm

import (
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/types"
)

// Memory is an instance of the memory value
type Memory struct {
	types.MemoryType
	Value []byte
}

func (mem *Memory) Grow(delta int) uint32 {
	if mem.Max != nil && uint32(delta)+uint32(len(mem.Value))/config.DefaultPageSize > *(mem.Max) {
		return 0 // failed to grow
	}

	ptr := uint32(len(mem.Value)) / config.DefaultPageSize
	mem.Value = append(mem.Value, make([]byte, delta*config.DefaultPageSize)...)

	return ptr
}
