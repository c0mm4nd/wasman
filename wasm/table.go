package wasm

import "github.com/c0mm4nd/wasman/types"

// Table is an instance of the table value.
//
// Value holds resolved function references (nil = uninitialized/null slot).
// Storing references rather than function indices is what makes shared
// (imported/exported) tables work: an index would be re-resolved in the
// calling module's function index space, which is wrong across modules.
type Table struct {
	types.TableType
	Value []fn
}
