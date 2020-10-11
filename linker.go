package wasman

import (
	"errors"
	"fmt"
	"github.com/c0mm4nd/wasman/wasm"
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
	Modules map[string]*wasm.Module // the built-in modules which acts as externs when instantiating coming main module
}

// NewLinker creates a new Linker
func NewLinker() *Linker {
	return &Linker{map[string]*wasm.Module{}}
}

// NewLinkerWithModuleMap creates a new Linker with the built-in modules
func NewLinkerWithModuleMap(in map[string]*wasm.Module) *Linker {
	return &Linker{in}
}

// Define put the module on its namespace
func (l *Linker) Define(modName string, mod *wasm.Module) {
	l.Modules[modName] = mod
}

// AdvancedFunc is a advanced host func comparing to normal go host func
// Dev will be able to handle the pre/post-call process of the func and manipulate
// the Instance's fields like memory
//
// e.g. when we wanna add toll after calling the host func f
//	func ExampleFuncGenerator_addToll() {
//		var linker = wasman.NewLinker()
//		var f = func() {fmt.Println("wasm")}
//
//		var af = wasman.AdvancedFunc(func(ins *wasman.Instance) interface{} {
//			return func() {
//				f()
//				ins.AddGas(11)
//			}
//		})
//		linker.DefineAdvancedFunc("env", "add_gas", af)
//	}
//
// e.g. when we wanna manipulate memory
//	func ExampleFuncGenerator_addToll() {
//		var linker = wasman.NewLinker()
//
//		var af = wasman.AdvancedFunc(func(ins *wasman.Instance) interface{} {
//			return func(ptr uint32, length uint32) {
//				msg := ins.Memory[int(ptr), int(ptr+uint32)]
//				fmt.Println(b)
//			}
//		})
//
//		linker.DefineAdvancedFunc("env", "print_msg", af)
//	}
type AdvancedFunc func(ins *wasm.Instance) interface{}

// DefineAdvancedFunc will define a AdvancedFunc on linker
func (l *Linker) DefineAdvancedFunc(modName, funcName string, funcGenerator AdvancedFunc) error {
	mod, exists := l.Modules[modName]
	if !exists {
		mod = &wasm.Module{IndexSpace: new(wasm.IndexSpace), ExportsSection: map[string]*segments.ExportSegment{}}
		l.Modules[modName] = mod
	}

	mod.ExportsSection[funcName] = &segments.ExportSegment{
		Name: funcName,
		Desc: &segments.ExportDesc{
			Kind:  segments.ExportKindFunction,
			Index: uint32(len(mod.IndexSpace.Functions)),
		},
	}

	sig, err := getSignature(reflect.ValueOf(funcGenerator(&wasm.Instance{})).Type())
	if err != nil {
		return ErrInvalidSign
	}

	mod.IndexSpace.Functions = append(mod.IndexSpace.Functions, &wasm.HostFunc{
		Generator: funcGenerator,
		Signature: sig,
	})

	return nil
}

// DefineFunc puts a simple go style func into Linker's modules.
// This f should be a simply func which doesnt handle ins's fields.
func (l *Linker) DefineFunc(modName, funcName string, f interface{}) error {
	fn := func(ins *wasm.Instance) interface{} {
		return f
	}

	mod, exists := l.Modules[modName]
	if !exists {
		mod = &wasm.Module{IndexSpace: new(wasm.IndexSpace), ExportsSection: map[string]*segments.ExportSegment{}}
		l.Modules[modName] = mod
	}

	mod.ExportsSection[funcName] = &segments.ExportSegment{
		Name: funcName,
		Desc: &segments.ExportDesc{
			Kind:  segments.ExportKindFunction,
			Index: uint32(len(mod.IndexSpace.Functions)),
		},
	}

	sig, err := getSignature(reflect.ValueOf(f).Type())
	if err != nil {
		return ErrInvalidSign
	}

	mod.IndexSpace.Functions = append(mod.IndexSpace.Functions, &wasm.HostFunc{
		Generator: fn,
		Signature: sig,
	})

	return nil
}

func (l *Linker) DefineGlobal(modName, globalName string, global interface{}) error {
	// TODO
	return nil
}

// Instantiate will instantiate a Module into an runnable Instance
func (l *Linker) Instantiate(mainModule *wasm.Module) (*wasm.Instance, error) {
	return wasm.NewInstance(mainModule, l.Modules)
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

const is64Bit = uint64(^uintptr(0)) == ^uint64(0)

// getTypeOf converts the go type into wasm val type
func getTypeOf(kind reflect.Kind) (types.ValueType, error) {
	if is64Bit {
		switch kind {
		case reflect.Float64:
			return types.ValueTypeF64, nil
		case reflect.Float32:
			return types.ValueTypeF32, nil
		case reflect.Int32, reflect.Uint32:
			return types.ValueTypeI32, nil
		case reflect.Int64, reflect.Uint64, reflect.Uintptr, reflect.UnsafePointer, reflect.Ptr:
			return types.ValueTypeI64, nil
		default:
			return 0x00, fmt.Errorf("invalid type: %s", kind.String())
		}
	} else {
		switch kind {
		case reflect.Float64:
			return types.ValueTypeF64, nil
		case reflect.Float32:
			return types.ValueTypeF32, nil
		case reflect.Int32, reflect.Uint32, reflect.Uintptr, reflect.UnsafePointer, reflect.Ptr:
			return types.ValueTypeI32, nil
		case reflect.Int64, reflect.Uint64:
			return types.ValueTypeI64, nil
		default:
			return 0x00, fmt.Errorf("invalid type: %s", kind.String())
		}
	}

}
