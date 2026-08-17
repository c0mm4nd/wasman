package wasm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sync/atomic"

	"github.com/c0mm4nd/wasman/config"

	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/stacks"

	"github.com/c0mm4nd/wasman/leb128decode"
)

// Instance is an instantiated module
type Instance struct {
	*Module

	// IndexSpace is this INSTANCE's own view of its index spaces. It shadows
	// the embedded Module's field: instantiating the same *Module twice must
	// not let the second instance's spaces leak into the first (the Module
	// keeps a copy of the most recent instantiation for import resolution).
	IndexSpace *IndexSpace

	Active     *Frame
	FrameStack *stacks.Stack[*Frame]

	Functions []fn
	Memory    *Memory
	// Globals points at the runtime storage cells of the instance's globals;
	// imported globals alias the exporter's cells (shared mutation).
	Globals []*uint64

	OperandStack *stacks.Stack[uint64]

	// reader is reused across fetch* calls to avoid allocating a new
	// bytes.Reader for every immediate-carrying instruction in the hot loop.
	// The zero value is ready to use after a Reset.
	reader bytes.Reader

	// nativeStack backs the in-stack frames of natively-called JIT code;
	// nativeEntries maps function index space slots to native entry points.
	// metered/tollMax: inline-metered JIT (baseline tier charges toll per
	// opcode, matching the interpreter, and traps at tollMax).
	metered       bool
	tollMax       uint64
	nativeStack   []uint64
	nativeEntries []uintptr
	nativeTop     int // first free slot while a chain is suspended in an exit
	// indirectMirror lets native code dispatch call_indirect without a host
	// exit: [len, {sigID<<32|needBytes, entry}...] for table 0. sigIDs
	// assigns instance-wide structural signature ids.
	indirectMirror []uint64
	sigIDs         map[string]uint32

	// interruptFlag is set (atomically, possibly from another goroutine) by
	// Interrupt and polled by the exec loop; opTick amortizes the atomic load.
	interruptFlag uint32
	opTick        uint32

	// post-instantiation snapshot for Reset: the owned memory's bytes and the
	// values of the instance's own (non-imported) global cells.
	memSnapshot     []byte
	globalSnapshot  []uint64
	importedGlobals int

	// canonNaN mirrors ModuleConfig.CanonicalizeNaNs (hot-loop friendly copy)
	canonNaN bool

	// framePool recycles call frames (with their label stacks and locals
	// backing arrays); instances are single-goroutine so no locking is needed.
	framePool []*Frame
}

// acquireFrame takes a recycled frame or makes a fresh one.
func (ins *Instance) acquireFrame() *Frame {
	if n := len(ins.framePool); n > 0 {
		fr := ins.framePool[n-1]
		ins.framePool = ins.framePool[:n-1]
		fr.PC = 0
		fr.LabelStack.Ptr = -1
		return fr
	}
	return &Frame{LabelStack: stacks.NewLabelStack()}
}

// releaseFrame returns a frame to the pool.
func (ins *Instance) releaseFrame(fr *Frame) {
	fr.Func = nil
	ins.framePool = append(ins.framePool, fr)
}

// Interrupt requests that the currently running (or next) execution on this
// instance stops with ErrInterrupted. It is safe to call from any goroutine.
// Note: a cross-module call executes on the callee's own instance; an
// interrupt takes effect there once control returns to this instance.
func (ins *Instance) Interrupt() {
	atomic.StoreUint32(&ins.interruptFlag, 1)
}

