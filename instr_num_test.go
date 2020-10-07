package wasman

import (
	"github.com/c0mm4nd/wasman/stacks"
	"github.com/stretchr/testify/suite"
	"testing"
)

type NumTestSuite struct {
	suite.Suite
	vm *Instance
}

func (suite *NumTestSuite) SetupTest() {
	suite.vm = &Instance{
		OperandStack: stacks.NewOperandStack(),
	}
}

func (suite *NumTestSuite) Test_i32eqz() {
	var testTable = []struct {
		input int
		want  uint64
	}{
		{input: 0, want: 1},
		{input: 1, want: 0},
	}
	for _, tt := range testTable {
		suite.vm.OperandStack.Push(uint64(tt.input))
		suite.NoError(i32eqz(suite.vm))
		suite.Equal(tt.want, suite.vm.OperandStack.Pop())
	}
}

func (suite *NumTestSuite) Test_i32ne() {
	var testTable = []struct {
		input [2]int
		want  uint64
	}{
		{input: [2]int{3, 4}, want: 1},
		{input: [2]int{3, 3}, want: 0},
	}
	for _, tt := range testTable {
		suite.vm.OperandStack.Push(uint64(tt.input[0]))
		suite.vm.OperandStack.Push(uint64(tt.input[1]))
		suite.NoError(i32ne(suite.vm))
		suite.Equal(tt.want, suite.vm.OperandStack.Pop())
	}
}

func (suite *NumTestSuite) Test_i32lts() {
	var testTable = []struct {
		input [2]int
		want  uint64
	}{
		{input: [2]int{-4, 1}, want: 1},
		{input: [2]int{4, -1}, want: 0},
	}
	for _, tt := range testTable {
		suite.vm.OperandStack.Push(uint64(tt.input[0]))
		suite.vm.OperandStack.Push(uint64(tt.input[1]))
		suite.NoError(i32lts(suite.vm))
		suite.Equal(tt.want, suite.vm.OperandStack.Pop())
	}
}

func (suite *NumTestSuite) Test_i32ltu() {
	var testTable = []struct {
		input [2]int
		want  uint64
	}{
		{input: [2]int{1, 4}, want: 1},
		{input: [2]int{4, 1}, want: 0},
	}
	for _, tt := range testTable {
		suite.vm.OperandStack.Push(uint64(tt.input[0]))
		suite.vm.OperandStack.Push(uint64(tt.input[1]))
		suite.NoError(i32ltu(suite.vm))
		suite.Equal(tt.want, suite.vm.OperandStack.Pop())
	}
}

func (suite *NumTestSuite) Test_i32gts() {
	var testTable = []struct {
		input [2]int
		want  uint64
	}{
		{input: [2]int{1, -4}, want: 1},
		{input: [2]int{-4, 1}, want: 0},
	}
	for _, tt := range testTable {
		suite.vm.OperandStack.Push(uint64(tt.input[0]))
		suite.vm.OperandStack.Push(uint64(tt.input[1]))
		suite.NoError(i32gts(suite.vm))
		suite.Equal(tt.want, suite.vm.OperandStack.Pop())
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRunSuite(t *testing.T) {
	suite.Run(t, new(NumTestSuite))
}
