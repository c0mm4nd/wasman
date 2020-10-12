package wasman

import (
	"errors"
	"fmt"
	"github.com/c0mm4nd/wasman/config"
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
	config.LinkerConfig

	Modules map[string]*Module // the built-in modules which acts as externs when instantiating coming main module
}

// NewLinker creates a new Linker
func NewLinker(config config.LinkerConfig) *Linker {
	return &Linker{
		LinkerConfig: config,
		Modules:      map[string]*Module{},
	}
}

// NewLinkerWithModuleMap creates a new Linker with the built-in modules
func NewLinkerWithModuleMap(config config.LinkerConfig, in map[string]*Module) *Linker {
	return &Linker{
		LinkerConfig: config,
		Modules:      in,
	}
}

// Define put the module on its namespace
func (l *Linker) Define(modName string, mod *Module) {
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
type AdvancedFunc func(ins *Instance) interface{}

// DefineAdvancedFunc will define a AdvancedFunc on linker
func (l *Linker) DefineAdvancedFunc(modName, funcName string, funcGenerator AdvancedFunc) error {
	sig, err := getSignature(reflect.ValueOf(funcGenerator(&Instance{})).Type())
	if err != nil {
		return ErrInvalidSign
	}

	mod, exists := l.Modules[modName]
	if !exists {
		mod = &Module{IndexSpace: new(wasm.IndexSpace), ExportSection: map[string]*segments.ExportSegment{}}
		l.Modules[modName] = mod
	}

	if l.DisableShadowing && mod.ExportSection[funcName] != nil {
		return config.ErrShadowing
	}

	mod.ExportSection[funcName] = &segments.ExportSegment{
		Name: funcName,
		Desc: &segments.ExportDesc{
			Kind:  segments.KindFunction,
			Index: uint32(len(mod.IndexSpace.Functions)),
		},
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
	fn := func(ins *Instance) interface{} {
		return f
	}

	sig, err := getSignature(reflect.ValueOf(f).Type())
	if err != nil {
		return ErrInvalidSign
	}

	mod, exists := l.Modules[modName]
	if !exists {
		mod = &Module{IndexSpace: new(wasm.IndexSpace), ExportSection: map[string]*segments.ExportSegment{}}
		l.Modules[modName] = mod
	}

	if l.DisableShadowing && mod.ExportSection[funcName] != nil {
		return config.ErrShadowing
	}

	mod.ExportSection[funcName] = &segments.ExportSegment{
		Name: funcName,
		Desc: &segments.ExportDesc{
			Kind:  segments.KindFunction,
			Index: uint32(len(mod.IndexSpace.Functions)),
		},
	}

	mod.IndexSpace.Functions = append(mod.IndexSpace.Functions, &wasm.HostFunc{
		Generator: fn,
		Signature: sig,
	})

	return nil
}

// DefineGlobal will defined an external global for the main module
func (l *Linker) DefineGlobal(modName, globalName string, global interface{}) error {
	ty, err := getTypeOf(reflect.TypeOf(global).Kind())
	if err != nil {
		return err
	}

	mod, exists := l.Modules[modName]
	if !exists {
		mod = &Module{IndexSpace: new(wasm.IndexSpace), ExportSection: map[string]*segments.ExportSegment{}}
		l.Modules[modName] = mod
	}

	if l.DisableShadowing && mod.ExportSection[globalName] != nil {
		return config.ErrShadowing
	}

	mod.ExportSection[globalName] = &segments.ExportSegment{
		Name: globalName,
		Desc: &segments.ExportDesc{
			Kind:  segments.KindGlobal,
			Index: uint32(len(mod.IndexSpace.Globals)),
		},
	}

	mod.IndexSpace.Globals = append(mod.IndexSpace.Globals, &wasm.Global{
		Type: &types.GlobalType{
			ValType: ty,
			Mutable: true,
		},
		Val: global,
	})

	return nil
}

// DefineTable will defined an external table for the main module
func (l *Linker) DefineTable(modName, tableName string, table []*uint32) error {
	mod, exists := l.Modules[modName]
	if !exists {
		mod = &Module{IndexSpace: new(wasm.IndexSpace), ExportSection: map[string]*segments.ExportSegment{}}
		l.Modules[modName] = mod
	}

	if l.DisableShadowing && mod.ExportSection[tableName] != nil {
		return config.ErrShadowing
	}

	mod.ExportSection[tableName] = &segments.ExportSegment{
		Name: tableName,
		Desc: &segments.ExportDesc{
			Kind:  segments.KindTable,
			Index: uint32(len(mod.IndexSpace.Tables)),
		},
	}

	mod.IndexSpace.Tables = append(mod.IndexSpace.Tables, table)

	return nil
}

// DefineMemory will defined an external memory for the main module
func (l *Linker) DefineMemory(modName, memName string, mem []byte) error {
	mod, exists := l.Modules[modName]
	if !exists {
		mod = &Module{IndexSpace: new(wasm.IndexSpace), ExportSection: map[string]*segments.ExportSegment{}}
		l.Modules[modName] = mod
	}

	if l.DisableShadowing && mod.ExportSection[memName] != nil {
		return config.ErrShadowing
	}

	mod.ExportSection[memName] = &segments.ExportSegment{
		Name: memName,
		Desc: &segments.ExportDesc{
			Kind:  segments.KindMem,
			Index: uint32(len(mod.IndexSpace.Memories)),
		},
	}

	mod.IndexSpace.Memories = append(mod.IndexSpace.Memories, mem)

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
