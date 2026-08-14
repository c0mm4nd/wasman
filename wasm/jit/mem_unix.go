//go:build (darwin || linux) && arm64

// Package jit provides native code generation for the wasman engine
// (template JIT: each wasm opcode expands to a fixed native sequence).
package jit

import (
	"fmt"
	"syscall"
)

// AllocExec copies code into a fresh page-aligned mapping and makes it
// executable (write first, then flip to R|X — the W^X-friendly order that
// needs no special entitlements on darwin/arm64). The returned slice is the
// live executable mapping; free it with Free when done.
func AllocExec(code []byte) ([]byte, error) {
	if len(code) == 0 {
		return nil, fmt.Errorf("empty code")
	}
	mem, err := syscall.Mmap(-1, 0, len(code),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_PRIVATE|syscall.MAP_ANON)
	if err != nil {
		return nil, fmt.Errorf("mmap: %w", err)
	}
	copy(mem, code)
	if err := syscall.Mprotect(mem, syscall.PROT_READ|syscall.PROT_EXEC); err != nil {
		_ = syscall.Munmap(mem)
		return nil, fmt.Errorf("mprotect: %w", err)
	}
	return mem, nil
}

// Free unmaps an AllocExec mapping.
func Free(mem []byte) error { return syscall.Munmap(mem) }
