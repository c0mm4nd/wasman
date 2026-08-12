package wasm

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/leb128decode"
	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
	"github.com/c0mm4nd/wasman/utils"
)

// errors on exec func
var (
	ErrExportedFuncNotFound = errors.New("exported func is not found")
	ErrFuncIndexOutOfRange  = errors.New("function index out of range")
	ErrInvalidArgNum        = errors.New("invalid number of arguments")
	ErrUnknownOpcode        = errors.New("unknown opcode")
)

func (ins *Instance) execExpr(expression *expr.Expression) (v interface{}, err error) {
	if expression.Extended {
		return ins.evalConstExpr(expression.Raw)
	}

	r := bytes.NewReader(expression.Data)
	switch expression.OpCode {
	case expr.OpCodeI32Const:
		v, _, err = leb128decode.DecodeInt32(r)
		if err != nil {
			return nil, fmt.Errorf("read int32: %w", err)
		}
	case expr.OpCodeI64Const:
		v, _, err = leb128decode.DecodeInt64(r)
		if err != nil {
			return nil, fmt.Errorf("read int64: %w", err)
		}
	case expr.OpCodeF32Const:
		v, err = utils.ReadFloat32(r)
		if err != nil {
			return nil, fmt.Errorf("read f34: %w", err)
		}
	case expr.OpCodeF64Const:
		v, err = utils.ReadFloat64(r)
		if err != nil {
			return nil, fmt.Errorf("read f64: %w", err)
		}
	case expr.OpCodeGlobalGet:
		id, _, err := leb128decode.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read index of global: %w", err)
		}
		if uint32(len(ins.IndexSpace.Globals)) <= id {
			return nil, fmt.Errorf("global index out of range")
		}
		v = ins.IndexSpace.Globals[id].Val
	default:
		return nil, fmt.Errorf("invalid opt code: %#x", expression.OpCode)
	}
	return v, nil
}

// evalConstExpr evaluates an extended constant expression (i32/i64/f32/f64
// const, global.get, and i32/i64 add/sub/mul) on a small typed stack.
func (ins *Instance) evalConstExpr(raw []byte) (interface{}, error) {
	r := bytes.NewReader(raw)
	stack := make([]interface{}, 0, 4)
	push := func(v interface{}) { stack = append(stack, v) }

	for r.Len() > 0 {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		switch expr.OpCode(b) {
		case expr.OpCodeI32Const:
			v, _, err := leb128decode.DecodeInt32(r)
			if err != nil {
				return nil, fmt.Errorf("read i32: %w", err)
			}
			push(v)
		case expr.OpCodeI64Const:
			v, _, err := leb128decode.DecodeInt64(r)
			if err != nil {
				return nil, fmt.Errorf("read i64: %w", err)
			}
			push(v)
		case expr.OpCodeF32Const:
			v, err := utils.ReadFloat32(r)
			if err != nil {
				return nil, fmt.Errorf("read f32: %w", err)
			}
			push(v)
		case expr.OpCodeF64Const:
			v, err := utils.ReadFloat64(r)
			if err != nil {
				return nil, fmt.Errorf("read f64: %w", err)
			}
			push(v)
		case expr.OpCodeGlobalGet:
			id, _, err := leb128decode.DecodeUint32(r)
			if err != nil {
				return nil, fmt.Errorf("read global index: %w", err)
			}
			if uint32(len(ins.IndexSpace.Globals)) <= id {
				return nil, fmt.Errorf("global index out of range")
			}
			push(ins.IndexSpace.Globals[id].Val)
		case expr.OpCodeI32Add, expr.OpCodeI32Sub, expr.OpCodeI32Mul,
			expr.OpCodeI64Add, expr.OpCodeI64Sub, expr.OpCodeI64Mul:
			if len(stack) < 2 {
				return nil, fmt.Errorf("const expression stack underflow")
			}
			b2 := stack[len(stack)-1]
			a2 := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			res, err := constArith(expr.OpCode(b), a2, b2)
			if err != nil {
				return nil, err
			}
			push(res)
		default:
			return nil, fmt.Errorf("invalid const opcode: %#x", b)
		}
	}

	if len(stack) != 1 {
		return nil, fmt.Errorf("const expression left %d values on the stack", len(stack))
	}
	return stack[0], nil
}

func constArith(op expr.OpCode, a, b interface{}) (interface{}, error) {
	switch op {
	case expr.OpCodeI32Add, expr.OpCodeI32Sub, expr.OpCodeI32Mul:
		x, ok1 := a.(int32)
		y, ok2 := b.(int32)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("i32 const arith on non-i32 operands")
		}
		switch op {
		case expr.OpCodeI32Add:
			return x + y, nil
		case expr.OpCodeI32Sub:
			return x - y, nil
		default:
			return x * y, nil
		}
	default: // i64
		x, ok1 := a.(int64)
		y, ok2 := b.(int64)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("i64 const arith on non-i64 operands")
		}
		switch op {
		case expr.OpCodeI64Add:
			return x + y, nil
		case expr.OpCodeI64Sub:
			return x - y, nil
		default:
			return x * y, nil
		}
	}
}

func (ins *Instance) execFunc() error {
	for ; int(ins.Active.PC) < len(ins.Active.Func.body); ins.Active.PC++ {
		opByte := ins.Active.Func.body[ins.Active.PC]
		op := expr.OpCode(opByte)
		instr := instructions[op]
		if instr == nil {
			return fmt.Errorf("%w: %#x", ErrUnknownOpcode, opByte)
		}
		err := instr(ins)
		if err != nil {
			return err
		}

		// Toll
		if ins.Module.ModuleConfig.TollStation != nil {
			price := ins.TollStation.GetOpPrice(op)
			err := ins.TollStation.AddToll(price)
			if err != nil {
				return err
			}
		}

		if op == expr.OpCodeReturn {
			return nil
		}
	}

	return nil
}

// CallExportedFunc will call the func `name` with the args
// TODO: enhance this
func (ins *Instance) CallExportedFunc(name string, args ...uint64) (returns []uint64, returnTypes []types.ValueType, err error) {
	exp, ok := ins.Module.ExportSection[name]
	if !ok || exp.Desc.Kind != segments.KindFunction {
		return nil, nil, ErrExportedFuncNotFound
	}

	if int(exp.Desc.Index) >= len(ins.Functions) {
		return nil, nil, ErrFuncIndexOutOfRange
	}

	f := ins.Functions[exp.Desc.Index]
	if len(f.getType().InputTypes) != len(args) {
		return nil, nil, ErrInvalidArgNum
	}

	// on any error, restore the operand stack to its pre-call height so a
	// trapped call leaves the instance reusable with no leaked values
	baseSp := ins.OperandStack.Ptr

	for i := range args {
		ins.OperandStack.Push(args[i])
	}

	err = f.call(ins)
	if err != nil {
		ins.OperandStack.Ptr = baseSp
		return nil, nil, err
	}

	ret := make([]uint64, len(f.getType().ReturnTypes))
	for i := range ret {
		ret[len(ret)-1-i] = ins.OperandStack.Pop()
	}

	return ret, f.getType().ReturnTypes, nil
}
