package wasman

import (
	"github.com/c0mm4nd/wasman/types"
)

type fn interface {
	FuncType() *types.FuncType
	Call(ins *Instance)
}
