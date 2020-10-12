package wasman

import (
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/wasm"
	"io"
)

// Module is same to wasm.Module
type Module = wasm.Module

// NewInstance is a wrapper to the wasm.NewModule
func NewModule(config config.ModuleConfig, r io.Reader) (*Module, error) {
	return wasm.NewModule(config, r)
}
