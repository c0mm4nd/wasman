package wasm

import "github.com/c0mm4nd/wasman/types"

// Global is an instance of the global value
type Global struct {
	*types.GlobalType
	Val interface{}
}
