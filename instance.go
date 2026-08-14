package wasman

import "github.com/c0mm4nd/wasman/wasm"

// Instance is same to wasm.Instance
type Instance = wasm.Instance

// NewInstance is a wrapper to the wasm.NewInstance. When the module opts
// into the wide-integer extension, the built-in "u128"/"u256" host modules
// join the extern set (user-provided modules of the same name win).
func NewInstance(module *Module, externModules map[string]*Module) (*Instance, error) {
	if module.ModuleConfig.EnableWideInt {
		merged := make(map[string]*Module, len(externModules)+2)
		for k, v := range externModules {
			merged[k] = v
		}
		for k, v := range wideIntModules() {
			if _, exists := merged[k]; !exists {
				merged[k] = v
			}
		}
		externModules = merged
	}
	return wasm.NewInstance(module, externModules)
}
