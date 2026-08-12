package stacks

const (
	// InitialLabelStackHeight is the initial length of stack
	InitialLabelStackHeight = 10
)

// Label acts as a signal on the workflow of the control instr
type Label struct {
	// Arity is the number of operand values kept when branching to this label
	// (a block/if result count, or a loop's parameter count).
	Arity int
	// Sp is the operand stack pointer captured when the block was entered, so a
	// branch can discard everything the block pushed and keep only Arity results.
	Sp             int
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
