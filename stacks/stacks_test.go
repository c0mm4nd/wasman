package stacks_test

import (
	"reflect"
	"testing"

	"github.com/c0mm4nd/wasman/stacks"
)

func TestVirtualMachineOperandStack(t *testing.T) {
	s := stacks.NewOperandStack()
	if stacks.InitialOperandStackHeight != len(s.Values) {
		t.Fail()
	}

	var exp uint64 = 10
	s.Push(exp)
	if exp != s.Pop() {
		t.Fail()
	}

	// verify the length grows
	for i := 0; i < stacks.InitialOperandStackHeight+1; i++ {
		s.Push(uint64(i))
	}
	if len(s.Values) <= stacks.InitialOperandStackHeight {
		t.Fail()
	}

	// verify the length is not shortened
	for i := 0; i < len(s.Values); i++ {
		_ = s.Pop()
	}

	if len(s.Values) <= stacks.InitialOperandStackHeight {
		t.Fail()
	}

	// for coverage OperandStack.Drop()
	// verify the length is not shortened
	for i := 0; i < len(s.Values); i++ {
		s.Drop()
	}

	if len(s.Values) <= stacks.InitialOperandStackHeight {
		t.Fail()
	}
}

func TestVirtualMachineLabelStack(t *testing.T) {
	s := stacks.NewLabelStack()
	if stacks.InitialLabelStackHeight != len(s.Values) {
		t.Fail()
	}

	exp := &stacks.Label{Arity: 100}
	s.Push(exp)
	if !reflect.DeepEqual(exp, s.Pop()) {
		t.Fail()
	}

	// verify the length grows
	for i := 0; i < stacks.InitialLabelStackHeight+1; i++ {
		s.Push(&stacks.Label{})
	}
	if len(s.Values) <= stacks.InitialLabelStackHeight {
		t.Fail()
	}

	// verify the length is not shortened
	for i := 0; i < len(s.Values); i++ {
		_ = s.Pop()
	}

	if len(s.Values) <= stacks.InitialLabelStackHeight {
		t.Fail()
	}
}
