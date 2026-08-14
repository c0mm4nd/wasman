//go:build (darwin || linux) && amd64

package jit

// CompileOpt: the amd64 port of the optimizing tier is pending; the
// baseline tier handles everything in the meantime.
func CompileOpt(fd *FuncDesc) (*Compiled, error) { return nil, ErrUnsupported }
