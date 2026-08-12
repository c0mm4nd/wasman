package wat

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// isNumToken reports whether text matches the spec's numeric token grammar:
// an optional sign followed by an integer or float form (decimal or hex),
// with underscores allowed only between digits, plus inf / nan / nan:0xh.
func isNumToken(text string) bool {
	s := text
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		s = s[1:]
	}
	if s == "inf" || s == "nan" {
		return true
	}
	if strings.HasPrefix(s, "nan:0x") {
		return isHexNum(s[6:])
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return isHexFloat(s[2:])
	}
	return isDecFloat(s)
}

// digits '_'-separated: digit (('_')? digit)*
func scanDigits(s string, hex bool) (rest string, ok bool) {
	isD := func(c byte) bool {
		if c >= '0' && c <= '9' {
			return true
		}
		if !hex {
			return false
		}
		return (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
	}
	if len(s) == 0 || !isD(s[0]) {
		return s, false
	}
	i := 1
	for i < len(s) {
		if isD(s[i]) {
			i++
		} else if s[i] == '_' && i+1 < len(s) && isD(s[i+1]) {
			i += 2
		} else {
			break
		}
	}
	return s[i:], true
}

func isHexNum(s string) bool {
	rest, ok := scanDigits(s, true)
	return ok && rest == ""
}

// decimal float: num ('.' num?)? (('e'|'E') sign? num)?
func isDecFloat(s string) bool {
	rest, ok := scanDigits(s, false)
	if !ok {
		return false
	}
	s = rest
	if len(s) > 0 && s[0] == '.' {
		s = s[1:]
		if len(s) > 0 && s[0] != 'e' && s[0] != 'E' {
			s, ok = scanDigits(s, false)
			if !ok {
				return false
			}
		}
	}
	if len(s) > 0 && (s[0] == 'e' || s[0] == 'E') {
		s = s[1:]
		if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
			s = s[1:]
		}
		s, ok = scanDigits(s, false)
		if !ok {
			return false
		}
	}
	return s == ""
}

// hex float (after 0x): hexnum ('.' hexnum?)? (('p'|'P') sign? num)?
func isHexFloat(s string) bool {
	rest, ok := scanDigits(s, true)
	if !ok {
		return false
	}
	s = rest
	if len(s) > 0 && s[0] == '.' {
		s = s[1:]
		if len(s) > 0 && s[0] != 'p' && s[0] != 'P' {
			s, ok = scanDigits(s, true)
			if !ok {
				return false
			}
		}
	}
	if len(s) > 0 && (s[0] == 'p' || s[0] == 'P') {
		s = s[1:]
		if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
			s = s[1:]
		}
		s, ok = scanDigits(s, false)
		if !ok {
			return false
		}
	}
	return s == ""
}

// isIntToken reports whether text is an integer literal (no float parts).
func isIntToken(text string) bool {
	s := text
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		s = s[1:]
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return isHexNum(s[2:])
	}
	rest, ok := scanDigits(s, false)
	return ok && rest == ""
}

// parseInt parses an integer literal of width bits, accepting either the
// signed or the unsigned range: [-2^(bits-1), 2^bits - 1].
func parseInt(text string, bits uint) (uint64, error) {
	if !isIntToken(text) {
		return 0, fmt.Errorf("not an integer literal: %q", text)
	}
	s := strings.ReplaceAll(text, "_", "")
	neg := false
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	base := 10
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		base = 16
		s = s[2:]
	}
	v, err := strconv.ParseUint(s, base, 64)
	if err != nil {
		return 0, errors.New("constant out of range")
	}
	if neg {
		// -v must fit in the signed range
		limit := uint64(1) << (bits - 1)
		if v > limit {
			return 0, errors.New("constant out of range")
		}
		return uint64(-int64(v)), nil
	}
	if bits < 64 && v >= uint64(1)<<bits {
		return 0, errors.New("constant out of range")
	}
	return v, nil
}

// parseUint parses an unsigned integer literal of width bits (indices, limits,
// alignment and offset values).
func parseUint(text string, bits uint) (uint64, error) {
	if len(text) > 0 && (text[0] == '+' || text[0] == '-') {
		return 0, fmt.Errorf("unexpected sign in %q", text)
	}
	return parseInt(text, bits)
}

// checkFloat validates a float literal (of width 32 or 64 bits), including
// the range of decimal/hex values and nan payloads.
func checkFloat(text string, bits uint) error {
	s := text
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		s = s[1:]
	}
	if s == "inf" || s == "nan" {
		return nil
	}
	if strings.HasPrefix(s, "nan:0x") {
		if !isHexNum(s[6:]) {
			return fmt.Errorf("malformed nan payload %q", text)
		}
		payload, err := strconv.ParseUint(strings.ReplaceAll(s[6:], "_", ""), 16, 64)
		if err != nil {
			return errors.New("constant out of range")
		}
		significand := uint(23)
		if bits == 64 {
			significand = 52
		}
		if payload == 0 || payload >= uint64(1)<<significand {
			return errors.New("constant out of range")
		}
		return nil
	}
	if !isNumToken(text) {
		return fmt.Errorf("not a float literal: %q", text)
	}
	clean := strings.ReplaceAll(text, "_", "")
	// Go requires an exponent-less hex float to be spelled with 'p0'
	if strings.HasPrefix(strings.TrimLeft(clean, "+-"), "0x") &&
		!strings.ContainsAny(clean, "pP") {
		clean += "p0"
	}
	// strconv rounds to the requested width and reports overflow as ErrRange
	if _, err := strconv.ParseFloat(clean, int(bits)); err != nil {
		return errors.New("constant out of range")
	}
	return nil
}
