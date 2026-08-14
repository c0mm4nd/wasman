//go:build (darwin || linux) && (arm64 || amd64)

package jit

// The optimizing tier: see opt_ir.go (frontend), opt_regalloc.go
// (allocation) and opt_<arch>.go (code generation). CompileOpt is defined
// per architecture; platforms without a port return ErrUnsupported and the
// engine falls back to the baseline tier.
