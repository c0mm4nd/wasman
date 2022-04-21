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

// memoryBytesNumToPages converts the given number of bytes into the number of pages.
func memoryBytesNumToPages(bytesNum uint64) (pages uint32) {
	return uint32(bytesNum >> config.DefaultMemoryPageSizeInBits)
}

// MemoryPagesToBytesNum converts the given pages into the number of bytes contained in these pages.
func MemoryPagesToBytesNum(pages uint32) (bytesNum uint64) {
	return uint64(pages) << config.DefaultMemoryPageSizeInBits
}

// PageSize returns the current memory buffer size in pages.
func (m *Memory) PageSize() uint32 {
	return memoryBytesNumToPages(uint64(len(m.Value)))
}

func (mem *Memory) Grow(newPages uint32) (result uint32) {
	currentPages := memoryBytesNumToPages(uint64(len(mem.Value)))

	if mem.Max != nil &&
		newPages+currentPages > *(mem.Max) {
		return 0xffffffff // failed to grow
	}

	mem.Value = append(mem.Value, make([]byte, MemoryPagesToBytesNum(newPages))...)

	return currentPages
}
