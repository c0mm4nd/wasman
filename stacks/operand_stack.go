package stacks

const (
	InitialOperandStackHeight = 1024
)

// https://www.w3.org/TR/wasm-core-1/#stack
type OperandStack struct {
	Operands []uint64
	Ptr      int // current pointer on stack
}

func NewOperandStack() *OperandStack {
	return &OperandStack{
		Operands: make([]uint64, InitialOperandStackHeight),
		Ptr:      -1,
	}
}

func (s *OperandStack) Pop() uint64 {
	ret := s.Operands[s.Ptr]
	s.Ptr--
	return ret
}

func (s *OperandStack) Drop() {
	s.Ptr--
}

func (s *OperandStack) Peek() uint64 {
	return s.Operands[s.Ptr]
}

func (s *OperandStack) Push(val uint64) {
	if s.Ptr+1 == len(s.Operands) {
		// grow stack
		s.Operands = append(s.Operands, val)
	} else {
		s.Operands[s.Ptr+1] = val
	}

	s.Ptr++
}

func (s *OperandStack) PushBool(b bool) {
	if b {
		s.Push(1)
	} else {
		s.Push(0)
	}
}
