package stacks

const (
	// InitialOperandStackHeight is the initial length of the OperandStack
	InitialOperandStackHeight = 1024
)

// NewOperandStack creates a new OperandStack with no limit
func NewOperandStack() *Stack[uint64] {
	return &Stack[uint64]{
		Values: make([]uint64, InitialOperandStackHeight),
		Ptr:    -1,
	}
}
