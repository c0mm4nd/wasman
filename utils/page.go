package utils

func CalcPageSize(contentLen int, pageSize int) int {
	n := contentLen / pageSize
	if n*pageSize < contentLen {
		n++
	}

	return n
}
