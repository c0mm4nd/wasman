package wasman

import (
	"reflect"
	"testing"

	"github.com/c0mm4nd/wasman/types"
)

// TODO: Add uint ptr tests

func Test_getTypeOf(t *testing.T) {
	for _, c := range []struct {
		kind reflect.Kind
		exp  types.ValueType
	}{
		{kind: reflect.Int32, exp: types.ValueTypeI32},
		{kind: reflect.Uint32, exp: types.ValueTypeI32},
		{kind: reflect.Int64, exp: types.ValueTypeI64},
		{kind: reflect.Uint64, exp: types.ValueTypeI64},
		{kind: reflect.Float32, exp: types.ValueTypeF32},
		{kind: reflect.Float64, exp: types.ValueTypeF64},
	} {
		actual, err := getTypeOf(c.kind)
		if err != nil {
			t.Fail()
		}
		if !reflect.DeepEqual(c.exp, actual) {
			t.Fail()
		}
	}
}

func Test_getSignature(t *testing.T) {
	v := reflect.ValueOf(func(int32, int64, float32, float64) (int32, float64) { return 0, 0 })
	actual, err := getSignature(v.Type())
	if err != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(&types.FuncType{
		InputTypes:  []types.ValueType{types.ValueTypeI32, types.ValueTypeI64, types.ValueTypeF32, types.ValueTypeF64},
		ReturnTypes: []types.ValueType{types.ValueTypeI32, types.ValueTypeF64},
	}, actual) {
		t.Fail()
	}
}
