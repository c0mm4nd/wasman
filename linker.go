package wasman

import (
	"fmt"
	"reflect"

	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
)

type Linker struct {
	Modules map[string]*Module
}

func NewLinker() *Linker {
	return &Linker{map[string]*Module{}}
}

func NewLinkerWithModuleMap(in map[string]*Module) *Linker {
	return &Linker{in}
}

func (l *Linker) Define(modName string, mod *Module) {
	l.Modules[modName] = mod
}

func (l *Linker) DefineFunction(modName, funcName string, fn func(ins *Instance) reflect.Value) error {
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

	sig, err := getSignature(fn(&Instance{}).Type())
	if err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	mod.indexSpace.Functions = append(mod.indexSpace.Functions, &goFunc{
		ClosureGenerator: fn,
		Signature:        sig,
	})

	return nil
}

func (l *Linker) Instantiate(mainModule *Module, config *InstanceConfig) (*Instance, error) {
	return NewInstance(mainModule, l.Modules, config)
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
