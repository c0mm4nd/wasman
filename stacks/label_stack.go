package stacks

const (
	InitialLabelStackHeight = 10
)

type LabelStack struct {
	Labels []*Label
	Ptr    int
}

type Label struct {
	Arity          int
	EndPC          uint64
	ContinuationPC uint64
}

func NewLabelStack() *LabelStack {
	return &LabelStack{
		Labels: make([]*Label, InitialLabelStackHeight),
		Ptr:    -1,
	}
}

func (s *LabelStack) Pop() *Label {
	ret := s.Labels[s.Ptr]
	s.Ptr--

	return ret
}

func (s *LabelStack) Push(val *Label) {
	if s.Ptr+1 == len(s.Labels) {
		// grow stack
		s.Labels = append(s.Labels, val)
	} else {
		s.Labels[s.Ptr+1] = val
	}

	s.Ptr++
}
