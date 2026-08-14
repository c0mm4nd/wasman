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
	StatusOK           = 0
	StatusUnreachable  = 1
	StatusMemOOB       = 2
	StatusDivZero      = 3
	StatusIntOverflow  = 4
	StatusCall         = 5
	StatusCallIndirect = 6
	StatusConvInvalid  = 7
	StatusConvOverflow = 8
)

// CallAt is unavailable on this platform.
func CallAt(code []byte, off int, ctx *Ctx) uint32 { return ^uint32(0) }

// CompileBaseline is unavailable on this platform: everything falls back
// to the interpreter.
func CompileBaseline(fd *FuncDesc) (*Compiled, error) { return nil, errUnsupported }

// CompileOpt is unavailable on this platform.
func CompileOpt(fd *FuncDesc) (*Compiled, error) { return nil, errUnsupported }
