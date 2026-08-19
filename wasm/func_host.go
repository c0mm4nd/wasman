package wasm

import (
	"fmt"
	"math"
	"reflect"

	"github.com/c0mm4nd/wasman/types"
)

// HostFunc is an implement of wasm.Fn,
// which represents all the functions defined under host(golang) environment
type HostFunc struct {
	Signature *types.FuncType // the shape of func (defined by inputs and outputs)

	// Generator is a func defined by other dev which acts as a Generator to the function
	// (generate when NewInstance's func initializing
	Generator func(ins *Instance) interface{}

	// function is the generated func from Generator, should be set at the time of wasm instance creation
	function interface{}

	// wideOp marks a built-in wide-integer operation the optimizing tier
	// may inline as native code instead of a host call (0: none).
	wideOp uint16
}

// SetWideOp marks this host function as an inlinable wide-integer
// operation (used by the built-in u128/u256 modules).
func (f *HostFunc) SetWideOp(id uint16) { f.wideOp = id }

func (f *HostFunc) getType() *types.FuncType {
	return f.Signature
}

func (f *HostFunc) call(ins *Instance) error {
	if f.wideOp != 0 {
		// built-in wide-integer operations skip the reflection machinery
		return wideDirect(ins, f.wideOp)
	}
	if f.function == nil {
		// an unbound host function reports instead of panicking in reflect
		return fmt.Errorf("host function called before binding")
	}
	fnVal := reflect.ValueOf(f.function)
	ty := fnVal.Type()
	in := make([]reflect.Value, ty.NumIn())

	for i := len(in) - 1; i >= 0; i-- {
		val := reflect.New(ty.In(i)).Elem()
		raw := ins.OperandStack.Pop()
		kind := ty.In(i).Kind()

		switch kind {
		case reflect.Float64:
			val.SetFloat(math.Float64frombits(raw))
		case reflect.Float32:
			// f32 operand-stack slots hold the 32-bit pattern
			val.SetFloat(float64(math.Float32frombits(uint32(raw))))
		case reflect.Uint32, reflect.Uint64:
			val.SetUint(raw)
		case reflect.Int32, reflect.Int64:
			val.SetInt(int64(raw))
		default:
			return ErrFuncInvalidInputType
		}
		in[i] = val
	}

	results := fnVal.Call(in)

	// a trailing error return traps the caller instead of pushing a value
	if n := len(results); n > 0 && ty.Out(n-1) == errorReflectType {
		if e := results[n-1]; !e.IsNil() {
			return e.Interface().(error)
		}
		results = results[:n-1]
	}

	for _, val := range results {
		switch val.Kind() {
		case reflect.Float64:
			ins.OperandStack.Push(math.Float64bits(val.Float()))
		case reflect.Float32:
			ins.OperandStack.Push(uint64(math.Float32bits(float32(val.Float()))))
		case reflect.Uint32, reflect.Uint64:
			ins.OperandStack.Push(val.Uint())
		case reflect.Int32, reflect.Int64:
			ins.OperandStack.Push(uint64(val.Int()))
		default:
			return ErrFuncInvalidReturnType
		}
	}

	return nil
}

var errorReflectType = reflect.TypeOf((*error)(nil)).Elem()
