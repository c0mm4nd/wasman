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

type fn interface {
	getType() *types.FuncType
	call(ins *Instance) error
}
