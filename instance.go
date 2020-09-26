package wasman

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/c0mm4nd/wasman/leb128"
)

// Instance is an instantiated module
type Instance struct {
	*Module

	Context   *wasmContext
	Functions []fn
	Memory    []byte
	Globals   []uint64

	OperandStack *operandStack
}


func NewInstance(module *Module, externModules map[string]*Module, config *InstanceConfig) (*Instance, error) {
	ins := &Instance{
		Module:       module,
		OperandStack: newOperandStack(),
	}

	if err := ins.buildIndexSpaces(externModules); err != nil {
		return nil, fmt.Errorf("build index space: %w", err)
	}

	if config != nil {
		// parse config
	}

	// initializing memory
	ins.Memory = ins.Module.indexSpace.Memories[0]
	if diff := uint64(ins.Module.MemorySection[0].Min)*defaultPageSize - uint64(len(ins.Memory)); diff > 0 {
		ins.Memory = append(ins.Memory, make([]byte, diff)...)
	}

	// initializing functions
	ins.Functions = make([]fn, len(ins.Module.indexSpace.Functions))
	for i, f := range ins.Module.indexSpace.Functions {
		if wasmFn, ok := f.(*goFunc); ok {
			wasmFn.function = wasmFn.ClosureGenerator(ins)
			ins.Functions[i] = wasmFn
		} else {
			ins.Functions[i] = f
		}
	}

	// initialize global
	ins.Globals = make([]uint64, len(ins.Module.indexSpace.Globals))
	for i, raw := range ins.Module.indexSpace.Globals {
		switch v := raw.Val.(type) {
		case int32:
			ins.Globals[i] = uint64(v)
		case int64:
			ins.Globals[i] = uint64(v)
		case float32:
			ins.Globals[i] = uint64(math.Float32bits(v))
		case float64:
			ins.Globals[i] = math.Float64bits(v)
		}
	}

	// exec start functions
	for _, id := range ins.Module.StartSection {
		if int(id) >= len(ins.Functions) {
			return nil, fmt.Errorf("function index out of range")
		}

		ins.Functions[id].Call(ins)
	}

	return ins, nil
}

func (ins *Instance) fetchInt32() int32 {
	ret, num, err := leb128.DecodeInt32(bytes.NewBuffer(
		ins.Context.Func.Body[ins.Context.PC:]))
	if err != nil {
		panic(err)
	}
	ins.Context.PC += num - 1
	return ret
}

func (ins *Instance) fetchUint32() uint32 {
	ret, num, err := leb128.DecodeUint32(bytes.NewBuffer(
		ins.Context.Func.Body[ins.Context.PC:]))
	if err != nil {
		panic(err)
	}
	ins.Context.PC += num - 1
	return ret
}

func (ins *Instance) fetchInt64() int64 {
	ret, num, err := leb128.DecodeInt64(bytes.NewBuffer(
		ins.Context.Func.Body[ins.Context.PC:]))
	if err != nil {
		panic(err)
	}
	ins.Context.PC += num - 1
	return ret
}

func (ins *Instance) fetchFloat32() float32 {
	v := math.Float32frombits(binary.LittleEndian.Uint32(
		ins.Context.Func.Body[ins.Context.PC:]))
	ins.Context.PC += 3
	return v
}

func (ins *Instance) fetchFloat64() float64 {
	v := math.Float64frombits(binary.LittleEndian.Uint64(
		ins.Context.Func.Body[ins.Context.PC:]))
	ins.Context.PC += 7
	return v
}
