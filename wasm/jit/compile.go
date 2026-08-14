package jit

import "errors"

// ErrUnsupported marks a function the template compiler cannot translate;
// the engine falls back to the interpreter for it.
var ErrUnsupported = errors.New("jit: unsupported construct")

// FuncDesc is the architecture-neutral description of one function body the
// compiler consumes (pre-decoded by the engine's loader).
type FuncDesc struct {
	Body      []byte
	Imms      []uint64 // pre-decoded immediates indexed by opcode PC
	PcEnd     []uint32 // last byte of each immediate-carrying instruction
	NumLocals int      // params + declared locals
	NumParams int
	NumRets   int
	// BrTables holds pre-decoded br_table plans keyed by opcode PC.
	BrTables map[int]BrTable
}

// BrTable is a pre-decoded br_table: label depths per index plus a default.
type BrTable struct {
	Targets []uint32
	Def     uint32
}

// Compiled is a translated function.
type Compiled struct {
	Code      []byte // executable mapping (AllocExec)
	MaxHeight int    // operand slots the caller must provide
}
