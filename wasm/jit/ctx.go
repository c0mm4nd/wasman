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
	Globals  uintptr // base of the []*uint64 global cells
}

// Supported reports whether native codegen exists for this platform.
func Supported() bool { return true }

// enter is implemented in enter_$GOARCH.s. It stores into ctx but does not
// retain it, so the pointer must not be treated as escaping — that keeps the
// per-call Ctx on the caller's stack (one heap allocation per call
// otherwise, which is pure GC pressure on call-heavy workloads).
//
//go:noescape
func enter(code uintptr, ctx *Ctx) uint32

// Call runs an AllocExec mapping against ctx and returns its status code.
func Call(code []byte, ctx *Ctx) uint32 {
	return enter(uintptr(unsafe.Pointer(&code[0])), ctx)
}

// status codes returned by generated code
const (
	StatusOK           = 0 // fell off the end of the function
	StatusUnreachable  = 1
	StatusMemOOB       = 2
	StatusDivZero      = 3
	StatusIntOverflow  = 4
	StatusCall         = 5 // call site: Ctx.TrapInfo holds the site id
	StatusCallIndirect = 6
)

// CallAt runs generated code starting at a continuation offset.
func CallAt(code []byte, off int, ctx *Ctx) uint32 {
	return enter(uintptr(unsafe.Pointer(&code[0]))+uintptr(off), ctx)
}
