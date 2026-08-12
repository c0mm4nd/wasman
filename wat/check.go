package wat

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

// funcSig is the shape of a function type in text form.
type funcSig struct {
	params  []string // value type names
	results []string
}

func sigEqual(a, b funcSig) bool {
	if len(a.params) != len(b.params) || len(a.results) != len(b.results) {
		return false
	}
	for i := range a.params {
		if a.params[i] != b.params[i] {
			return false
		}
	}
	for i := range a.results {
		if a.results[i] != b.results[i] {
			return false
		}
	}
	return true
}

type checker struct {
	types   []funcSig
	typeIDs map[string]int

	funcIDs, tableIDs, memIDs, globalIDs map[string]bool

	// import-order rule: imports must precede any definition of the same kinds
	sawDefinition bool
	sawStart      bool
}

func isValType(s string) bool {
	switch s {
	case "i32", "i64", "f32", "f64":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// public entry points
// ---------------------------------------------------------------------------

// ValidateText checks a module text: either a single (module ...) form or a
// bare sequence of module fields (as the spec's `quote` payloads allow).
func ValidateText(src []byte) error {
	toks, err := lex(src)
	if err != nil {
		return err
	}
	forest, err := parseSExprs(toks)
	if err != nil {
		return err
	}

	fields := forest
	if len(forest) == 1 && forest[0].head() == "module" {
		fields = forest[0].list[1:]
		// an optional module id
		if len(fields) > 0 && !fields[0].isList() && fields[0].atom.kind == tokID {
			fields = fields[1:]
		}
	}
	return checkModuleFields(fields)
}

// ScriptModules parses a .wast script and grammar-checks every plain
// (module ...) form in it (skipping `binary` and `quote` payload modules).
// It returns how many modules were checked; any rejection is an error.
// This is the positive control guaranteeing the checker accepts valid text.
func ScriptModules(src []byte) (int, error) {
	toks, err := lex(src)
	if err != nil {
		return 0, err
	}
	forest, err := parseSExprs(toks)
	if err != nil {
		return 0, err
	}

	checked := 0
	for _, top := range forest {
		if top.head() != "module" {
			continue
		}
		fields := top.list[1:]
		if len(fields) > 0 && !fields[0].isList() && fields[0].atom.kind == tokID {
			fields = fields[1:]
		}
		// skip (module binary "...") / (module quote "...") payload forms
		if len(fields) > 0 && !fields[0].isList() && fields[0].atom.kind == tokKeyword &&
			(fields[0].atom.text == "binary" || fields[0].atom.text == "quote") {
			continue
		}
		if err := checkModuleFields(fields); err != nil {
			return checked, fmt.Errorf("valid module rejected: %w", err)
		}
		checked++
	}
	return checked, nil
}

// ---------------------------------------------------------------------------
// module fields
// ---------------------------------------------------------------------------

func checkModuleFields(fields []node) error {
	c := &checker{
		typeIDs:   map[string]int{},
		funcIDs:   map[string]bool{},
		tableIDs:  map[string]bool{},
		memIDs:    map[string]bool{},
		globalIDs: map[string]bool{},
	}

	// pass 1: collect ids (they may be referenced before their definition)
	// and parse type definitions for typeuse matching
	for i := range fields {
		f := &fields[i]
		if !f.isList() || len(f.list) == 0 {
			return errors.New("unexpected token at module level")
		}
		head := f.head()
		items := f.list[1:]
		var id string
		if len(items) > 0 && !items[0].isList() && items[0].atom.kind == tokID {
			id = items[0].atom.text
		}
		switch head {
		case "type":
			if id != "" {
				if _, dup := c.typeIDs[id]; dup {
					return fmt.Errorf("duplicate type %s", id)
				}
				c.typeIDs[id] = len(c.types)
				items = items[1:]
			}
			if len(items) != 1 || items[0].head() != "func" {
				return errors.New("malformed type definition")
			}
			sig, _, err := c.parseFuncType(items[0].list[1:], true)
			if err != nil {
				return err
			}
			c.types = append(c.types, sig)
		case "func":
			if err := collectID(c.funcIDs, id, "func"); err != nil {
				return err
			}
		case "table":
			if err := collectID(c.tableIDs, id, "table"); err != nil {
				return err
			}
		case "memory":
			if err := collectID(c.memIDs, id, "memory"); err != nil {
				return err
			}
		case "global":
			if err := collectID(c.globalIDs, id, "global"); err != nil {
				return err
			}
		case "import":
			// (import "m" "n" (func $id ...)) — the id lives on the desc
			if len(f.list) == 4 && f.list[3].isList() {
				desc := &f.list[3]
				dItems := desc.list[1:]
				if len(dItems) > 0 && !dItems[0].isList() && dItems[0].atom.kind == tokID {
					var set map[string]bool
					switch desc.head() {
					case "func":
						set = c.funcIDs
					case "table":
						set = c.tableIDs
					case "memory":
						set = c.memIDs
					case "global":
						set = c.globalIDs
					}
					if set != nil {
						if err := collectID(set, dItems[0].atom.text, desc.head()); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	// pass 2: full grammar check per field
	for i := range fields {
		f := &fields[i]
		var err error
		switch f.head() {
		case "type":
			// checked in pass 1
		case "func":
			err = c.checkFunc(f.list[1:])
		case "table":
			err = c.checkTable(f.list[1:])
		case "memory":
			err = c.checkMemory(f.list[1:])
		case "global":
			err = c.checkGlobal(f.list[1:])
		case "import":
			err = c.checkImport(f.list[1:])
		case "export":
			err = c.checkExport(f.list[1:])
		case "start":
			if c.sawStart {
				return errors.New("multiple start sections")
			}
			c.sawStart = true
			err = c.checkStart(f.list[1:])
		case "elem":
			err = c.checkElem(f.list[1:])
		case "data":
			err = c.checkData(f.list[1:])
		default:
			return fmt.Errorf("unexpected module field %q", f.head())
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func collectID(set map[string]bool, id, kind string) error {
	if id == "" {
		return nil
	}
	if set[id] {
		return fmt.Errorf("duplicate %s %s", kind, id)
	}
	set[id] = true
	return nil
}

// markImport enforces the imports-before-definitions rule.
func (c *checker) markImport(kind string) error {
	if c.sawDefinition {
		return fmt.Errorf("import after %s definition", kind)
	}
	return nil
}

// ---------------------------------------------------------------------------
// shared pieces
// ---------------------------------------------------------------------------

// parseFuncType parses (param ...)* (result ...)* with ordering enforced.
// Named params are only allowed when named is true; named results never are.
// paramNames is aligned with sig.params ("" = unnamed).
func (c *checker) parseFuncType(items []node, named bool) (funcSig, []string, error) {
	sig := funcSig{}
	var names []string
	seenResult := false
	for i := range items {
		it := &items[i]
		switch it.head() {
		case "param":
			if seenResult {
				return sig, nil, errors.New("unexpected param after result")
			}
			ps, ns, err := parseValTypeVec(it.list[1:], named)
			if err != nil {
				return sig, nil, err
			}
			sig.params = append(sig.params, ps...)
			names = append(names, ns...)
		case "result":
			seenResult = true
			rs, _, err := parseValTypeVec(it.list[1:], false)
			if err != nil {
				return sig, nil, err
			}
			sig.results = append(sig.results, rs...)
		default:
			return sig, nil, fmt.Errorf("unexpected %q in function type", it.head())
		}
	}
	return sig, names, nil
}

// parseValTypeVec parses the contents of a (param ...) or (result ...) list,
// returning the value types and the aligned names ("" = unnamed).
func parseValTypeVec(items []node, allowName bool) ([]string, []string, error) {
	if len(items) == 0 {
		return nil, nil, nil
	}
	// named form: exactly ($id valtype)
	if !items[0].isList() && items[0].atom.kind == tokID {
		if !allowName {
			return nil, nil, errors.New("unexpected name")
		}
		if len(items) != 2 || items[1].isList() || !isValType(items[1].atom.text) {
			return nil, nil, errors.New("malformed named parameter")
		}
		return []string{items[1].atom.text}, []string{items[0].atom.text}, nil
	}
	var out, names []string
	for i := range items {
		if items[i].isList() || !isValType(items[i].atom.text) {
			return nil, nil, errors.New("expected value type")
		}
		out = append(out, items[i].atom.text)
		names = append(names, "")
	}
	return out, names, nil
}

// parseTypeuse parses (type x)? (param..)* (result..)* from items, returning
// the resolved signature, the param names, and how many items were consumed.
// If a (type x) is given together with inline params/results, they must match
// the definition.
func (c *checker) parseTypeuse(items []node, named bool) (funcSig, []string, int, error) {
	consumed := 0
	var declared *funcSig
	if len(items) > 0 && items[0].head() == "type" {
		ti := items[0].list[1:]
		if len(ti) != 1 || ti[0].isList() {
			return funcSig{}, nil, 0, errors.New("malformed type use")
		}
		idx, err := c.resolveIdx(ti[0].atom, c.typeIDs, len(c.types), "type")
		if err != nil {
			return funcSig{}, nil, 0, err
		}
		if idx >= 0 { // numeric index in range, or resolved id
			declared = &c.types[idx]
		}
		consumed++
	}

	inlineEnd := consumed
	for inlineEnd < len(items) {
		h := items[inlineEnd].head()
		if h == "param" || h == "result" {
			inlineEnd++
		} else {
			break
		}
	}
	// e.g. a (type ...) appearing after (param)/(result) lands here
	if inlineEnd != len(items) {
		return funcSig{}, nil, 0, errors.New("unexpected token in type use")
	}
	inline, names, err := c.parseFuncType(items[consumed:inlineEnd], named)
	if err != nil {
		return funcSig{}, nil, 0, err
	}

	if declared != nil {
		if inlineEnd > consumed && !sigEqual(inline, *declared) {
			return funcSig{}, nil, 0, errors.New("inline function type does not match type use")
		}
		return *declared, names, inlineEnd, nil
	}
	return inline, names, inlineEnd, nil
}

// resolveIdx checks an index atom: a u32 number (returns -1: range-checked at
// instantiation, not here) or an id which must resolve.
func (c *checker) resolveIdx(t *token, ids map[string]int, n int, kind string) (int, error) {
	switch t.kind {
	case tokNum:
		if _, err := parseUint(t.text, 32); err != nil {
			return 0, fmt.Errorf("malformed %s index: %v", kind, err)
		}
		return -1, nil
	case tokID:
		idx, ok := ids[t.text]
		if !ok {
			return 0, fmt.Errorf("unknown %s %s", kind, t.text)
		}
		return idx, nil
	}
	return 0, fmt.Errorf("expected %s index", kind)
}

// checkIdxAtom checks an index against a plain id set.
func checkIdxAtom(t *token, ids map[string]bool, kind string) error {
	switch t.kind {
	case tokNum:
		if _, err := parseUint(t.text, 32); err != nil {
			return fmt.Errorf("malformed %s index: %v", kind, err)
		}
		return nil
	case tokID:
		if !ids[t.text] {
			return fmt.Errorf("unknown %s %s", kind, t.text)
		}
		return nil
	}
	return fmt.Errorf("expected %s index, got %q", kind, t.text)
}

// checkLimits parses "u32 u32?" limits.
func checkLimits(items []node) (rest []node, err error) {
	if len(items) == 0 || items[0].isList() || items[0].atom.kind != tokNum {
		return nil, errors.New("expected limits")
	}
	if _, err := parseUint(items[0].atom.text, 32); err != nil {
		return nil, errors.New("i32 constant out of range")
	}
	items = items[1:]
	if len(items) > 0 && !items[0].isList() && items[0].atom.kind == tokNum {
		if _, err := parseUint(items[0].atom.text, 32); err != nil {
			return nil, errors.New("i32 constant out of range")
		}
		items = items[1:]
	}
	return items, nil
}

// checkName validates that a decoded string is a valid UTF-8 name.
func checkName(t *token) error {
	if t.kind != tokString {
		return errors.New("expected a name string")
	}
	if !utf8.Valid(t.str) {
		return errors.New("malformed UTF-8 encoding in name")
	}
	return nil
}

// splitInlines consumes leading (export "n")* and an optional (import "m" "n")
// from a field's items (after its id).
func (c *checker) splitInlines(items []node, kind string) (rest []node, imported bool, err error) {
	for len(items) > 0 && items[0].head() == "export" {
		e := items[0].list[1:]
		if len(e) != 1 || e[0].isList() {
			return nil, false, errors.New("malformed inline export")
		}
		if err := checkName(e[0].atom); err != nil {
			return nil, false, err
		}
		items = items[1:]
	}
	if len(items) > 0 && items[0].head() == "import" {
		im := items[0].list[1:]
		if len(im) != 2 || im[0].isList() || im[1].isList() {
			return nil, false, errors.New("malformed inline import")
		}
		if err := checkName(im[0].atom); err != nil {
			return nil, false, err
		}
		if err := checkName(im[1].atom); err != nil {
			return nil, false, err
		}
		if err := c.markImport(kind); err != nil {
			return nil, false, err
		}
		return items[1:], true, nil
	}
	return items, false, nil
}

// ---------------------------------------------------------------------------
// fields
// ---------------------------------------------------------------------------

func (c *checker) checkFunc(items []node) error {
	if len(items) > 0 && !items[0].isList() && items[0].atom.kind == tokID {
		items = items[1:]
	}
	items, imported, err := c.splitInlines(items, "function")
	if err != nil {
		return err
	}
	tuEnd := 0
	for tuEnd < len(items) && items[tuEnd].isList() {
		h := items[tuEnd].head()
		if h == "type" || h == "param" || h == "result" {
			tuEnd++
		} else {
			break
		}
	}
	sig, paramNames, _, err := c.parseTypeuse(items[:tuEnd], true)
	if err != nil {
		return err
	}
	items = items[tuEnd:]
	if imported {
		if len(items) != 0 {
			return errors.New("unexpected body on imported function")
		}
		return nil
	}
	c.sawDefinition = true

	// locals (params first; duplicates across params and locals are malformed)
	locals := map[string]bool{}
	for _, name := range paramNames {
		if name == "" {
			continue
		}
		if locals[name] {
			return fmt.Errorf("duplicate local %s", name)
		}
		locals[name] = true
	}
	for len(items) > 0 && items[0].head() == "local" {
		li := items[0].list[1:]
		if len(li) > 0 && !li[0].isList() && li[0].atom.kind == tokID {
			if locals[li[0].atom.text] {
				return fmt.Errorf("duplicate local %s", li[0].atom.text)
			}
			locals[li[0].atom.text] = true
			if len(li) != 2 || li[1].isList() || !isValType(li[1].atom.text) {
				return errors.New("malformed named local")
			}
		} else {
			for i := range li {
				if li[i].isList() || !isValType(li[i].atom.text) {
					return errors.New("expected value type in local")
				}
			}
		}
		items = items[1:]
	}
	_ = sig

	return c.checkInstrSeq(items, &instrCtx{locals: locals, labels: nil})
}

func (c *checker) checkTable(items []node) error {
	if len(items) > 0 && !items[0].isList() && items[0].atom.kind == tokID {
		items = items[1:]
	}
	items, imported, err := c.splitInlines(items, "table")
	if err != nil {
		return err
	}
	if !imported {
		c.sawDefinition = true
	}
	// either: limits elemtype  |  elemtype (elem ...)
	if len(items) > 0 && !items[0].isList() && items[0].atom.text == "funcref" {
		if imported {
			return errors.New("inline elements on imported table")
		}
		if len(items) != 2 || items[1].head() != "elem" {
			return errors.New("malformed inline table elements")
		}
		return c.checkElemList(items[1].list[1:])
	}
	items, err = checkLimits(items)
	if err != nil {
		return err
	}
	if len(items) != 1 || items[0].isList() || items[0].atom.text != "funcref" {
		return errors.New("expected element type funcref")
	}
	return nil
}

func (c *checker) checkMemory(items []node) error {
	if len(items) > 0 && !items[0].isList() && items[0].atom.kind == tokID {
		items = items[1:]
	}
	items, imported, err := c.splitInlines(items, "memory")
	if err != nil {
		return err
	}
	if !imported {
		c.sawDefinition = true
	}
	// (memory (data "..")) inline form
	if len(items) == 1 && items[0].head() == "data" {
		if imported {
			return errors.New("inline data on imported memory")
		}
		for _, d := range items[0].list[1:] {
			if d.isList() || d.atom.kind != tokString {
				return errors.New("expected string in inline data")
			}
		}
		return nil
	}
	items, err = checkLimits(items)
	if err != nil {
		return err
	}
	if len(items) != 0 {
		return errors.New("unexpected token after memory limits")
	}
	return nil
}

func (c *checker) checkGlobal(items []node) error {
	if len(items) > 0 && !items[0].isList() && items[0].atom.kind == tokID {
		items = items[1:]
	}
	items, imported, err := c.splitInlines(items, "global")
	if err != nil {
		return err
	}
	if !imported {
		c.sawDefinition = true
	}
	if len(items) == 0 {
		return errors.New("expected global type")
	}
	// globaltype: valtype | (mut valtype)
	gt := items[0]
	if gt.isList() {
		if gt.head() != "mut" || len(gt.list) != 2 || gt.list[1].isList() || !isValType(gt.list[1].atom.text) {
			return errors.New("malformed global type")
		}
	} else if !isValType(gt.atom.text) {
		return errors.New("malformed global type")
	}
	items = items[1:]
	if imported {
		if len(items) != 0 {
			return errors.New("unexpected init on imported global")
		}
		return nil
	}
	return c.checkInstrSeq(items, &instrCtx{})
}

func (c *checker) checkImport(items []node) error {
	if len(items) != 3 || items[0].isList() || items[1].isList() || !items[2].isList() {
		return errors.New("malformed import")
	}
	if err := checkName(items[0].atom); err != nil {
		return err
	}
	if err := checkName(items[1].atom); err != nil {
		return err
	}
	desc := items[2]
	d := desc.list[1:]
	if len(d) > 0 && !d[0].isList() && d[0].atom.kind == tokID {
		d = d[1:]
	}
	switch desc.head() {
	case "func":
		if err := c.markImport("function"); err != nil {
			return err
		}
		sig, _, _, err := c.parseTypeuse(d, true)
		if err != nil {
			return err
		}
		_ = sig
		return nil
	case "table":
		if err := c.markImport("table"); err != nil {
			return err
		}
		rest, err := checkLimits(d)
		if err != nil {
			return err
		}
		if len(rest) != 1 || rest[0].isList() || rest[0].atom.text != "funcref" {
			return errors.New("expected element type funcref")
		}
		return nil
	case "memory":
		if err := c.markImport("memory"); err != nil {
			return err
		}
		rest, err := checkLimits(d)
		if err != nil {
			return err
		}
		if len(rest) != 0 {
			return errors.New("unexpected token in memory import")
		}
		return nil
	case "global":
		if err := c.markImport("global"); err != nil {
			return err
		}
		if len(d) != 1 {
			return errors.New("malformed global import")
		}
		gt := d[0]
		if gt.isList() {
			if gt.head() != "mut" || len(gt.list) != 2 || gt.list[1].isList() || !isValType(gt.list[1].atom.text) {
				return errors.New("malformed global type")
			}
		} else if !isValType(gt.atom.text) {
			return errors.New("malformed global type")
		}
		return nil
	}
	return fmt.Errorf("unexpected import description %q", desc.head())
}

func (c *checker) checkExport(items []node) error {
	if len(items) != 2 || items[0].isList() || !items[1].isList() {
		return errors.New("malformed export")
	}
	if err := checkName(items[0].atom); err != nil {
		return err
	}
	desc := items[1]
	d := desc.list[1:]
	if len(d) != 1 || d[0].isList() {
		return errors.New("malformed export description")
	}
	switch desc.head() {
	case "func":
		return checkIdxAtom(d[0].atom, c.funcIDs, "function")
	case "table":
		return checkIdxAtom(d[0].atom, c.tableIDs, "table")
	case "memory":
		return checkIdxAtom(d[0].atom, c.memIDs, "memory")
	case "global":
		return checkIdxAtom(d[0].atom, c.globalIDs, "global")
	}
	return fmt.Errorf("unexpected export description %q", desc.head())
}

func (c *checker) checkStart(items []node) error {
	if len(items) != 1 || items[0].isList() {
		return errors.New("malformed start")
	}
	return checkIdxAtom(items[0].atom, c.funcIDs, "function")
}

func (c *checker) checkElem(items []node) error {
	// optional segment id
	if len(items) > 0 && !items[0].isList() && items[0].atom.kind == tokID {
		items = items[1:]
	}
	// declarative segments: (elem declare func $f ...)
	if len(items) > 0 && !items[0].isList() && items[0].atom.text == "declare" {
		return c.checkElemTail(items[1:])
	}
	// optional (table x) — or, in the legacy abbreviation, a bare table index
	if len(items) > 0 && items[0].head() == "table" {
		ti := items[0].list[1:]
		if len(ti) != 1 || ti[0].isList() {
			return errors.New("malformed table use")
		}
		if err := checkIdxAtom(ti[0].atom, c.tableIDs, "table"); err != nil {
			return err
		}
		items = items[1:]
	} else if len(items) > 0 && !items[0].isList() && (items[0].atom.kind == tokNum || items[0].atom.kind == tokID) {
		if err := checkIdxAtom(items[0].atom, c.tableIDs, "table"); err != nil {
			return err
		}
		items = items[1:]
	}
	// offset: (offset instr*) or a single folded instruction
	if len(items) == 0 || !items[0].isList() {
		return errors.New("expected offset expression")
	}
	if items[0].head() == "offset" {
		if err := c.checkInstrSeq(items[0].list[1:], &instrCtx{}); err != nil {
			return err
		}
	} else {
		if err := c.checkInstrSeq(items[0:1], &instrCtx{}); err != nil {
			return err
		}
	}
	return c.checkElemTail(items[1:])
}

// checkElemTail checks the element list: optional `func` keyword + indices,
// or `funcref` + (ref.func x)/(ref.null func) items.
func (c *checker) checkElemTail(items []node) error {
	if len(items) > 0 && !items[0].isList() && items[0].atom.text == "func" {
		return c.checkElemList(items[1:])
	}
	if len(items) > 0 && !items[0].isList() && items[0].atom.text == "funcref" {
		for _, it := range items[1:] {
			if !it.isList() {
				return errors.New("expected element expression")
			}
			switch it.head() {
			case "ref.func":
				if len(it.list) != 2 || it.list[1].isList() {
					return errors.New("malformed ref.func")
				}
				if err := checkIdxAtom(it.list[1].atom, c.funcIDs, "function"); err != nil {
					return err
				}
			case "ref.null":
				// (ref.null func)
			default:
				return errors.New("expected element expression")
			}
		}
		return nil
	}
	return c.checkElemList(items)
}

func (c *checker) checkElemList(items []node) error {
	for i := range items {
		if items[i].isList() {
			return errors.New("expected function index")
		}
		if err := checkIdxAtom(items[i].atom, c.funcIDs, "function"); err != nil {
			return err
		}
	}
	return nil
}

func (c *checker) checkData(items []node) error {
	if len(items) > 0 && !items[0].isList() && items[0].atom.kind == tokID {
		items = items[1:]
	}
	// optional (memory x) — or, in the legacy abbreviation, a bare memory index
	if len(items) > 0 && items[0].head() == "memory" {
		mi := items[0].list[1:]
		if len(mi) != 1 || mi[0].isList() {
			return errors.New("malformed memory use")
		}
		if err := checkIdxAtom(mi[0].atom, c.memIDs, "memory"); err != nil {
			return err
		}
		items = items[1:]
	} else if len(items) > 1 && !items[0].isList() && (items[0].atom.kind == tokNum || items[0].atom.kind == tokID) {
		if err := checkIdxAtom(items[0].atom, c.memIDs, "memory"); err != nil {
			return err
		}
		items = items[1:]
	}
	// passive form: only strings
	if len(items) == 0 || (len(items) > 0 && !items[0].isList()) {
		for i := range items {
			if items[i].isList() || items[i].atom.kind != tokString {
				return errors.New("expected string in data segment")
			}
		}
		return nil
	}
	// offset
	if items[0].head() == "offset" {
		if err := c.checkInstrSeq(items[0].list[1:], &instrCtx{}); err != nil {
			return err
		}
	} else {
		if err := c.checkInstrSeq(items[0:1], &instrCtx{}); err != nil {
			return err
		}
	}
	for _, it := range items[1:] {
		if it.isList() || it.atom.kind != tokString {
			return errors.New("expected string in data segment")
		}
	}
	return nil
}
