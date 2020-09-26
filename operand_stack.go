package wasman

const (
	initialOperandStackHeight = 1024
)

// https://www.w3.org/TR/wasm-core-1/#stack
type operandStack struct {
	Stack []uint64
	SP    int
}

func newOperandStack() *operandStack {
	return &operandStack{
		Stack: make([]uint64, initialOperandStackHeight),
		SP:    -1,
	}
}

func (s *operandStack) pop() uint64 {
	ret := s.Stack[s.SP]
	s.SP--
	return ret
}

func (s *operandStack) drop() {
	s.SP--
}

func (s *operandStack) peek() uint64 {
	return s.Stack[s.SP]
}

func (s *operandStack) push(val uint64) {
	if s.SP+1 == len(s.Stack) {
		// grow stack
		s.Stack = append(s.Stack, val)
	} else {
		s.Stack[s.SP+1] = val
	}
	s.SP++
}

func (s *operandStack) pushBool(b bool) {
	if b {
		s.push(1)
	} else {
		s.push(0)
	}
}
