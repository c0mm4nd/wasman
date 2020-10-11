package stacks

const (
	// InitialLabelStackHeight is the initial length of stack
	InitialLabelStackHeight = 10
)

// LabelStack is the stack of Label. https://www.w3.org/TR/wasm-core-1/#labels%E2%91%A0
type LabelStack struct {
	Labels []*Label
	Ptr    int
}

// Label acts as a signal on the workflow of the control instr
type Label struct {
	Arity          int
	EndPC          uint64
	ContinuationPC uint64
}

// NewLabelStack creates a new LabelStack
func NewLabelStack() *LabelStack {
	return &LabelStack{
		Labels: make([]*Label, InitialLabelStackHeight),
		Ptr:    -1,
	}
}

// Pop will return the Label on current Ptr, and backspace the Ptr
func (s *LabelStack) Pop() *Label {
	ret := s.Labels[s.Ptr]
	s.Ptr--

	return ret
}

// Push will push one Label into the stack on the next Ptr
func (s *LabelStack) Push(val *Label) {
	if s.Ptr+1 == len(s.Labels) {
		// grow stack
		s.Labels = append(s.Labels, val)
	} else {
		s.Labels[s.Ptr+1] = val
	}

	s.Ptr++
}
