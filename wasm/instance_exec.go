package wasm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync/atomic"

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
	// ErrInterrupted is returned when execution is stopped by Instance.Interrupt
	// (e.g. a context cancellation in CallExportedFuncWithContext).
	ErrInterrupted = errors.New("execution interrupted")
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

// nanCanonKind marks opcodes whose float result may carry a nondeterministic
// NaN payload (1 = f32 result, 2 = f64 result); with CanonicalizeNaNs set,
// such results are rewritten to the canonical NaN.
var nanCanonKind = buildNanCanonKind()

func buildNanCanonKind() (t [256]byte) {
	for op := 0x8d; op <= 0x97; op++ { // f32 ceil..max (excl. bitwise abs/neg/copysign)
		t[op] = 1
	}
	for op := 0x9b; op <= 0xa5; op++ { // f64 ceil..max
		t[op] = 2
	}
	t[0xb6] = 1 // f32.demote_f64
	t[0xbb] = 2 // f64.promote_f32
	return t
}

// opCharger is the optional single-call charging fast path a TollStation may
// implement (SimpleTollStation does); it must be equivalent to
// AddToll(GetOpPrice(op)).
type opCharger interface {
	ChargeOp(expr.OpCode) error
}

// errReturn is the internal sentinel the return opcode uses to unwind the
// exec loop without a per-instruction opcode comparison.
var errReturn = errors.New("wasm return")

func returnOp(_ *Instance) error { return errReturn }

