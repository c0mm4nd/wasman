package wasman

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/c0mm4nd/wasman/instr"
	"github.com/c0mm4nd/wasman/leb128"
	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
)

// errors on exec func
var (
	ErrExportedFuncNotFound = errors.New("exported func is not found")
	ErrFuncIndexOutOfRange  = errors.New("function index out of range")
	ErrInvalidArgNum        = errors.New("invalid number of arguments")
)

func (ins *Instance) execExpr(expression *instr.Expr) (v interface{}, err error) {
	r := bytes.NewBuffer(expression.Data)
	switch expression.OpCode {
	case instr.OpCodeI32Const:
		v, _, err = leb128.DecodeInt32(r)
		if err != nil {
			return nil, fmt.Errorf("read int32: %w", err)
		}
	case instr.OpCodeI64Const:
		v, _, err = leb128.DecodeInt64(r)
		if err != nil {
			return nil, fmt.Errorf("read int64: %w", err)
		}
	case instr.OpCodeF32Const:
		v, err = instr.ReadFloat32(r)
		if err != nil {
			return nil, fmt.Errorf("read f34: %w", err)
		}
	case instr.OpCodeF64Const:
		v, err = instr.ReadFloat32(r)
		if err != nil {
			return nil, fmt.Errorf("read f64: %w", err)
		}
	case instr.OpCodeGlobalGet:
		id, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read index of global: %w", err)
		}
		if uint32(len(ins.indexSpace.Globals)) <= id {
			return nil, fmt.Errorf("global index out of range")
		}
		v = ins.indexSpace.Globals[id].Val
	default:
		return nil, fmt.Errorf("invalid opt code: %#x", expression.OpCode)
	}
	return v, nil
}

func (ins *Instance) execFunc() error {
	for ; int(ins.Context.PC) < len(ins.Context.Func.Body); ins.Context.PC++ {
		opByte := ins.Context.Func.Body[ins.Context.PC]
		op := instr.OpCode(opByte)
		err := instructions[op](ins)
		if err != nil {
			return err
		}

		// Toll
		if ins.TollStation != nil {
			price := ins.TollStation.GetOpPrice(op)
			err := ins.TollStation.AddToll(price)
			if err != nil {
				return err
			}
		}

		if op == instr.OpCodeReturn {
			return nil
		}
	}

	return nil
}

// CallExportedFunc will call the func `name` with the args
// TODO: enhance this
func (ins *Instance) CallExportedFunc(name string, args ...uint64) (returns []uint64, returnTypes []types.ValueType, err error) {
	exp, ok := ins.Module.ExportsSection[name]
	if !ok || exp.Desc.Kind != segments.ExportKindFunction {
		return nil, nil, ErrExportedFuncNotFound
	}

	if int(exp.Desc.Index) >= len(ins.Functions) {
		return nil, nil, ErrFuncIndexOutOfRange
	}

	f := ins.Functions[exp.Desc.Index]
	if len(f.getType().InputTypes) != len(args) {
		return nil, nil, ErrInvalidArgNum
	}

	for i := range args {
		ins.OperandStack.Push(args[i])
	}

	err = f.call(ins)
	if err != nil {
		return nil, nil, err
	}

	ret := make([]uint64, len(f.getType().ReturnTypes))
	for i := range ret {
		ret[len(ret)-1-i] = ins.OperandStack.Pop()
	}

	return ret, f.getType().ReturnTypes, nil
}
