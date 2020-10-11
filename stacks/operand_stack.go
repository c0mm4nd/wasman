package stacks

const (
	// InitialOperandStackHeight is the initial length of the OperandStack
	InitialOperandStackHeight = 1024
)

// OperandStack is the stack of the operand. https://www.w3.org/TR/wasm-core-1/#stack
type OperandStack struct {
	Operands []uint64
	Ptr      int // current pointer on stack
}

// NewOperandStack creates a new OperandStack
func NewOperandStack() *OperandStack {
	return &OperandStack{
		Operands: make([]uint64, InitialOperandStackHeight),
		Ptr:      -1,
	}
}

// Pop will return the operand on current Ptr, and backspace the Ptr
func (s *OperandStack) Pop() uint64 {
	ret := s.Operands[s.Ptr]
	s.Ptr--
	return ret
}

// Drop is same to Pop but no return
func (s *OperandStack) Drop() {
	s.Ptr--
}

// Peek will return the operand on current Ptr like Pop but Ptr does not get backspace
func (s *OperandStack) Peek() uint64 {
	return s.Operands[s.Ptr]
}

// Push will push one operand into the stack on the next Ptr
func (s *OperandStack) Push(val uint64) {
	if s.Ptr+1 == len(s.Operands) {
		// grow stack
		s.Operands = append(s.Operands, val)
	} else {
		s.Operands[s.Ptr+1] = val
	}

	s.Ptr++
}

// PushBool will push one boolean operand into the stack on the next Ptr
func (s *OperandStack) PushBool(b bool) {
	if b {
		s.Push(1)
	} else {
		s.Push(0)
	}
}
