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
	// NativeFuncs marks index-space entries compiled with the native-call
	// ABI: direct calls to them bypass the host exit entirely. nil keeps
	// every call on the exit protocol.
	NativeFuncs []bool
	// SelfIdx is this function's index-space position (host exits report
	// funcIdx<<32|siteID so the host can find the site table of whichever
	// frame exited); DepthLimit, when nonzero, bakes a call-depth check
	// into the prologue.
	SelfIdx    uint32
	DepthLimit uint64
	// TypeSigIDs maps type-section indices to instance-wide structural
	// signature ids (the call_indirect fast path compares them natively).
	TypeSigIDs []uint32
}

// FuncSig is a function arity (parameter and result slot counts) plus the
// total locals count, which native call sites need to lay out the callee
// frame.
type FuncSig struct {
	In, Out int
	Locals  int
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
	// NativeABI marks code built for the in-stack frame convention: the
	// host enters it with ctx.Stack pointing at the frame's stack base
	// inside the dedicated native stack, and ctx.Sp reports frame
	// pointers rather than slot counts.
	NativeABI  bool
	FrameSlots int // locals + stack + spills + linkage, for the host entry
	LocalSlots int // locals portion of FrameSlots (frame base offset)
	// CallSites, indexed by the id the exiting code leaves in Ctx.TrapInfo,
	// tell the host what to call and where to re-enter.
	CallSites []CallSite
}
