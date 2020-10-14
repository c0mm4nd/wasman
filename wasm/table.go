package wasm

import "github.com/c0mm4nd/wasman/types"

// Table is an instance of the table value
type Table struct {
	types.TableType
	Value []*uint32 // vec of addr to func
}
