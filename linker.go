package wasman

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
)

// errors on linking modules
var (
	ErrInvalidSign = errors.New("invalid signature")
)

// Linker is a helper to instantiate new modules
type Linker struct {
	Modules map[string]*Module // the built-in modules which acts as externs when instantiating coming main module
}

// NewLinker creates a new Linker
func NewLinker() *Linker {
	return &Linker{map[string]*Module{}}
}

// NewLinkerWithModuleMap creates a new Linker with the built-in modules
func NewLinkerWithModuleMap(in map[string]*Module) *Linker {
	return &Linker{in}
}

// Define put the module on its namespace
func (l *Linker) Define(modName string, mod *Module) {
	l.Modules[modName] = mod
}

// FuncGenerator is a advanced host func comparing to normal go host func
// Dev will be able to handle the pre/post-call process of the func
// e.g. when we wanna add toll after calling the host func f
//	func ExampleFuncGenerator_addToll() {
//		var f = func() {fmt.Println("wasm")}
//
//		var fg = wasman.FuncGenerator(func(ins *wasman.Instance) interface{} {
//			return func() {
//				f()
//				ins.AddGas(11)
//			}
//		})
//		// Then use wasman.DefineFuncGenerator
//	}
type FuncGenerator = func(ins *Instance) interface{}

// DefineFuncGenerator will define a FuncGenerator on linker
func (l *Linker) DefineFuncGenerator(modName, funcName string, funcGenerator FuncGenerator) error {
	mod, exists := l.Modules[modName]
	if !exists {
		mod = &Module{indexSpace: new(indexSpace), ExportsSection: map[string]*segments.ExportSegment{}}
		l.Modules[modName] = mod
	}

	mod.ExportsSection[funcName] = &segments.ExportSegment{
		Name: funcName,
		Desc: &segments.ExportDesc{
			Kind:  segments.ExportKindFunction,
			Index: uint32(len(mod.indexSpace.Functions)),
		},
	}

	sig, err := getSignature(reflect.ValueOf(funcGenerator(&Instance{})).Type())
	if err != nil {
		return ErrInvalidSign
	}

	mod.indexSpace.Functions = append(mod.indexSpace.Functions, &hostFunc{
		generator: funcGenerator,
		signature: sig,
	})

	return nil
}

// DefineFunc puts a go style func into linker's modules
func (l *Linker) DefineFunc(modName, funcName string, f interface{}) error {
	fn := func(ins *Instance) interface{} {
		return f
	}

	mod, exists := l.Modules[modName]
	if !exists {
		mod = &Module{indexSpace: new(indexSpace), ExportsSection: map[string]*segments.ExportSegment{}}
		l.Modules[modName] = mod
	}

	mod.ExportsSection[funcName] = &segments.ExportSegment{
		Name: funcName,
		Desc: &segments.ExportDesc{
			Kind:  segments.ExportKindFunction,
			Index: uint32(len(mod.indexSpace.Functions)),
		},
	}

	sig, err := getSignature(reflect.ValueOf(f).Type())
	if err != nil {
		return ErrInvalidSign
	}

	mod.indexSpace.Functions = append(mod.indexSpace.Functions, &hostFunc{
		generator: fn,
		signature: sig,
	})

	return nil
}

// Instantiate will instantiate a Module into an runnable Instance
func (l *Linker) Instantiate(mainModule *Module) (*Instance, error) {
	return NewInstance(mainModule, l.Modules)
}

func getSignature(p reflect.Type) (*types.FuncType, error) {
	var err error
	in := make([]types.ValueType, p.NumIn())
	for i := range in {
		in[i], err = getTypeOf(p.In(i).Kind())
		if err != nil {
			return nil, err
		}
	}

	out := make([]types.ValueType, p.NumOut())
	for i := range out {
		out[i], err = getTypeOf(p.Out(i).Kind())
		if err != nil {
			return nil, err
		}
	}
	return &types.FuncType{InputTypes: in, ReturnTypes: out}, nil
}

func getTypeOf(kind reflect.Kind) (types.ValueType, error) {
	switch kind {
	case reflect.Float64:
		return types.ValueTypeF64, nil
	case reflect.Float32:
		return types.ValueTypeF32, nil
	case reflect.Int32, reflect.Uint32:
		return types.ValueTypeI32, nil
	case reflect.Int64, reflect.Uint64:
		return types.ValueTypeI64, nil
	default:
		return 0x00, fmt.Errorf("invalid type: %s", kind.String())
	}
}