// NewInstance will instantiate the module with extern modules
func NewInstance(module *Module, externModules map[string]*Module) (*Instance, error) {
	ins := &Instance{
		Module:       module,
		OperandStack: stacks.NewOperandStack(),
		FrameStack: &stacks.Stack[*Frame]{
			Ptr:    -1,
			Values: make([]*Frame, stacks.InitialLabelStackHeight),
		},
	}

	ins.canonNaN = module.ModuleConfig.CanonicalizeNaNs

	if err := ins.buildIndexSpaces(externModules); err != nil {
		return nil, fmt.Errorf("build index space: %w", err)
	}

	// initializing memory (a module is not required to define or import one)
	if len(ins.IndexSpace.Memories) > 0 {
		ins.Memory = ins.IndexSpace.Memories[0]
		if hostMax := uint64(module.ModuleConfig.MaxMemoryPages); hostMax != 0 &&
			uint64(len(ins.Memory.Value))/uint64(config.DefaultMemoryPageSize) > hostMax {
			return nil, fmt.Errorf("memory size exceeds the host limit of %d pages", hostMax)
		}
		if len(ins.Module.MemorySection) > 0 {
			if diff := uint64(ins.Module.MemorySection[0].Min)*uint64(config.DefaultMemoryPageSize) - uint64(len(ins.Memory.Value)); diff > 0 {
				ins.Memory.Value = append(ins.Memory.Value, make([]byte, diff)...)
			}
		}
	}

	// initializing functions
	ins.Functions = make([]fn, len(ins.IndexSpace.Functions))
	for i, f := range ins.IndexSpace.Functions {
		if wasmFn, ok := f.(*HostFunc); ok {
			// every instance gets its OWN bound copy: mutating only the
			// shared HostFunc would re-bind the imports of every EARLIER
			// instance to the latest instance (and its memory) whenever
			// several modules import the same host module
			ins.Functions[i] = &HostFunc{
				Signature: wasmFn.Signature,
				Generator: wasmFn.Generator,
				function:  wasmFn.Generator(ins),
			}
			// the shared object stays callable for the paths resolving
			// through the raw index space (e.g. table elements); those
			// keep the pre-existing last-binder semantics
			wasmFn.function = wasmFn.Generator(ins)
		} else {
			ins.Functions[i] = f
		}
	}

	// initialize globals: collect the shared storage cells (imported globals
	// keep aliasing the exporter's cell)
	ins.Globals = make([]*uint64, len(ins.IndexSpace.Globals))
	for i, raw := range ins.IndexSpace.Globals {
		ins.Globals[i] = raw.ensureCell()
	}

	// exec start functions
	for _, id := range ins.Module.StartSection {
		if int(id) >= len(ins.Functions) {
			return nil, ErrFuncIndexOutOfRange
		}

		// a start function must take no parameters and return nothing
		ft := ins.Functions[id].getType()
		if len(ft.InputTypes) != 0 || len(ft.ReturnTypes) != 0 {
			return nil, fmt.Errorf("invalid start function signature: must be [] -> []")
		}

		err := ins.Functions[id].call(ins)
		if err != nil {
			return nil, err
		}
	}

	// snapshot the post-instantiation state (after data segments and start
	// functions) so Reset can restore it. Imported memories/globals are shared
	// with their exporter and are deliberately NOT snapshotted.
	ownsMemory := len(module.MemorySection) > 0
	if ins.Memory != nil && ownsMemory {
		ins.memSnapshot = make([]byte, len(ins.Memory.Value))
		copy(ins.memSnapshot, ins.Memory.Value)
	}
	for _, imp := range module.ImportSection {
		if imp.Desc.Kind == segments.KindGlobal {
			ins.importedGlobals++
		}
	}
	ins.globalSnapshot = make([]uint64, 0, len(ins.Globals)-ins.importedGlobals)
	for i := ins.importedGlobals; i < len(ins.Globals); i++ {
		ins.globalSnapshot = append(ins.globalSnapshot, *ins.Globals[i])
	}

	return ins, nil
}

// Reset restores the instance to its post-instantiation state: the owned
// linear memory (content and size) and the instance's own globals. Imported
// memories and globals are shared with their exporter and are left untouched.
// Useful for pooling: one instance can serve many isolated runs without
// paying for re-instantiation.
func (ins *Instance) Reset() {
	if ins.memSnapshot != nil && ins.Memory != nil {
		if cap(ins.Memory.Value) >= len(ins.memSnapshot) {
			ins.Memory.Value = ins.Memory.Value[:len(ins.memSnapshot)]
		} else {
			ins.Memory.Value = make([]byte, len(ins.memSnapshot))
		}
		copy(ins.Memory.Value, ins.memSnapshot)
	}
	for i, v := range ins.globalSnapshot {
		*ins.Globals[ins.importedGlobals+i] = v
	}
	ins.OperandStack.Ptr = -1
	ins.FrameStack.Ptr = -1
	ins.Active = nil
	atomic.StoreUint32(&ins.interruptFlag, 0)
}

func (ins *Instance) fetchInt32() (int32, error) {
	ins.reader.Reset(ins.Active.Func.body[ins.Active.PC:])
	ret, num, err := leb128decode.DecodeInt32(&ins.reader)
	if err != nil {
		return 0, err
	}
	ins.Active.PC += num - 1

	return ret, nil
}

func (ins *Instance) fetchUint32() (uint32, error) {
	ins.reader.Reset(ins.Active.Func.body[ins.Active.PC:])
	ret, num, err := leb128decode.DecodeUint32(&ins.reader)
	if err != nil {
		return 0, err
	}

	ins.Active.PC += num - 1

	return ret, nil
}

func (ins *Instance) fetchInt64() (int64, error) {
	ins.reader.Reset(ins.Active.Func.body[ins.Active.PC:])
	ret, num, err := leb128decode.DecodeInt64(&ins.reader)
	if err != nil {
		return 0, err
	}

	ins.Active.PC += num - 1

	return ret, nil
}

func (ins *Instance) fetchFloat32() (float32, error) {
	v := math.Float32frombits(binary.LittleEndian.Uint32(
		ins.Active.Func.body[ins.Active.PC:]))
	ins.Active.PC += 3

	return v, nil
}

func (ins *Instance) fetchFloat64() (float64, error) {
	v := math.Float64frombits(binary.LittleEndian.Uint64(
		ins.Active.Func.body[ins.Active.PC:]))
	ins.Active.PC += 7

	return v, nil
}
