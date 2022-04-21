package stacks

const (
	// InitialLabelStackHeight is the initial length of stack
	InitialLabelStackHeight = 10
)

// Label acts as a signal on the workflow of the control instr
type Label struct {
	Arity          int
	EndPC          uint64
	ContinuationPC uint64
}

// NewLabelStack creates a new LabelStack
func NewLabelStack() *Stack[*Label] {
	return &Stack[*Label]{
		Values: make([]*Label, InitialLabelStackHeight),
		Ptr:    -1,
	}
}
