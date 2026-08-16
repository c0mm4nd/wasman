package wat

import (
	"errors"
	"fmt"
	"strings"
)

// immKind classifies an instruction's immediates in text form.
type immKind int

const (
	immNone immKind = iota
	immLabel
	immBrTable
	immFunc
	immLocal
	immGlobal
	immCallIndirect
	immMemarg
	immConstI32
	immConstI64
	immConstF32
	immConstF64
	immMemFill // bulk memory.fill: one implicit memory-index byte, no text immediates
	immMemCopy // bulk memory.copy: two implicit memory-index bytes, no text immediates
)

type instrInfo struct {
	imm      immKind
	maxAlign uint64 // natural alignment for memarg ops (bytes)
}

var instrTable = buildInstrTable()

func buildInstrTable() map[string]instrInfo {
	t := map[string]instrInfo{}
	none := func(names ...string) {
		for _, n := range names {
			t[n] = instrInfo{imm: immNone}
		}
	}

	// parametric / control with no immediates
	none("unreachable", "nop", "return", "drop", "select")

	// control with immediates
	t["br"] = instrInfo{imm: immLabel}
	t["br_if"] = instrInfo{imm: immLabel}
	t["br_table"] = instrInfo{imm: immBrTable}
	t["call"] = instrInfo{imm: immFunc}
	t["call_indirect"] = instrInfo{imm: immCallIndirect}

	// variables
	t["local.get"] = instrInfo{imm: immLocal}
	t["local.set"] = instrInfo{imm: immLocal}
	t["local.tee"] = instrInfo{imm: immLocal}
	t["global.get"] = instrInfo{imm: immGlobal}
	t["global.set"] = instrInfo{imm: immGlobal}

	// memory
	mem := func(name string, align uint64) { t[name] = instrInfo{imm: immMemarg, maxAlign: align} }
	mem("i32.load", 4)
	mem("i64.load", 8)
	mem("f32.load", 4)
	mem("f64.load", 8)
	mem("i32.load8_s", 1)
	mem("i32.load8_u", 1)
	mem("i32.load16_s", 2)
	mem("i32.load16_u", 2)
	mem("i64.load8_s", 1)
	mem("i64.load8_u", 1)
	mem("i64.load16_s", 2)
	mem("i64.load16_u", 2)
	mem("i64.load32_s", 4)
	mem("i64.load32_u", 4)
	mem("i32.store", 4)
	mem("i64.store", 8)
	mem("f32.store", 4)
	mem("f64.store", 8)
	mem("i32.store8", 1)
	mem("i32.store16", 2)
	mem("i64.store8", 1)
	mem("i64.store16", 2)
	mem("i64.store32", 4)
	// note: the optional memory-index immediate (multi-memory) is unsupported
	none("memory.size", "memory.grow")
	t["memory.fill"] = instrInfo{imm: immMemFill}
	t["memory.copy"] = instrInfo{imm: immMemCopy}

	// constants
	t["i32.const"] = instrInfo{imm: immConstI32}
	t["i64.const"] = instrInfo{imm: immConstI64}
	t["f32.const"] = instrInfo{imm: immConstF32}
	t["f64.const"] = instrInfo{imm: immConstF64}

	// numeric operators without immediates
	iuni := "clz ctz popcnt eqz extend8_s extend16_s"
	ibin := "eq ne lt_s lt_u gt_s gt_u le_s le_u ge_s ge_u add sub mul div_s div_u rem_s rem_u and or xor shl shr_s shr_u rotl rotr"
	funi := "abs neg ceil floor trunc nearest sqrt"
	fbin := "eq ne lt gt le ge add sub mul div min max copysign"
	for _, op := range strings.Fields(iuni + " " + ibin) {
		none("i32."+op, "i64."+op)
	}
	for _, op := range strings.Fields(funi + " " + fbin) {
		none("f32."+op, "f64."+op)
	}
	none("i64.extend32_s")

	// conversions
	none(
		"i32.wrap_i64", "i64.extend_i32_s", "i64.extend_i32_u",
		"i32.trunc_f32_s", "i32.trunc_f32_u", "i32.trunc_f64_s", "i32.trunc_f64_u",
		"i64.trunc_f32_s", "i64.trunc_f32_u", "i64.trunc_f64_s", "i64.trunc_f64_u",
		"i32.trunc_sat_f32_s", "i32.trunc_sat_f32_u", "i32.trunc_sat_f64_s", "i32.trunc_sat_f64_u",
		"i64.trunc_sat_f32_s", "i64.trunc_sat_f32_u", "i64.trunc_sat_f64_s", "i64.trunc_sat_f64_u",
		"f32.convert_i32_s", "f32.convert_i32_u", "f32.convert_i64_s", "f32.convert_i64_u",
		"f64.convert_i32_s", "f64.convert_i32_u", "f64.convert_i64_s", "f64.convert_i64_u",
		"f32.demote_f64", "f64.promote_f32",
		"i32.reinterpret_f32", "i64.reinterpret_f64", "f32.reinterpret_i32", "f64.reinterpret_i64",
	)

	return t
}

