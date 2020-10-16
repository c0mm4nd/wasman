package wasm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/c0mm4nd/wasman/config"

	"github.com/c0mm4nd/wasman/stacks"

	"github.com/c0mm4nd/wasman/leb128decode"
)

// Instance is an instantiated module
type Instance struct {
	*Module

	Context   *wasmContext
	Functions []fn
	Memory    *Memory
	Globals   []uint64

	*stacks.OperandStack
}

// NewInstance will instantiate the module with extern modules
func NewInstance(module *Module, externModules map[string]*Module) (*Instance, error) {
	ins := &Instance{
		Module:       module,
		OperandStack: stacks.NewOperandStack(),
	}

	if err := ins.buildIndexSpaces(externModules); err != nil {
		return nil, fmt.Errorf("build index space: %w", err)
	}

	// initializing memory
	ins.Memory = ins.Module.IndexSpace.Memories[0]
	if diff := uint64(ins.Module.MemorySection[0].Min)*config.DefaultPageSize - uint64(len(ins.Memory.Value)); diff > 0 {
		ins.Memory.Value = append(ins.Memory.Value, make([]byte, diff)...)
	}

	// initializing functions
	ins.Functions = make([]fn, len(ins.Module.IndexSpace.Functions))
	for i, f := range ins.Module.IndexSpace.Functions {
		if wasmFn, ok := f.(*HostFunc); ok {
			wasmFn.function = wasmFn.Generator(ins)
			ins.Functions[i] = wasmFn
		} else {
			ins.Functions[i] = f
		}
	}

	// initialize global
	ins.Globals = make([]uint64, len(ins.Module.IndexSpace.Globals))
	for i, raw := range ins.Module.IndexSpace.Globals {
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
			return nil, ErrFuncIndexOutOfRange
		}

		err := ins.Functions[id].call(ins)
		if err != nil {
			return nil, err
		}
	}

	return ins, nil
}

func (ins *Instance) fetchInt32() (int32, error) {
	ret, num, err := leb128decode.DecodeInt32(bytes.NewBuffer(
		ins.Context.Func.body[ins.Context.PC:]))
	if err != nil {
		return 0, err
	}
	ins.Context.PC += num - 1

	return ret, nil
}

func (ins *Instance) fetchUint32() (uint32, error) {
	ret, num, err := leb128decode.DecodeUint32(bytes.NewBuffer(
		ins.Context.Func.body[ins.Context.PC:]))
	if err != nil {
		return 0, err
	}

	ins.Context.PC += num - 1

	return ret, nil
}

func (ins *Instance) fetchInt64() (int64, error) {
	ret, num, err := leb128decode.DecodeInt64(bytes.NewBuffer(
		ins.Context.Func.body[ins.Context.PC:]))
	if err != nil {
		return 0, err
	}

	ins.Context.PC += num - 1

	return ret, nil
}

func (ins *Instance) fetchFloat32() (float32, error) {
	v := math.Float32frombits(binary.LittleEndian.Uint32(
		ins.Context.Func.body[ins.Context.PC:]))
	ins.Context.PC += 3

	return v, nil
}

func (ins *Instance) fetchFloat64() (float64, error) {
	v := math.Float64frombits(binary.LittleEndian.Uint64(
		ins.Context.Func.body[ins.Context.PC:]))
	ins.Context.PC += 7

	return v, nil
}
