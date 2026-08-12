// Package wat implements a reader for the WebAssembly text format: a
// spec-conformant lexer, S-expression parser and module grammar checker.
// It accepts exactly the module texts the spec calls well-formed and rejects
// malformed ones; it does not compile text to binary.
package wat

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

type tokenKind int

const (
	tokLParen tokenKind = iota
	tokRParen
	tokKeyword // starts with a lowercase letter, e.g. func, i32.const, offset=8
	tokID      // starts with '$'
	tokNum     // matches the numeric token grammar (int or float form)
	tokString  // a string literal; Str holds the DECODED bytes
	tokReserved
)

type token struct {
	kind tokenKind
	text string // raw source text (without quotes for strings)
	str  []byte // decoded bytes for tokString / string-form IDs
}

var errUnterminatedString = errors.New("unterminated string literal")

func isIDChar(c byte) bool {
	switch {
	case c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return true
	}
	switch c {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '/',
		':', '<', '=', '>', '?', '@', '\\', '^', '_', '`', '|', '~':
		return true
	}
	return false
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// lex tokenizes a whole source buffer. The source must be valid UTF-8 and
// tokens must be separated per the spec (e.g. a string immediately followed
// by an idchar is malformed).
func lex(src []byte) ([]token, error) {
	if !utf8.Valid(src) {
		return nil, errors.New("source is not valid UTF-8")
	}

	var toks []token
	i := 0
	n := len(src)

	// delimiterAt reports whether position i may legally follow a token.
	delimiterAt := func(i int) bool {
		if i >= n {
			return true
		}
		c := src[i]
		return isSpace(c) || c == '(' || c == ')' || c == ';'
	}

	for i < n {
		c := src[i]
		switch {
		case isSpace(c):
			i++

		case c == '(':
			if i+1 < n && src[i+1] == ';' { // block comment, may nest
				depth := 1
				j := i + 2
				for j < n && depth > 0 {
					if j+1 < n && src[j] == '(' && src[j+1] == ';' {
						depth++
						j += 2
					} else if j+1 < n && src[j] == ';' && src[j+1] == ')' {
						depth--
						j += 2
					} else {
						j++
					}
				}
				if depth != 0 {
					return nil, errors.New("unterminated block comment")
				}
				i = j
				continue
			}
			toks = append(toks, token{kind: tokLParen, text: "("})
			i++

		case c == ')':
			toks = append(toks, token{kind: tokRParen, text: ")"})
			i++

		case c == ';':
			if i+1 < n && src[i+1] == ';' { // line comment
				for i < n && src[i] != '\n' {
					i++
				}
				continue
			}
			return nil, errors.New("unexpected ';'")

		case c == '"':
			raw, decoded, next, err := lexString(src, i)
			if err != nil {
				return nil, err
			}
			if !delimiterAt(next) {
				return nil, fmt.Errorf("unexpected character after string: %q", src[next])
			}
			toks = append(toks, token{kind: tokString, text: raw, str: decoded})
			i = next

		case isIDChar(c):
			start := i
			for i < n && isIDChar(src[i]) {
				i++
			}
			text := string(src[start:i])
			// '$' immediately followed by a string is the string-form id
			if text == "$" && i < n && src[i] == '"' {
				raw, decoded, next, err := lexString(src, i)
				if err != nil {
					return nil, err
				}
				if !delimiterAt(next) {
					return nil, fmt.Errorf("unexpected character after id: %q", src[next])
				}
				if !utf8.Valid(decoded) {
					return nil, errors.New("id is not valid UTF-8")
				}
				toks = append(toks, token{kind: tokID, text: "$" + raw, str: decoded})
				i = next
				continue
			}
			if !delimiterAt(i) {
				return nil, fmt.Errorf("unexpected character after token %q", text)
			}
			toks = append(toks, classify(text))

		default:
			return nil, fmt.Errorf("unexpected character %q", c)
		}
	}

	return toks, nil
}

// classify determines the token kind of a bare atom.
func classify(text string) token {
	c := text[0]
	switch {
	case c == '$':
		if len(text) == 1 {
			return token{kind: tokReserved, text: text}
		}
		return token{kind: tokID, text: text}
	case c >= 'a' && c <= 'z':
		return token{kind: tokKeyword, text: text}
	case c >= '0' && c <= '9', c == '+', c == '-':
		if isNumToken(text) {
			return token{kind: tokNum, text: text}
		}
		return token{kind: tokReserved, text: text}
	default:
		return token{kind: tokReserved, text: text}
	}
}

// lexString reads a string literal starting at src[i] == '"'. It returns the
// raw text (without quotes), the decoded bytes, and the index just past the
// closing quote.
func lexString(src []byte, i int) (raw string, decoded []byte, next int, err error) {
	n := len(src)
	j := i + 1
	start := j
	for {
		if j >= n {
			return "", nil, 0, errUnterminatedString
		}
		c := src[j]
		if c == '"' {
			return string(src[start:j]), decoded, j + 1, nil
		}
		if c == '\\' {
			if j+1 >= n {
				return "", nil, 0, errUnterminatedString
			}
			e := src[j+1]
			switch e {
			case 't':
				decoded = append(decoded, '\t')
				j += 2
			case 'n':
				decoded = append(decoded, '\n')
				j += 2
			case 'r':
				decoded = append(decoded, '\r')
				j += 2
			case '"', '\'', '\\':
				decoded = append(decoded, e)
				j += 2
			case 'u':
				if j+2 >= n || src[j+2] != '{' {
					return "", nil, 0, errors.New("malformed \\u escape")
				}
				k := j + 3
				v := uint64(0)
				digits := 0
				for k < n && src[k] != '}' {
					d, ok := hexVal(src[k])
					if !ok {
						if src[k] == '_' && digits > 0 {
							k++
							continue
						}
						return "", nil, 0, errors.New("malformed \\u escape")
					}
					v = v*16 + uint64(d)
					if v > 0x10FFFF {
						return "", nil, 0, errors.New("unicode escape out of range")
					}
					digits++
					k++
				}
				if k >= n || digits == 0 {
					return "", nil, 0, errors.New("malformed \\u escape")
				}
				if v >= 0xD800 && v < 0xE000 {
					return "", nil, 0, errors.New("unicode escape is a surrogate")
				}
				var buf [4]byte
				sz := utf8.EncodeRune(buf[:], rune(v))
				decoded = append(decoded, buf[:sz]...)
				j = k + 1
			default:
				d1, ok1 := hexVal(e)
				if !ok1 || j+2 >= n {
					return "", nil, 0, fmt.Errorf("malformed escape \\%c", e)
				}
				d2, ok2 := hexVal(src[j+2])
				if !ok2 {
					return "", nil, 0, fmt.Errorf("malformed escape \\%c", e)
				}
				decoded = append(decoded, byte(d1*16+d2))
				j += 3
			}
			continue
		}
		if c < 0x20 || c == 0x7f {
			return "", nil, 0, fmt.Errorf("illegal control character %#x in string", c)
		}
		decoded = append(decoded, c)
		j++
	}
}

func hexVal(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10, true
	}
	return 0, false
}
