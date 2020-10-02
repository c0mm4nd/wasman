package wasman

import (
	"github.com/c0mm4nd/wasman/types"
)

type fn interface {
	getType() *types.FuncType
	call(ins *Instance) error
}
