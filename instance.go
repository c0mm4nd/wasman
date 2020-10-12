package wasman

import "github.com/c0mm4nd/wasman/wasm"

// Instance is same to wasm.Instance
type Instance = wasm.Instance

// NewInstance is a wrapper to the wasm.NewInstance
func NewInstance(module *Module, externModules map[string]*Module) (*Instance, error) {
	return wasm.NewInstance(module, externModules)
}
