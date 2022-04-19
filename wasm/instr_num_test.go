package wasm

import (
	"testing"

	"github.com/c0mm4nd/wasman/stacks"
)

type NumTestSet struct {
	vm *Instance
}

func (s *NumTestSet) SetupTest() {
	s.vm = &Instance{
		OperandStack: stacks.NewOperandStack(),
	}
}

func (s *NumTestSet) Test_i32eqz(t *testing.T) {
	var testTable = []struct {
		input int
		want  uint64
	}{
		{input: 0, want: 1},
		{input: 1, want: 0},
	}
	for _, tt := range testTable {
		s.vm.OperandStack.Push(uint64(tt.input))
		if i32eqz(s.vm) != nil {
			t.Fail()
		}
		if s.vm.OperandStack.Pop() != tt.want {
			t.Fail()
		}
	}
}

func (s *NumTestSet) Test_i32ne(t *testing.T) {
	var testTable = []struct {
		input [2]int
		want  uint64
	}{
		{input: [2]int{3, 4}, want: 1},
		{input: [2]int{3, 3}, want: 0},
	}
	for _, tt := range testTable {
		s.vm.OperandStack.Push(uint64(tt.input[0]))
		s.vm.OperandStack.Push(uint64(tt.input[1]))
		if i32ne(s.vm) != nil {
			t.Fail()
		}
		if s.vm.OperandStack.Pop() != tt.want {
			t.Fail()
		}
	}
}

func (s *NumTestSet) Test_i32lts(t *testing.T) {
	var testTable = []struct {
		input [2]int
		want  uint64
	}{
		{input: [2]int{-4, 1}, want: 1},
		{input: [2]int{4, -1}, want: 0},
	}
	for _, tt := range testTable {
		s.vm.OperandStack.Push(uint64(tt.input[0]))
		s.vm.OperandStack.Push(uint64(tt.input[1]))
		if i32lts(s.vm) != nil {
			t.Fail()
		}
		if s.vm.OperandStack.Pop() != tt.want {
			t.Fail()
		}
	}
}

func (s *NumTestSet) Test_i32ltu(t *testing.T) {
	var testTable = []struct {
		input [2]int
		want  uint64
	}{
		{input: [2]int{1, 4}, want: 1},
		{input: [2]int{4, 1}, want: 0},
	}
	for _, tt := range testTable {
		s.vm.OperandStack.Push(uint64(tt.input[0]))
		s.vm.OperandStack.Push(uint64(tt.input[1]))
		if i32ltu(s.vm) != nil {
			t.Fail()
		}
		if s.vm.OperandStack.Pop() != tt.want {
			t.Fail()
		}
	}
}

func (s *NumTestSet) Test_i32gts(t *testing.T) {
	var testTable = []struct {
		input [2]int
		want  uint64
	}{
		{input: [2]int{1, -4}, want: 1},
		{input: [2]int{-4, 1}, want: 0},
	}
	for _, tt := range testTable {
		s.vm.OperandStack.Push(uint64(tt.input[0]))
		s.vm.OperandStack.Push(uint64(tt.input[1]))
		if i32gts(s.vm) != nil {
			t.Fail()
		}
		if s.vm.OperandStack.Pop() != tt.want {
			t.Fail()
		}
	}
}

func TestRunSuite(t *testing.T) {
	set := new(NumTestSet)
	set.SetupTest()
	set.Test_i32eqz(t)
	set.Test_i32ne(t)
	set.Test_i32lts(t)
	set.Test_i32ltu(t)
	set.Test_i32gts(t)
}