// instrCtx carries the per-function name scopes during body checking.
type instrCtx struct {
	locals map[string]bool
	labels []string // innermost last; "" = unnamed
}

func (ctx *instrCtx) resolveLabel(t *token) error {
	switch t.kind {
	case tokNum:
		_, err := parseUint(t.text, 32)
		return err
	case tokID:
		for i := len(ctx.labels) - 1; i >= 0; i-- {
			if ctx.labels[i] == t.text {
				return nil
			}
		}
		return fmt.Errorf("unknown label %s", t.text)
	}
	return fmt.Errorf("expected label, got %q", t.text)
}

// checkInstrSeq validates a mixed sequence of plain instructions (atoms) and
// folded instructions (lists).
func (c *checker) checkInstrSeq(items []node, ctx *instrCtx) error {
	i := 0
	for i < len(items) {
		n, err := c.checkOneInstr(items, i, ctx)
		if err != nil {
			return err
		}
		i = n
	}
	return nil
}

// checkOneInstr validates the instruction starting at items[i] and returns the
// index just past it (plain block/loop/if consume through their `end`).
func (c *checker) checkOneInstr(items []node, i int, ctx *instrCtx) (int, error) {
	it := &items[i]

	if it.isList() {
		return i + 1, c.checkFolded(it, ctx)
	}

	t := it.atom
	if t.kind != tokKeyword {
		return 0, fmt.Errorf("unexpected token %q", t.text)
	}

	switch t.text {
	case "block", "loop":
		return c.checkPlainBlock(items, i, ctx, false)
	case "if":
		return c.checkPlainBlock(items, i, ctx, true)
	case "else", "end":
		return 0, fmt.Errorf("unexpected %q", t.text)
	}

	info, ok := instrTable[t.text]
	if !ok {
		return 0, fmt.Errorf("unknown operator %q", t.text)
	}
	return c.checkImmediates(items, i+1, info, ctx)
}

