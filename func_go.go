package wasman

import (
	"math"
	"reflect"

	"github.com/c0mm4nd/wasman/types"
)

type goFunc struct {
	Signature *types.FuncType

	ClosureGenerator func(ins *Instance) reflect.Value
	function         reflect.Value // should be set at the time of wasm instance creation
}

func (f *goFunc) FuncType() *types.FuncType {
	return f.Signature
}

func (f *goFunc) Call(ins *Instance) {
	tp := f.function.Type()
	in := make([]reflect.Value, tp.NumIn())
	for i := len(in) - 1; i >= 0; i-- {
		val := reflect.New(tp.In(i)).Elem()
		raw := ins.OperandStack.pop()
		kind := tp.In(i).Kind()

		switch kind {
		case reflect.Float64, reflect.Float32:
			val.SetFloat(math.Float64frombits(raw))
		case reflect.Uint32, reflect.Uint64:
			val.SetUint(raw)
		case reflect.Int32, reflect.Int64:
			val.SetInt(int64(raw))
		default:
			panic("invalid input type")
		}
		in[i] = val
	}

	for _, ret := range f.function.Call(in) {
		switch ret.Kind() {
		case reflect.Float64, reflect.Float32:
			ins.OperandStack.push(math.Float64bits(ret.Float()))
		case reflect.Uint32, reflect.Uint64:
			ins.OperandStack.push(ret.Uint())
		case reflect.Int32, reflect.Int64:
			ins.OperandStack.push(uint64(ret.Int()))
		default:
			panic("invalid return type")
		}
	}
}
