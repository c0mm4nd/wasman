package stacks

const (
	initialOperandStackHeight = 1024
)

// https://www.w3.org/TR/wasm-core-1/#stack
type OperandStack struct {
	operands []uint64
	ptr      int
}

func NewOperandStack() *OperandStack {
	return &OperandStack{
		operands: make([]uint64, initialOperandStackHeight),
		ptr:      -1,
	}
}

func (s *OperandStack) Pop() uint64 {
	ret := s.operands[s.ptr]
	s.ptr--
	return ret
}

func (s *OperandStack) Drop() {
	s.ptr--
}

func (s *OperandStack) Peek() uint64 {
	return s.operands[s.ptr]
}

func (s *OperandStack) Push(val uint64) {
	if s.ptr+1 == len(s.operands) {
		// grow stack
		s.operands = append(s.operands, val)
	} else {
		s.operands[s.ptr+1] = val
	}

	s.ptr++
}

func (s *OperandStack) PushBool(b bool) {
	if b {
		s.Push(1)
	} else {
		s.Push(0)
	}
}
