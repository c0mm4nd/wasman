//go:build (darwin || linux) && (arm64 || amd64)

package jit

// The optimizing tier: see opt_ir.go (frontend), opt_regalloc.go
// (allocation) and opt_<arch>.go (code generation). CompileOpt is defined
// per architecture; platforms without a port return ErrUnsupported and the
// engine falls back to the baseline tier.

// OptEligible reports whether fd is inside the optimizing tier's subset
// without generating code — the engine uses it to fix the native-call
// target set before compiling anything (mutually recursive functions may
// reference targets that compile later).
func OptEligible(fd *FuncDesc) bool {
	fe := &irFrontend{fd: fd}
	if err := fe.lower(); err != nil {
		return false
	}
	return fd.NumLocals*8 < 4096
}
