package wasm

import (
	"errors"

	"github.com/c0mm4nd/wasman/types"
)

// errors of func
var (
	ErrFuncInvalidInputType  = errors.New("invalid func input type")
	ErrFuncInvalidReturnType = errors.New("invalid func return type")
)

// fn is an instance of the func value
type fn interface {
	getType() *types.FuncType
	call(ins *Instance) error
}

// Fn is the exported alias of the function-instance interface, so external
// packages can reference table slots (e.g. sizing a Table's Value). Its
// implementations live in this package.
type Fn = fn
