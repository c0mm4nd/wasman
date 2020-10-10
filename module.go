package wasman

import (
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/wasm"
	"io"
)

type Module = wasm.Module

func NewModule(r io.Reader, config *config.ModuleConfig) (*wasm.Module, error) {
	return wasm.NewModule(r, config)
}
