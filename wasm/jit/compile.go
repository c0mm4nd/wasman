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
	// FuncSigs holds the arity of every function in the module's index
	// space; TypeSigs the arity of every entry in the type section. Both
	// are needed to compile call/call_indirect sites.
	FuncSigs []FuncSig
	TypeSigs []FuncSig
}

// FuncSig is a function arity (parameter and result slot counts).
type FuncSig struct {
	In, Out int
}

// CallSite describes one call/call_indirect exit point: where to re-enter,
// the static stack heights around the call, and what to invoke.
type CallSite struct {
	Cont     int // code offset of the continuation (starts with a prologue)
	SpBefore int // stack height at the exit (args, and index if indirect)
	SpAfter  int // stack height expected after the exit completes
	Kind     byte
	FuncIdx  uint32 // direct calls
	TypeIdx  uint32 // indirect calls
	TableIdx uint32
}

// CallSite kinds.
const (
	SiteCall = iota
	SiteCallIndirect
	SiteMemGrow
)

// BrTable is a pre-decoded br_table: label depths per index plus a default.
type BrTable struct {
	Targets []uint32
	Def     uint32
}

// Compiled is a translated function.
type Compiled struct {
	Code      []byte // executable mapping (AllocExec)
	MaxHeight int    // operand slots the caller must provide
	// CallSites, indexed by the id the exiting code leaves in Ctx.TrapInfo,
	// tell the host what to call and where to re-enter.
	CallSites []CallSite
}
