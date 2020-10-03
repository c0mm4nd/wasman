package stacks

const (
	initialLabelStackHeight = 10
)

type LabelStack struct {
	labels []*Label
	ptr    int
}

type Label struct {
	Arity          int
	EndPC          uint64
	ContinuationPC uint64
}

func NewLabelStack() *LabelStack {
	return &LabelStack{
		labels: make([]*Label, initialLabelStackHeight),
		ptr:    -1,
	}
}

func (s *LabelStack) GetPtr() int {
	return s.ptr
}

func (s *LabelStack) Pop() *Label {
	ret := s.labels[s.ptr]
	s.ptr--

	return ret
}

func (s *LabelStack) Push(val *Label) {
	if s.ptr+1 == len(s.labels) {
		// grow stack
		s.labels = append(s.labels, val)
	} else {
		s.labels[s.ptr+1] = val
	}

	s.ptr++
}
