//go:build (darwin || linux) && (arm64 || amd64)

package jit

// CompileOpt translates a function body through the optimizing tier:
// the wasm stack machine is first lowered to a control-flow graph of
// virtual-register three-address code (values, including locals, live in
// registers rather than the memory operand stack), then linear-scan
// register allocation maps virtual registers onto machine registers with
// spills placed in the operand-stack area above the wasm stack heights.
// The runtime contract (Ctx, statuses, host-exit call sites) is identical
// to the baseline tier, so ErrUnsupported simply falls back there.
func CompileOpt(fd *FuncDesc) (*Compiled, error) {
	return nil, ErrUnsupported // frontend lands next
}
