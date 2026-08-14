//go:build (darwin || linux) && (arm64 || amd64)

package jit

import "unsafe"

// Ctx is the register file the generated code loads on entry and stores on
// exit. Field order is part of the ABI with the generated prologue/epilogue.
type Ctx struct {
	Stack    uintptr // operand stack base (\*uint64)
	Sp       uint64  // stack index in slots
	Locals   uintptr // locals base (\*uint64)
	Mem      uintptr // linear memory base
	MemLen   uint64  // linear memory length in bytes
	TrapInfo uint64  // extra trap detail (e.g. faulting offset)
}

// Supported reports whether native codegen exists for this platform.
func Supported() bool { return true }

// enter is implemented in enter_$GOARCH.s.
func enter(code uintptr, ctx *Ctx) uint32

// Call runs an AllocExec mapping against ctx and returns its status code.
func Call(code []byte, ctx *Ctx) uint32 {
	return enter(uintptr(unsafe.Pointer(&code[0])), ctx)
}

// status codes returned by generated code
const (
	StatusOK          = 0 // fell off the end of the function
	StatusUnreachable = 1
	StatusMemOOB      = 2
	StatusDivZero     = 3
	StatusIntOverflow = 4
)
