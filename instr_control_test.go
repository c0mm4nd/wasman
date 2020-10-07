package wasman

import (
	"github.com/c0mm4nd/wasman/instr"
	"github.com/c0mm4nd/wasman/stacks"
	"github.com/c0mm4nd/wasman/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

type dummyFunc struct {
	cnt int
}

func (d *dummyFunc) call(_ *Instance) error {
	d.cnt++
	return nil
}

func (d *dummyFunc) getType() *types.FuncType { return &types.FuncType{} }

func Test_call(t *testing.T) {
	df := &dummyFunc{}
	ins := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				Body: []byte{byte(instr.OpCodeCall), 0x01},
			},
		},
		Functions: []fn{nil, df},
	}

	assert.NoError(t, call(ins))
	assert.Equal(t, 1, df.cnt)
}

func Test_callIndirect(t *testing.T) {
	df := &dummyFunc{}
	ins := &Instance{
		Context: &wasmContext{
			Func: &wasmFunc{
				Body: []byte{byte(instr.OpCodeCall), 0x01, 0x00},
			},
		},
		Functions: []fn{nil, df},
		Module: &Module{
			TypesSection: []*types.FuncType{nil, {}},
			indexSpace: &indexSpace{
				Tables: [][]*uint32{{nil, uint32Ptr(1)}},
			},
		},
		OperandStack: stacks.NewOperandStack(),
	}
	ins.OperandStack.Push(1)

	assert.NoError(t, callIndirect(ins))
	assert.Equal(t, 1, df.cnt)
}
