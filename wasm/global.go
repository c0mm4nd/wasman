package wasm

import (
	"math"

	"github.com/c0mm4nd/wasman/types"
)

// Global is an instance of the global value
type Global struct {
	*types.GlobalType
	Val interface{}

	// cell is the shared runtime storage (raw 64-bit encoding). An imported
	// global aliases the exporter's cell, so a global.set in one module is
	// visible in every module sharing the global.
	cell *uint64
}

// ensureCell lazily allocates the runtime storage, encoding Val into it.
func (g *Global) ensureCell() *uint64 {
	if g.cell == nil {
		var v uint64
		switch x := g.Val.(type) {
		case int32:
			v = uint64(x)
		case int64:
			v = uint64(x)
		case float32:
			v = uint64(math.Float32bits(x))
		case float64:
			v = math.Float64bits(x)
		}
		g.cell = &v
	}
	return g.cell
}
