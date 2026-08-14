//go:build !((darwin || linux) && (arm64 || amd64))

package jit

// Ctx mirrors the arm64 layout on unsupported platforms.
type Ctx struct {
	Stack    uintptr
	Sp       uint64
	Locals   uintptr
	Mem      uintptr
	MemLen   uint64
	TrapInfo uint64
	Globals  uintptr
}

// Supported reports whether native codegen exists for this platform.
func Supported() bool { return false }

// AllocExec is unavailable on this platform.
func AllocExec(code []byte) ([]byte, error) { return nil, errUnsupported }

// Free is unavailable on this platform.
func Free(mem []byte) error { return errUnsupported }

// Call is unavailable on this platform.
func Call(code []byte, ctx *Ctx) uint32 { return ^uint32(0) }

type unsupportedErr struct{}

func (unsupportedErr) Error() string { return "jit: unsupported platform" }

var errUnsupported = unsupportedErr{}

const (
	StatusOK          = 0
	StatusUnreachable = 1
	StatusMemOOB      = 2
	StatusDivZero     = 3
	StatusIntOverflow = 4
)

// Compile is unavailable on this platform: everything falls back to the
// interpreter.
func Compile(fd *FuncDesc) (*Compiled, error) { return nil, errUnsupported }
