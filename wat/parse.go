package wat

import "errors"

// node is one S-expression: either an atom (tok set) or a list.
type node struct {
	atom *token
	list []node
}

func (n *node) isList() bool { return n.atom == nil }

// head returns the leading keyword of a list node, or "".
func (n *node) head() string {
	if n.isList() && len(n.list) > 0 && !n.list[0].isList() && n.list[0].atom.kind == tokKeyword {
		return n.list[0].atom.text
	}
	return ""
}

// parseSExprs builds the S-expression forest for a token stream.
func parseSExprs(toks []token) ([]node, error) {
	var stack [][]node
	var cur []node
	for i := range toks {
		t := &toks[i]
		switch t.kind {
		case tokLParen:
			stack = append(stack, cur)
			cur = nil
		case tokRParen:
			if len(stack) == 0 {
				return nil, errors.New("unbalanced ')'")
			}
			parent := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			parent = append(parent, node{list: cur})
			cur = parent
		default:
			cur = append(cur, node{atom: t})
		}
	}
	if len(stack) != 0 {
		return nil, errors.New("unbalanced '('")
	}
	return cur, nil
}