// checkImmediates consumes an instruction's immediates starting at items[i]
// (for a folded instruction, the immediates live inside the folded list).
func (c *checker) checkImmediates(items []node, i int, info instrInfo, ctx *instrCtx) (int, error) {
	atom := func(j int) *token {
		if j < len(items) && !items[j].isList() {
			return items[j].atom
		}
		return nil
	}

	switch info.imm {
	case immNone, immMemFill, immMemCopy:
		// bulk-memory ops take no text immediates; the implicit
		// memory-index bytes are emitted at compile time
		return i, nil

	case immLabel:
		t := atom(i)
		if t == nil {
			return 0, errors.New("expected label")
		}
		return i + 1, ctx.resolveLabel(t)

	case immBrTable:
		count := 0
		for {
			t := atom(i)
			if t == nil || (t.kind != tokNum && t.kind != tokID) {
				break
			}
			if err := ctx.resolveLabel(t); err != nil {
				return 0, err
			}
			i++
			count++
		}
		if count == 0 {
			return 0, errors.New("expected label")
		}
		return i, nil

	case immFunc:
		t := atom(i)
		if t == nil {
			return 0, errors.New("expected function index")
		}
		return i + 1, checkIdxAtom(t, c.funcIDs, "function")

	case immLocal:
		t := atom(i)
		if t == nil {
			return 0, errors.New("expected local index")
		}
		return i + 1, checkIdxAtom(t, ctx.locals, "local")

	case immGlobal:
		t := atom(i)
		if t == nil {
			return 0, errors.New("expected global index")
		}
		return i + 1, checkIdxAtom(t, c.globalIDs, "global")

	case immCallIndirect:
		// optional table index
		if t := atom(i); t != nil && (t.kind == tokNum || t.kind == tokID) {
			if err := checkIdxAtom(t, c.tableIDs, "table"); err != nil {
				return 0, err
			}
			i++
		}
		// typeuse lists follow (they are lists even in plain form)
		_, consumed, err := c.parseTypeuse(items[i:], false)
		if err != nil {
			return 0, err
		}
		return i + consumed, nil

	case immMemarg:
		if t := atom(i); t != nil && strings.HasPrefix(t.text, "offset=") {
			if _, err := parseUint(t.text[len("offset="):], 32); err != nil {
				return 0, errors.New("offset out of range")
			}
			i++
		}
		if t := atom(i); t != nil && strings.HasPrefix(t.text, "align=") {
			v, err := parseUint(t.text[len("align="):], 32)
			if err != nil {
				return 0, errors.New("align out of range")
			}
			if v == 0 || v&(v-1) != 0 {
				return 0, errors.New("alignment must be a power of two")
			}
			if v > info.maxAlign {
				return 0, errors.New("alignment must not be larger than natural")
			}
			i++
		}
		// a stray offset= AFTER align= is malformed
		if t := atom(i); t != nil && strings.HasPrefix(t.text, "offset=") {
			return 0, errors.New("unexpected offset=")
		}
		return i, nil

	case immConstI32, immConstI64:
		t := atom(i)
		if t == nil || t.kind != tokNum {
			if t != nil {
				return 0, fmt.Errorf("unexpected token %q", t.text)
			}
			return 0, errors.New("expected integer constant")
		}
		bits := uint(32)
		if info.imm == immConstI64 {
			bits = 64
		}
		if _, err := parseInt(t.text, bits); err != nil {
			return 0, err
		}
		return i + 1, nil

	case immConstF32, immConstF64:
		t := atom(i)
		if t == nil {
			return 0, errors.New("expected float constant")
		}
		// floats may lex as numbers (1.5) or keywords (inf, nan, nan:0x…)
		if t.kind != tokNum && t.kind != tokKeyword {
			return 0, fmt.Errorf("unexpected token %q", t.text)
		}
		if t.kind == tokKeyword && !isNumToken(t.text) {
			return 0, fmt.Errorf("unexpected token %q", t.text)
		}
		bits := uint(32)
		if info.imm == immConstF64 {
			bits = 64
		}
		if err := checkFloat(t.text, bits); err != nil {
			return 0, err
		}
		return i + 1, nil
	}
	return 0, errors.New("internal: unhandled immediate kind")
}

// parseBlockType consumes a block's label and blocktype from items[i:] and
// pushes the label. Blocktype params must be unnamed.
func (c *checker) parseBlockLabelAndType(items []node, i int, ctx *instrCtx) (int, error) {
	label := ""
	if i < len(items) && !items[i].isList() && items[i].atom.kind == tokID {
		label = items[i].atom.text
		i++
	}
	_, consumed, err := c.parseTypeuse(items[i:], false)
	if err != nil {
		return 0, err
	}
	ctx.labels = append(ctx.labels, label)
	return i + consumed, nil
}

