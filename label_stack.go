package wasman

const (
	initialLabelStackHeight = 10
)

type labelStack struct {
	Stack []*label
	SP    int
}

type label struct {
	Arity                 int
	EndPC uint64
	ContinuationPC  uint64
}

func newLabelStack() *labelStack {
	return &labelStack{
		Stack: make([]*label, initialLabelStackHeight),
		SP:    -1,
	}
}

func (s *labelStack) pop() *label {
	ret := s.Stack[s.SP]
	s.SP--

	return ret
}

func (s *labelStack) push(val *label) {
	if s.SP+1 == len(s.Stack) {
		// grow stack
		s.Stack = append(s.Stack, val)
	} else {
		s.Stack[s.SP+1] = val
	}

	s.SP++
}