func (ins *Instance) execFunc() error {
	// hoist the toll station (and its fast path) out of the hot loop
	ts := ins.Module.ModuleConfig.TollStation
	charger, fastToll := ts.(opCharger)
	// ins.Active is stable across instr() calls (nested calls restore it),
	// so the frame and body can be hoisted out of the loop
	frame := ins.Active
	body := frame.Func.body
	f := frame.Func
	os := ins.OperandStack
	// the inline fast path needs pre-decoded immediates and no per-op
	// bookkeeping (metering / NaN canonicalization)
	fast := f.imms != nil && ts == nil && !ins.canonNaN

	for ; int(frame.PC) < len(body); frame.PC++ {
		// poll the interrupt flag every 256 instructions: cheap enough for the
		// hot loop, responsive enough for timeouts
		ins.opTick++
		if ins.opTick&0xff == 0 && atomic.LoadUint32(&ins.interruptFlag) != 0 {
			return ErrInterrupted
		}

		op := expr.OpCode(body[frame.PC])

		// trap-free hot opcodes inlined: no indirect call, no error return
		if fast {
			switch op {
			case expr.OpCodeLocalGet:
				p := frame.PC
				os.Push(frame.Locals[f.imms[p]])
				frame.PC = uint64(f.pcEnd[p])
				continue
			case expr.OpCodeLocalSet:
				p := frame.PC
				frame.Locals[f.imms[p]] = os.Pop()
				frame.PC = uint64(f.pcEnd[p])
				continue
			case expr.OpCodeLocalTee:
				p := frame.PC
				frame.Locals[f.imms[p]] = os.Peek()
				frame.PC = uint64(f.pcEnd[p])
				continue
			case expr.OpCodeI32Const, expr.OpCodeI64Const, expr.OpCodeF32Const, expr.OpCodeF64Const:
				p := frame.PC
				os.Push(f.imms[p])
				frame.PC = uint64(f.pcEnd[p])
				continue
			case expr.OpCodeI32Add:
				v2 := os.Pop()
				os.Values[os.Ptr] = uint64(int32(os.Values[os.Ptr]) + int32(v2))
				continue
			case expr.OpCodeI32Sub:
				v2 := os.Pop()
				os.Values[os.Ptr] = uint64(int32(os.Values[os.Ptr]) - int32(v2))
				continue
			case expr.OpCodeI32Mul:
				v2 := os.Pop()
				os.Values[os.Ptr] = uint64(int32(os.Values[os.Ptr]) * int32(v2))
				continue
			case expr.OpCodeI32And:
				v2 := os.Pop()
				os.Values[os.Ptr] = uint64(uint32(os.Values[os.Ptr]) & uint32(v2))
				continue
			case expr.OpCodeI32Or:
				v2 := os.Pop()
				os.Values[os.Ptr] = uint64(uint32(os.Values[os.Ptr]) | uint32(v2))
				continue
			case expr.OpCodeI32Xor:
				v2 := os.Pop()
				os.Values[os.Ptr] = uint64(uint32(os.Values[os.Ptr]) ^ uint32(v2))
				continue
			case expr.OpCodeI32Eqz:
				if uint32(os.Values[os.Ptr]) == 0 {
					os.Values[os.Ptr] = 1
				} else {
					os.Values[os.Ptr] = 0
				}
				continue
			case expr.OpCodeI32LtU:
				v2 := os.Pop()
				if uint32(os.Values[os.Ptr]) < uint32(v2) {
					os.Values[os.Ptr] = 1
				} else {
					os.Values[os.Ptr] = 0
				}
				continue
			case expr.OpCodeI32GeU:
				v2 := os.Pop()
				if uint32(os.Values[os.Ptr]) >= uint32(v2) {
					os.Values[os.Ptr] = 1
				} else {
					os.Values[os.Ptr] = 0
				}
				continue
			case expr.OpCodeI64Add:
				v2 := os.Pop()
				os.Values[os.Ptr] += v2
				continue
			case expr.OpCodeI64Sub:
				v2 := os.Pop()
				os.Values[os.Ptr] -= v2
				continue
			case expr.OpCodeI64ExtendI32U:
				os.Values[os.Ptr] = uint64(uint32(os.Values[os.Ptr]))
				continue
			case expr.OpCodeI32WrapI64:
				os.Values[os.Ptr] = uint64(uint32(os.Values[os.Ptr]))
				continue
			}
		}

		instr := instructions[op]
		if instr == nil {
			return fmt.Errorf("%w: %#x", ErrUnknownOpcode, body[frame.PC])
		}
		err := instr(ins)
		if err != nil && err != errReturn {
			return err
		}

		if ins.canonNaN {
			switch nanCanonKind[op] {
			case 1:
				if v := uint32(ins.OperandStack.Peek()); v&0x7f800000 == 0x7f800000 && v&0x007fffff != 0 {
					ins.OperandStack.Values[ins.OperandStack.Ptr] = 0x7fc00000
				}
			case 2:
				if v := ins.OperandStack.Peek(); v&0x7ff0000000000000 == 0x7ff0000000000000 && v&0x000fffffffffffff != 0 {
					ins.OperandStack.Values[ins.OperandStack.Ptr] = 0x7ff8000000000000
				}
			}
		}

		// Toll
		if ts != nil {
			if fastToll {
				err = charger.ChargeOp(op)
			} else {
				err = ts.AddToll(ts.GetOpPrice(op))
			}
			if err != nil {
				return err
			}
		}

		if err == errReturn {
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

	// a fresh call starts with any stale interrupt request cleared
	atomic.StoreUint32(&ins.interruptFlag, 0)

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

	// an invalid body run with SkipValidation may return fewer values than its
	// signature promises; make that a clean error instead of a stack underflow
	ret := make([]uint64, len(f.getType().ReturnTypes))
	if ins.OperandStack.Ptr-baseSp < len(ret) {
		ins.OperandStack.Ptr = baseSp
		return nil, nil, fmt.Errorf("function %q returned too few values", name)
	}
	for i := range ret {
		ret[len(ret)-1-i] = ins.OperandStack.Pop()
	}

	return ret, f.getType().ReturnTypes, nil
}

// CallExportedFuncWithContext is CallExportedFunc bound to a context: when the
// context is cancelled or times out, execution is interrupted and the call
// returns an error wrapping ErrInterrupted.
func (ins *Instance) CallExportedFuncWithContext(ctx context.Context, name string, args ...uint64) ([]uint64, []types.ValueType, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			ins.Interrupt()
		case <-done:
		}
	}()

	rets, retTypes, err := ins.CallExportedFunc(name, args...)
	close(done)

	if err != nil && errors.Is(err, ErrInterrupted) && ctx.Err() != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInterrupted, ctx.Err())
	}
	return rets, retTypes, err
}
