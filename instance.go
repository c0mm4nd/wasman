package wasman

import "github.com/c0mm4nd/wasman/wasm"

type Instance = wasm.Instance

func NewInstance(module *wasm.Module, externModules map[string]*wasm.Module) (*wasm.Instance, error) {
	return wasm.NewInstance(module, externModules)
}