// checkEndLabel validates the optional id after `end` / `else`.
func checkEndLabel(items []node, i int, label string) (int, error) {
	if i < len(items) && !items[i].isList() && items[i].atom.kind == tokID {
		if items[i].atom.text != label {
			return 0, errors.New("mismatching label")
		}
		return i + 1, nil
	}
	return i, nil
}

// checkPlainBlock handles plain-form block/loop/if … [else …] end.
func (c *checker) checkPlainBlock(items []node, i int, ctx *instrCtx, isIf bool) (int, error) {
	i++ // past block/loop/if
	next, err := c.parseBlockLabelAndType(items, i, ctx)
	if err != nil {
		return 0, err
	}
	label := ctx.labels[len(ctx.labels)-1]
	defer func() { ctx.labels = ctx.labels[:len(ctx.labels)-1] }()
	i = next

	seenElse := false
	for {
		if i >= len(items) {
			return 0, errors.New("unclosed block: expected end")
		}
		if !items[i].isList() && items[i].atom.kind == tokKeyword {
			switch items[i].atom.text {
			case "end":
				return checkEndLabel(items, i+1, label)
			case "else":
				if !isIf || seenElse {
					return 0, errors.New("unexpected else")
				}
				seenElse = true
				i, err = checkEndLabel(items, i+1, label)
				if err != nil {
					return 0, err
				}
				continue
			}
		}
		i, err = c.checkOneInstr(items, i, ctx)
		if err != nil {
			return 0, err
		}
	}
}

// checkFolded handles a folded instruction list.
func (c *checker) checkFolded(n *node, ctx *instrCtx) error {
	if len(n.list) == 0 {
		return errors.New("empty expression")
	}
	if n.list[0].isList() || n.list[0].atom.kind != tokKeyword {
		return errors.New("expected an operator")
	}
	name := n.list[0].atom.text
	items := n.list[1:]

	switch name {
	case "block", "loop":
		next, err := c.parseBlockLabelAndType(items, 0, ctx)
		if err != nil {
			return err
		}
		defer func() { ctx.labels = ctx.labels[:len(ctx.labels)-1] }()
		return c.checkInstrSeq(items[next:], ctx)

	case "if":
		next, err := c.parseBlockLabelAndType(items, 0, ctx)
		if err != nil {
			return err
		}
		defer func() { ctx.labels = ctx.labels[:len(ctx.labels)-1] }()
		i := next
		// folded condition operands
		for i < len(items) && items[i].isList() && items[i].head() != "then" && items[i].head() != "else" {
			if err := c.checkFolded(&items[i], ctx); err != nil {
				return err
			}
			i++
		}
		if i >= len(items) || items[i].head() != "then" {
			return errors.New("expected (then ...) in folded if")
		}
		if err := c.checkInstrSeq(items[i].list[1:], ctx); err != nil {
			return err
		}
		i++
		if i < len(items) {
			if items[i].head() != "else" {
				return errors.New("unexpected token in folded if")
			}
			if err := c.checkInstrSeq(items[i].list[1:], ctx); err != nil {
				return err
			}
			i++
		}
		if i != len(items) {
			return errors.New("unexpected token after folded if")
		}
		return nil

	case "then", "else", "end":
		return fmt.Errorf("unexpected %q", name)
	}

	info, ok := instrTable[name]
	if !ok {
		return fmt.Errorf("unknown operator %q", name)
	}
	next, err := c.checkImmediates(items, 0, info, ctx)
	if err != nil {
		return err
	}
	// all remaining items must be folded operand expressions
	for i := next; i < len(items); i++ {
		if !items[i].isList() {
			return fmt.Errorf("unexpected token %q", items[i].atom.text)
		}
		if err := c.checkFolded(&items[i], ctx); err != nil {
			return err
		}
	}
	return nil
}
