package stacks

type Stack[T any] struct {
	Values []T
	Ptr    int
}

func (s *Stack[T]) Push(val T) {
	if s.Ptr+1 == len(s.Values) {
		// grow stack
		s.Values = append(s.Values, val)
	} else {
		s.Values[s.Ptr+1] = val
	}

	s.Ptr++
}

// Pop will return the value on current Ptr, and backspace the Ptr
func (s *Stack[T]) Pop() T {
	ret := s.Values[s.Ptr]
	s.Ptr--
	return ret
}

// Drop is same to Pop but no return
func (s *Stack[T]) Drop() {
	s.Ptr--
}

// Peek will return the value on current Ptr like Pop but Ptr does not get backspace
func (s *Stack[T]) Peek() T {
	return s.Values[s.Ptr]
}

