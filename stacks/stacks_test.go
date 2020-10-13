package stacks_test

import (
	"testing"

	"github.com/c0mm4nd/wasman/stacks"
	"github.com/stretchr/testify/assert"
)

func TestVirtualMachineOperandStack(t *testing.T) {
	s := stacks.NewOperandStack()
	assert.Equal(t, stacks.InitialOperandStackHeight, len(s.Operands))

	var exp uint64 = 10
	s.Push(exp)
	assert.Equal(t, exp, s.Pop())

	// verify the length grows
	for i := 0; i < stacks.InitialOperandStackHeight+1; i++ {
		s.Push(uint64(i))
	}
	assert.True(t, len(s.Operands) > stacks.InitialOperandStackHeight)

	// verify the length is not shortened
	for i := 0; i < len(s.Operands); i++ {
		_ = s.Pop()
	}

	assert.True(t, len(s.Operands) > stacks.InitialOperandStackHeight)

	// for coverage OperandStack.Drop()
	// verify the length is not shortened
	for i := 0; i < len(s.Operands); i++ {
		s.Drop()
	}

	assert.True(t, len(s.Operands) > stacks.InitialOperandStackHeight)
}

func TestVirtualMachineLabelStack(t *testing.T) {
	s := stacks.NewLabelStack()
	assert.Equal(t, stacks.InitialLabelStackHeight, len(s.Labels))

	exp := &stacks.Label{Arity: 100}
	s.Push(exp)
	assert.Equal(t, exp, s.Pop())

	// verify the length grows
	for i := 0; i < stacks.InitialLabelStackHeight+1; i++ {
		s.Push(&stacks.Label{})
	}
	assert.True(t, len(s.Labels) > stacks.InitialLabelStackHeight)

	// verify the length is not shortened
	for i := 0; i < len(s.Labels); i++ {
		_ = s.Pop()
	}

	assert.True(t, len(s.Labels) > stacks.InitialLabelStackHeight)
}
