package wasman

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/wasm"
)

// Module is same to wasm.Module
type Module = wasm.Module

// NewModule is a wrapper to the wasm.NewModule
func NewModule(config config.ModuleConfig, r io.Reader) (*Module, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return wasm.NewModule(config, bytes.NewReader(b))
}
