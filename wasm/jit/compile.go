package jit

import (
	"errors"
	"runtime"
)

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
	// TollPrice is the per-opcode toll (uniform pricing); 0 disables inline
	// metering. When set, generated code charges it before every opcode and
	// traps on TollMax, matching the interpreter's per-op charge.
	TollPrice uint64
	// SelfIdx is this function's index-space position (host exits report
	// funcIdx<<32|siteID so the host can find the site table of whichever
	// frame exited); DepthLimit, when nonzero, bakes a call-depth check
	// into the prologue.
	SelfIdx    uint32
	DepthLimit uint64
	// TypeSigIDs maps type-section indices to instance-wide structural
	// signature ids (the call_indirect fast path compares them natively).
	TypeSigIDs []uint32
	// WideOps marks index-space entries that are built-in wide-integer
	// operations: calls to them inline as native carry chains instead of
	// host exits (0: not a wide op).
	WideOps []uint16
}

// Wide-integer intrinsic ids: 1 + op + width*32 (width 0: u128, 1: u256).
// The first ten (through WideMul) inline as native code in the optimizing
// tier; the rest dispatch through the reflection-free direct host path.
const (
	WideAdd = iota
	WideSub
	WideAnd
	WideOr
	WideXor
	WideNot
	WideIsZero
	WideCmpU
	WideCmpS
	WideMul
	WideDivU
	WideDivS
	WideRemU
	WideRemS
	WideShl
	WideShrU
	WideShrS
)

// WideOpID builds an intrinsic id; wide256 selects the 256-bit width.
func WideOpID(op int, wide256 bool) uint16 {
	id := uint16(1 + op)
	if wide256 {
		id += 32
	}
	return id
}

// WideOpKind splits an id into operation and width.
func WideOpKind(id uint16) (op int, wide256 bool) {
	return int(id-1) & 0x1f, id > 32
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
	SiteMemFill
	SiteMemCopy
)

// BrTable is a pre-decoded br_table: label depths per index plus a default.
type BrTable struct {
	Targets []uint32
	Def     uint32
}

// finishCompiled wraps a mapping in a Compiled whose executable pages are
// returned to the OS when the value becomes unreachable (compilation is
// per instance, so without this every NewInstance would leak RX pages).
func finishCompiled(cd *Compiled) *Compiled {
	runtime.SetFinalizer(cd, func(c *Compiled) { _ = Free(c.Code) })
	return cd
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

// chargeAfterOp reports whether an opcode is charged after its native code
// (trapping ops, whose trap path exits before the charge, and host exits,
// which charge at the re-entry continuation). All other opcodes charge
// before, so back-edges and branch targets skip the structural charge.
func chargeAfterOp(op byte) bool {
	switch {
	case op == 0x00: // unreachable
		return true
	case op == 0x10 || op == 0x11 || op == 0x40 || op == 0xfc: // call/indirect/grow/bulk
		return true
	case op >= 0x28 && op <= 0x3e: // memory loads and stores
		return true
	case op == 0x6d || op == 0x6e || op == 0x6f || op == 0x70: // i32 div/rem
		return true
	case op == 0x7f || op == 0x80 || op == 0x81 || op == 0x82: // i64 div/rem
		return true
	case op >= 0xa8 && op <= 0xb1: // trapping float->int truncations
		return true
	}
	return false
}
