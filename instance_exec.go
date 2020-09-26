package wasman

import (
	"bytes"
	"fmt"

	"github.com/c0mm4nd/wasman/instr"
	"github.com/c0mm4nd/wasman/leb128"
	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
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

func (ins *Instance) execFunc() {
	for ; int(ins.Context.PC) < len(ins.Context.Func.Body); ins.Context.PC++ {
		opByte := ins.Context.Func.Body[ins.Context.PC]
		switch op := instr.OpCode(opByte); op {
		case instr.OpCodeReturn:
			return
		default:
			instructions[op](ins)

			// Toll
			if ins.TollStation != nil {
				err := ins.TollStation.AddToll(op)
				if err != nil {
					panic(err) // TODO: avoid panic
				}
			}
		}
	}
}

func (ins *Instance) CallExportedFunc(name string, args ...uint64) (returns []uint64, returnTypes []types.ValueType, err error) {
	exp, ok := ins.Module.ExportsSection[name]
	if !ok {
		return nil, nil, fmt.Errorf("exported func of name %s not found", name)
	}

	if exp.Desc.Kind != segments.ExportKindFunction {
		return nil, nil, fmt.Errorf("exported elent of name %s is not functype", name)
	}

	if int(exp.Desc.Index) >= len(ins.Functions) {
		return nil, nil, fmt.Errorf("function index out of range")
	}

	f := ins.Functions[exp.Desc.Index]
	if len(f.FuncType().InputTypes) != len(args) {
		return nil, nil, fmt.Errorf("invalid number of arguments")
	}

	for _, arg := range args {
		ins.OperandStack.push(arg)
	}

	f.Call(ins)

	ret := make([]uint64, len(f.FuncType().ReturnTypes))
	for i := range ret {
		ret[len(ret)-1-i] = ins.OperandStack.pop()
	}
	return ret, f.FuncType().ReturnTypes, nil
}
