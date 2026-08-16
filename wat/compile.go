package wat

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Compile translates a module in text format into its binary encoding.
// The input is either a single (module ...) form or a bare sequence of
// module fields. The translation is deterministic: the same text always
// yields the same bytes.
func Compile(src []byte) ([]byte, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	forest, err := parseSExprs(toks)
	if err != nil {
		return nil, err
	}

	fields := forest
	if len(forest) == 1 && forest[0].head() == "module" {
		fields = moduleFields(&forest[0])
	}

	// grammar-check first so the emitter can assume well-formed input
	if err := checkModuleFields(fields); err != nil {
		return nil, err
	}

	c := newCompiler()
	if err := c.collect(fields); err != nil {
		return nil, err
	}
	return c.emit()
}

// ---------------------------------------------------------------------------
// LEB128 and primitive encoders
// ---------------------------------------------------------------------------

func appendUleb(out []byte, v uint64) []byte {
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		out = append(out, b)
		if v == 0 {
			return out
		}
	}
}

func appendSleb(out []byte, v int64) []byte {
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if (v == 0 && b&0x40 == 0) || (v == -1 && b&0x40 != 0) {
			return append(out, b)
		}
		out = append(out, b|0x80)
	}
}

func appendName(out []byte, str []byte) []byte {
	out = appendUleb(out, uint64(len(str)))
	return append(out, str...)
}

func valTypeByte(name string) byte {
	switch name {
	case "i32":
		return 0x7f
	case "i64":
		return 0x7e
	case "f32":
		return 0x7d
	case "f64":
		return 0x7c
	}
	panic("unreachable: checked value type " + name)
}

// ---------------------------------------------------------------------------
// collected module representation
// ---------------------------------------------------------------------------

type importEntry struct {
	mod, name []byte
	desc      []byte // encoded importdesc
}

type exportEntry struct {
	name []byte
	kind byte  // 0 func, 1 table, 2 mem, 3 global
	idx  *node // resolved at emit time
}

type funcDef struct {
	typeIdx    int
	localNames map[string]int // param + local names -> local index
	numParams  int
	localTypes []byte // value types of the non-param locals
	body       []node
}

type globalDef struct {
	typ  []byte // encoded globaltype
	init []node // constant expression items
}

type elemDef struct {
	table  *node  // explicit table index atom, nil = table 0
	offset []node // constant expression items
	funcs  []node // function index atoms
}

type dataDef struct {
	offset []node
	bytes  []byte
}

type compiler struct {
	types   []funcSig
	typeIDs map[string]int

	imports []importEntry

	funcIDs            map[string]int
	numImportedFuncs   int
	funcs              []*funcDef
	tableIDs           map[string]int
	numImportedTables  int
	tables             [][]byte // encoded tabletypes
	memIDs             map[string]int
	numImportedMems    int
	mems               [][]byte // encoded memtypes
	globalIDs          map[string]int
	numImportedGlobals int
	globals            []globalDef

	exports []exportEntry
	start   *node
	elems   []elemDef
	datas   []dataDef
}

func newCompiler() *compiler {
	return &compiler{
		typeIDs:   map[string]int{},
		funcIDs:   map[string]int{},
		tableIDs:  map[string]int{},
		memIDs:    map[string]int{},
		globalIDs: map[string]int{},
	}
}

// typeIdxFor returns the index of sig in the type section, appending it
// when absent (the spec's typeuse resolution rule)
func (c *compiler) typeIdxFor(sig funcSig) int {
	for i := range c.types {
		if sigEqual(c.types[i], sig) {
			return i
		}
	}
	c.types = append(c.types, sig)
	return len(c.types) - 1
}

// resolveTypeuse resolves the (type x)? (param..)* (result..)* prefix into a
// type index, also returning the aligned param names and consumed items
func (c *compiler) resolveTypeuse(items []node, named bool) (int, []string, int, error) {
	if len(items) > 0 && items[0].head() == "type" {
		t := items[0].list[1].atom
		var idx int
		switch t.kind {
		case tokID:
			idx = c.typeIDs[t.text]
		case tokNum:
			n, err := parseUint(t.text, 32)
			if err != nil {
				return 0, nil, 0, err
			}
			if int(n) >= len(c.types) {
				return 0, nil, 0, fmt.Errorf("unknown type %d", n)
			}
			idx = int(n)
		}
		// consume the matching inline params/results, keeping their names
		inlineEnd := 1
		for inlineEnd < len(items) {
			if h := items[inlineEnd].head(); h == "param" || h == "result" {
				inlineEnd++
			} else {
				break
			}
		}
		_, names, err := c.parseFuncTypeC(items[1:inlineEnd], named)
		if err != nil {
			return 0, nil, 0, err
		}
		if len(names) == 0 {
			names = make([]string, len(c.types[idx].params))
		}
		return idx, names, inlineEnd, nil
	}

	inlineEnd := 0
	for inlineEnd < len(items) {
		if h := items[inlineEnd].head(); h == "param" || h == "result" {
			inlineEnd++
		} else {
			break
		}
	}
	sig, names, err := c.parseFuncTypeC(items[:inlineEnd], named)
	if err != nil {
		return 0, nil, 0, err
	}
	return c.typeIdxFor(sig), names, inlineEnd, nil
}

// parseFuncTypeC mirrors checker.parseFuncType without touching checker state
func (c *compiler) parseFuncTypeC(items []node, named bool) (funcSig, []string, error) {
	ck := &checker{}
	return ck.parseFuncType(items, named)
}

// ---------------------------------------------------------------------------
// pass A: collect fields and assign indices
// ---------------------------------------------------------------------------

func (c *compiler) collect(fields []node) error {
	// types must be registered before any typeuse referencing them by name
	for i := range fields {
		f := &fields[i]
		if f.head() != "type" {
			continue
		}
		items := f.list[1:]
		if len(items) > 0 && !items[0].isList() && items[0].atom.kind == tokID {
			c.typeIDs[items[0].atom.text] = len(c.types)
			items = items[1:]
		}
		sig, _, err := c.parseFuncTypeC(items[0].list[1:], true)
		if err != nil {
			return err
		}
		c.types = append(c.types, sig)
	}

	for i := range fields {
		f := &fields[i]
		var err error
		switch f.head() {
		case "type":
			// done above
		case "import":
			err = c.collectImport(f.list[1:])
		case "func":
			err = c.collectFunc(f.list[1:])
		case "table":
			err = c.collectTable(f.list[1:])
		case "memory":
			err = c.collectMemory(f.list[1:])
		case "global":
			err = c.collectGlobal(f.list[1:])
		case "export":
			items := f.list[1:]
			desc := &items[1]
			c.exports = append(c.exports, exportEntry{
				name: items[0].atom.str,
				kind: exportKindByte(desc.head()),
				idx:  &desc.list[1],
			})
		case "start":
			c.start = &f.list[1]
		case "elem":
			err = c.collectElem(f.list[1:])
		case "data":
			err = c.collectData(f.list[1:])
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func exportKindByte(kind string) byte {
	switch kind {
	case "func":
		return 0x00
	case "table":
		return 0x01
	case "memory":
		return 0x02
	case "global":
		return 0x03
	}
	panic("unreachable: checked export kind " + kind)
}

// takeID pops an optional leading $id
func takeID(items []node) (string, []node) {
	if len(items) > 0 && !items[0].isList() && items[0].atom.kind == tokID {
		return items[0].atom.text, items[1:]
	}
	return "", items
}

// takeInlineExports pops leading (export "name") forms, registering them
// against the upcoming index
func (c *compiler) takeInlineExports(items []node, kind byte, idx int) []node {
	for len(items) > 0 && items[0].head() == "export" {
		idxNode := numNode(idx)
		c.exports = append(c.exports, exportEntry{
			name: items[0].list[1].atom.str,
			kind: kind,
			idx:  idxNode,
		})
		items = items[1:]
	}
	return items
}

// numNode builds a synthetic numeric index atom
func numNode(idx int) *node {
	return &node{atom: &token{kind: tokNum, text: strconv.Itoa(idx)}}
}

// takeInlineImport pops an optional (import "m" "n") form
func takeInlineImport(items []node) (mod, name []byte, imported bool, rest []node) {
	if len(items) > 0 && items[0].head() == "import" {
		im := items[0].list[1:]
		return im[0].atom.str, im[1].atom.str, true, items[1:]
	}
	return nil, nil, false, items
}

func (c *compiler) collectImport(items []node) error {
	mod, name := items[0].atom.str, items[1].atom.str
	desc := &items[2]
	dItems := desc.list[1:]
	var id string
	id, dItems = takeID(dItems)

	switch desc.head() {
	case "func":
		typeIdx, _, _, err := c.resolveTypeuse(dItems, false)
		if err != nil {
			return err
		}
		c.registerFunc(id, true)
		c.imports = append(c.imports, importEntry{mod: mod, name: name,
			desc: appendUleb([]byte{0x00}, uint64(typeIdx))})
	case "table":
		tt, err := encodeTableType(dItems)
		if err != nil {
			return err
		}
		if id != "" {
			c.tableIDs[id] = c.numImportedTables
		}
		c.numImportedTables++
		c.imports = append(c.imports, importEntry{mod: mod, name: name, desc: append([]byte{0x01}, tt...)})
	case "memory":
		mt, err := encodeLimits(dItems)
		if err != nil {
			return err
		}
		if id != "" {
			c.memIDs[id] = c.numImportedMems
		}
		c.numImportedMems++
		c.imports = append(c.imports, importEntry{mod: mod, name: name, desc: append([]byte{0x02}, mt...)})
	case "global":
		gt := encodeGlobalType(&dItems[0])
		if id != "" {
			c.globalIDs[id] = c.numImportedGlobals
		}
		c.numImportedGlobals++
		c.imports = append(c.imports, importEntry{mod: mod, name: name, desc: append([]byte{0x03}, gt...)})
	}
	return nil
}

func (c *compiler) registerFunc(id string, imported bool) {
	idx := c.numImportedFuncs + len(c.funcs)
	if imported {
		idx = c.numImportedFuncs
		c.numImportedFuncs++
	}
	if id != "" {
		c.funcIDs[id] = idx
	}
}

func (c *compiler) collectFunc(items []node) error {
	id, items := takeID(items)
	items = c.takeInlineExports(items, 0x00, c.numImportedFuncs+len(c.funcs))
	mod, name, imported, items := takeInlineImport(items)

	typeIdx, paramNames, consumed, err := c.resolveTypeuse(items, !imported)
	if err != nil {
		return err
	}
	items = items[consumed:]

	if imported {
		c.registerFunc(id, true)
		c.imports = append(c.imports, importEntry{mod: mod, name: name,
			desc: appendUleb([]byte{0x00}, uint64(typeIdx))})
		return nil
	}

	def := &funcDef{typeIdx: typeIdx, localNames: map[string]int{}}
	for i, n := range paramNames {
		if n != "" {
			def.localNames[n] = i
		}
	}
	def.numParams = len(c.types[typeIdx].params)

	// locals
	for len(items) > 0 && items[0].head() == "local" {
		li := items[0].list[1:]
		if len(li) > 0 && !li[0].isList() && li[0].atom.kind == tokID {
			def.localNames[li[0].atom.text] = def.numParams + len(def.localTypes)
			def.localTypes = append(def.localTypes, valTypeByte(li[1].atom.text))
		} else {
			for i := range li {
				def.localTypes = append(def.localTypes, valTypeByte(li[i].atom.text))
			}
		}
		items = items[1:]
	}

	def.body = items
	c.registerFunc(id, false)
	c.funcs = append(c.funcs, def)
	return nil
}

func (c *compiler) collectTable(items []node) error {
	id, items := takeID(items)
	items = c.takeInlineExports(items, 0x01, c.numImportedTables+len(c.tables))
	mod, name, imported, items := takeInlineImport(items)

	// inline elem abbreviation: (table funcref (elem funcidx*))
	if len(items) == 2 && !items[0].isList() && items[0].atom.text == "funcref" && items[1].head() == "elem" {
		funcs := items[1].list[1:]
		n := uint64(len(funcs))
		if id != "" {
			c.tableIDs[id] = c.numImportedTables + len(c.tables)
		}
		tt := appendUleb(appendUleb([]byte{0x70, 0x01}, n), n) // funcref, min=max=n
		c.tables = append(c.tables, tt)
		c.elems = append(c.elems, elemDef{
			offset: []node{*numConstNode(0)},
			funcs:  funcs,
		})
		return nil
	}

	tt, err := encodeTableType(items)
	if err != nil {
		return err
	}
	if id != "" {
		c.tableIDs[id] = c.numImportedTables + len(c.tables)
	}
	if imported {
		c.tableIDs[id] = c.numImportedTables // fix index for the import case
		c.numImportedTables++
		c.imports = append(c.imports, importEntry{mod: mod, name: name, desc: append([]byte{0x01}, tt...)})
		return nil
	}
	c.tables = append(c.tables, tt)
	return nil
}

func (c *compiler) collectMemory(items []node) error {
	id, items := takeID(items)
	items = c.takeInlineExports(items, 0x02, c.numImportedMems+len(c.mems))
	mod, name, imported, items := takeInlineImport(items)

	// inline data abbreviation: (memory (data "str"*))
	if len(items) == 1 && items[0].head() == "data" {
		var bytes []byte
		for _, s := range items[0].list[1:] {
			bytes = append(bytes, s.atom.str...)
		}
		pages := uint64((len(bytes) + 65535) / 65536)
		if id != "" {
			c.memIDs[id] = c.numImportedMems + len(c.mems)
		}
		c.mems = append(c.mems, appendUleb(appendUleb([]byte{0x01}, pages), pages))
		c.datas = append(c.datas, dataDef{
			offset: []node{*numConstNode(0)},
			bytes:  bytes,
		})
		return nil
	}

	mt, err := encodeLimits(items)
	if err != nil {
		return err
	}
	if id != "" {
		c.memIDs[id] = c.numImportedMems + len(c.mems)
	}
	if imported {
		c.memIDs[id] = c.numImportedMems
		c.numImportedMems++
		c.imports = append(c.imports, importEntry{mod: mod, name: name, desc: append([]byte{0x02}, mt...)})
		return nil
	}
	c.mems = append(c.mems, mt)
	return nil
}

func (c *compiler) collectGlobal(items []node) error {
	id, items := takeID(items)
	items = c.takeInlineExports(items, 0x03, c.numImportedGlobals+len(c.globals))
	mod, name, imported, items := takeInlineImport(items)

	gt := encodeGlobalType(&items[0])
	if id != "" {
		c.globalIDs[id] = c.numImportedGlobals + len(c.globals)
	}
	if imported {
		c.globalIDs[id] = c.numImportedGlobals
		c.numImportedGlobals++
		c.imports = append(c.imports, importEntry{mod: mod, name: name, desc: append([]byte{0x03}, gt...)})
		return nil
	}
	c.globals = append(c.globals, globalDef{typ: gt, init: items[1:]})
	return nil
}

func (c *compiler) collectElem(items []node) error {
	// active form: (elem $id? (table x)? tableidx? (offset|expr) func? funcidx*)
	_, items = takeID(items)

	var table *node
	switch {
	case len(items) > 0 && items[0].head() == "table":
		table = &items[0].list[1]
		items = items[1:]
	case len(items) > 0 && !items[0].isList() && items[0].atom.kind == tokNum:
		table = &items[0]
		items = items[1:]
	}

	if len(items) == 0 || !items[0].isList() {
		return errors.New("unsupported elem form (passive/declarative segments are not supported)")
	}
	offset := unwrapOffset(&items[0])
	items = items[1:]
	if len(items) > 0 && !items[0].isList() && items[0].atom.text == "func" {
		items = items[1:]
	}
	for i := range items {
		if items[i].isList() {
			return errors.New("unsupported elem form (expression entries are not supported)")
		}
	}
	c.elems = append(c.elems, elemDef{table: table, offset: offset, funcs: items})
	return nil
}

func (c *compiler) collectData(items []node) error {
	// active form: (data $id? (memory x)? memidx? (offset|expr) "str"*)
	_, items = takeID(items)

	if len(items) > 0 && items[0].head() == "memory" {
		// only memory 0 exists in the binary MVP data encoding
		idx, err := c.resolveIdx(&items[0].list[1], 0x02)
		if err != nil {
			return err
		}
		if idx != 0 {
			return errors.New("unsupported data form (non-zero memory index)")
		}
		items = items[1:]
	} else if len(items) > 0 && !items[0].isList() && items[0].atom.kind == tokNum {
		items = items[1:] // memory index 0
	}

	if len(items) == 0 || !items[0].isList() || items[0].head() == "" {
		return errors.New("unsupported data form (passive segments are not supported)")
	}
	offset := unwrapOffset(&items[0])
	items = items[1:]
	var bytes []byte
	for i := range items {
		if items[i].isList() || items[i].atom.kind != tokString {
			return errors.New("unsupported data form")
		}
		bytes = append(bytes, items[i].atom.str...)
	}
	c.datas = append(c.datas, dataDef{offset: offset, bytes: bytes})
	return nil
}

// unwrapOffset accepts both (offset e) and the direct instruction abbreviation
func unwrapOffset(n *node) []node {
	if n.head() == "offset" {
		return n.list[1:]
	}
	return []node{*n}
}

// numConstNode builds a synthetic (i32.const n) expression node
func numConstNode(v int) *node {
	return &node{list: []node{
		{atom: &token{kind: tokKeyword, text: "i32.const"}},
		{atom: &token{kind: tokNum, text: strconv.Itoa(v)}},
	}}
}

func encodeLimits(items []node) ([]byte, error) {
	if len(items) == 0 || items[0].isList() || items[0].atom.kind != tokNum {
		return nil, errors.New("unsupported limits form")
	}
	min, err := parseUint(items[0].atom.text, 32)
	if err != nil {
		return nil, err
	}
	if len(items) > 1 && !items[1].isList() && items[1].atom.kind == tokNum {
		max, err := parseUint(items[1].atom.text, 32)
		if err != nil {
			return nil, err
		}
		return appendUleb(appendUleb([]byte{0x01}, min), max), nil
	}
	return appendUleb([]byte{0x00}, min), nil
}

func encodeTableType(items []node) ([]byte, error) {
	// limits funcref -> funcref limits in binary
	lim, err := encodeLimits(items[:len(items)-1])
	if err != nil {
		return nil, err
	}
	return append([]byte{0x70}, lim...), nil
}

func encodeGlobalType(gt *node) []byte {
	if gt.isList() { // (mut valtype)
		return []byte{valTypeByte(gt.list[1].atom.text), 0x01}
	}
	return []byte{valTypeByte(gt.atom.text), 0x00}
}

// ---------------------------------------------------------------------------
// pass B: emission
// ---------------------------------------------------------------------------

func (c *compiler) emit() ([]byte, error) {
	// code section first: emitting bodies may append types (blocktype and
	// call_indirect typeuses), which must land in the type section
	var code []byte
	code = appendUleb(code, uint64(len(c.funcs)))
	for _, def := range c.funcs {
		body, err := c.emitFuncBody(def)
		if err != nil {
			return nil, err
		}
		code = appendUleb(code, uint64(len(body)))
		code = append(code, body...)
	}

	out := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	if len(c.types) > 0 {
		var p []byte
		p = appendUleb(p, uint64(len(c.types)))
		for _, sig := range c.types {
			p = append(p, 0x60)
			p = appendUleb(p, uint64(len(sig.params)))
			for _, t := range sig.params {
				p = append(p, valTypeByte(t))
			}
			p = appendUleb(p, uint64(len(sig.results)))
			for _, t := range sig.results {
				p = append(p, valTypeByte(t))
			}
		}
		out = appendSection(out, 1, p)
	}

	if len(c.imports) > 0 {
		var p []byte
		p = appendUleb(p, uint64(len(c.imports)))
		for _, im := range c.imports {
			p = appendName(p, im.mod)
			p = appendName(p, im.name)
			p = append(p, im.desc...)
		}
		out = appendSection(out, 2, p)
	}

	if len(c.funcs) > 0 {
		var p []byte
		p = appendUleb(p, uint64(len(c.funcs)))
		for _, def := range c.funcs {
			p = appendUleb(p, uint64(def.typeIdx))
		}
		out = appendSection(out, 3, p)
	}

	if len(c.tables) > 0 {
		var p []byte
		p = appendUleb(p, uint64(len(c.tables)))
		for _, tt := range c.tables {
			p = append(p, tt...)
		}
		out = appendSection(out, 4, p)
	}

	if len(c.mems) > 0 {
		var p []byte
		p = appendUleb(p, uint64(len(c.mems)))
		for _, mt := range c.mems {
			p = append(p, mt...)
		}
		out = appendSection(out, 5, p)
	}

	if len(c.globals) > 0 {
		var p []byte
		p = appendUleb(p, uint64(len(c.globals)))
		for _, g := range c.globals {
			p = append(p, g.typ...)
			expr, err := c.emitConstExpr(g.init)
			if err != nil {
				return nil, err
			}
			p = append(p, expr...)
		}
		out = appendSection(out, 6, p)
	}

	if len(c.exports) > 0 {
		var p []byte
		p = appendUleb(p, uint64(len(c.exports)))
		for _, e := range c.exports {
			p = appendName(p, e.name)
			p = append(p, e.kind)
			idx, err := c.resolveIdx(e.idx, e.kind)
			if err != nil {
				return nil, err
			}
			p = appendUleb(p, uint64(idx))
		}
		out = appendSection(out, 7, p)
	}

	if c.start != nil {
		idx, err := c.resolveIdx(c.start, 0x00)
		if err != nil {
			return nil, err
		}
		out = appendSection(out, 8, appendUleb(nil, uint64(idx)))
	}

	if len(c.elems) > 0 {
		var p []byte
		p = appendUleb(p, uint64(len(c.elems)))
		for _, e := range c.elems {
			tableIdx := 0
			if e.table != nil {
				idx, err := c.resolveIdx(e.table, 0x01)
				if err != nil {
					return nil, err
				}
				tableIdx = idx
			}
			if tableIdx == 0 {
				p = append(p, 0x00) // active, table 0
			} else {
				p = appendUleb(append(p, 0x02), uint64(tableIdx)) // active, explicit table
			}
			expr, err := c.emitConstExpr(e.offset)
			if err != nil {
				return nil, err
			}
			p = append(p, expr...)
			if tableIdx != 0 {
				p = append(p, 0x00) // elemkind: funcref
			}
			p = appendUleb(p, uint64(len(e.funcs)))
			for i := range e.funcs {
				idx, err := c.resolveIdx(&e.funcs[i], 0x00)
				if err != nil {
					return nil, err
				}
				p = appendUleb(p, uint64(idx))
			}
		}
		out = appendSection(out, 9, p)
	}

	if len(c.funcs) > 0 {
		out = appendSection(out, 10, code)
	}

	if len(c.datas) > 0 {
		var p []byte
		p = appendUleb(p, uint64(len(c.datas)))
		for _, d := range c.datas {
			p = append(p, 0x00) // active, memory 0
			expr, err := c.emitConstExpr(d.offset)
			if err != nil {
				return nil, err
			}
			p = append(p, expr...)
			p = appendName(p, d.bytes)
		}
		out = appendSection(out, 11, p)
	}

	return out, nil
}

func appendSection(out []byte, id byte, payload []byte) []byte {
	out = append(out, id)
	out = appendUleb(out, uint64(len(payload)))
	return append(out, payload...)
}

// resolveIdx resolves a numeric or named index atom against the space of the
// given export kind (0 func, 1 table, 2 mem, 3 global)
func (c *compiler) resolveIdx(n *node, kind byte) (int, error) {
	t := n.atom
	if t.kind == tokNum {
		v, err := parseUint(t.text, 32)
		return int(v), err
	}
	var m map[string]int
	switch kind {
	case 0x00:
		m = c.funcIDs
	case 0x01:
		m = c.tableIDs
	case 0x02:
		m = c.memIDs
	case 0x03:
		m = c.globalIDs
	}
	idx, ok := m[t.text]
	if !ok {
		return 0, fmt.Errorf("unknown index %s", t.text)
	}
	return idx, nil
}

// emitConstExpr emits a constant expression (global init / segment offset)
func (c *compiler) emitConstExpr(items []node) ([]byte, error) {
	em := &emitter{c: c, def: &funcDef{localNames: map[string]int{}}}
	if err := em.emitInstrSeq(items); err != nil {
		return nil, err
	}
	return append(em.out, 0x0b), nil
}

// ---------------------------------------------------------------------------
// function body emission
// ---------------------------------------------------------------------------

type emitter struct {
	c      *compiler
	def    *funcDef
	labels []string // innermost last, "" = unnamed
	out    []byte
}

func (c *compiler) emitFuncBody(def *funcDef) ([]byte, error) {
	em := &emitter{c: c, def: def}
	if err := em.emitInstrSeq(def.body); err != nil {
		return nil, err
	}

	// locals as RLE (count, type) pairs
	var locals []byte
	runs := 0
	for i := 0; i < len(def.localTypes); {
		j := i
		for j < len(def.localTypes) && def.localTypes[j] == def.localTypes[i] {
			j++
		}
		locals = appendUleb(locals, uint64(j-i))
		locals = append(locals, def.localTypes[i])
		runs++
		i = j
	}

	body := appendUleb(nil, uint64(runs))
	body = append(body, locals...)
	body = append(body, em.out...)
	return append(body, 0x0b), nil
}

func (em *emitter) emitInstrSeq(items []node) error {
	i := 0
	for i < len(items) {
		n, err := em.emitOneInstr(items, i)
		if err != nil {
			return err
		}
		i = n
	}
	return nil
}

func (em *emitter) emitOneInstr(items []node, i int) (int, error) {
	it := &items[i]

	if it.isList() {
		return i + 1, em.emitFolded(it)
	}

	name := it.atom.text
	switch name {
	case "block", "loop", "if":
		return em.emitPlainBlock(items, i)
	}

	info, ok := instrTable[name]
	if !ok {
		return 0, fmt.Errorf("unknown operator %q", name)
	}
	next, err := em.emitOp(name, info, items, i+1)
	if err != nil {
		return 0, err
	}
	return next, nil
}

// blockOpcode returns the opcode of a structured instruction
func blockOpcode(name string) byte {
	switch name {
	case "block":
		return 0x02
	case "loop":
		return 0x03
	default:
		return 0x04 // if
	}
}

// emitBlockHead consumes the label and blocktype, emits opcode + blocktype,
// pushes the label, and returns the next item index
func (em *emitter) emitBlockHead(name string, items []node, i int) (int, error) {
	label := ""
	if i < len(items) && !items[i].isList() && items[i].atom.kind == tokID {
		label = items[i].atom.text
		i++
	}

	// blocktype: empty | (result t) | full typeuse
	typeStart := i
	for i < len(items) {
		if h := items[i].head(); h == "param" || h == "result" || (h == "type" && i == typeStart) {
			i++
		} else {
			break
		}
	}

	em.out = append(em.out, blockOpcode(name))

	blockItems := items[typeStart:i]
	switch {
	case len(blockItems) == 0:
		em.out = append(em.out, 0x40) // empty blocktype
	case len(blockItems) == 1 && blockItems[0].head() == "result" && len(blockItems[0].list) == 2:
		em.out = append(em.out, valTypeByte(blockItems[0].list[1].atom.text))
	default:
		typeIdx, _, _, err := em.c.resolveTypeuse(blockItems, false)
		if err != nil {
			return 0, err
		}
		em.out = appendSleb(em.out, int64(typeIdx))
	}

	em.labels = append(em.labels, label)
	return i, nil
}

func (em *emitter) popLabel() {
	em.labels = em.labels[:len(em.labels)-1]
}

// emitPlainBlock handles plain-form block/loop/if … [else …] end
func (em *emitter) emitPlainBlock(items []node, i int) (int, error) {
	name := items[i].atom.text
	i++
	i, err := em.emitBlockHead(name, items, i)
	if err != nil {
		return 0, err
	}
	label := em.labels[len(em.labels)-1]
	defer em.popLabel()

	for {
		if !items[i].isList() && items[i].atom.kind == tokKeyword {
			switch items[i].atom.text {
			case "end":
				em.out = append(em.out, 0x0b)
				return skipEndLabel(items, i+1, label), nil
			case "else":
				em.out = append(em.out, 0x05)
				i = skipEndLabel(items, i+1, label)
				continue
			}
		}
		i, err = em.emitOneInstr(items, i)
		if err != nil {
			return 0, err
		}
	}
}

func skipEndLabel(items []node, i int, label string) int {
	if i < len(items) && !items[i].isList() && items[i].atom.kind == tokID {
		return i + 1
	}
	return i
}

// emitFolded handles a folded instruction list
func (em *emitter) emitFolded(n *node) error {
	name := n.list[0].atom.text
	items := n.list[1:]

	switch name {
	case "block", "loop":
		next, err := em.emitBlockHead(name, items, 0)
		if err != nil {
			return err
		}
		defer em.popLabel()
		if err := em.emitInstrSeq(items[next:]); err != nil {
			return err
		}
		em.out = append(em.out, 0x0b)
		return nil

	case "if":
		// condition operands come BEFORE the if opcode in binary form, so
		// scan the blocktype first without emitting
		label := ""
		i := 0
		if len(items) > 0 && !items[0].isList() && items[0].atom.kind == tokID {
			label = items[0].atom.text
			i++
		}
		typeStart := i
		for i < len(items) {
			if h := items[i].head(); h == "result" || h == "param" || (h == "type" && i == typeStart) {
				i++
			} else {
				break
			}
		}
		blockItems := items[typeStart:i]

		// folded condition operands
		for i < len(items) && items[i].head() != "then" && items[i].head() != "else" {
			if err := em.emitFolded(&items[i]); err != nil {
				return err
			}
			i++
		}

		em.out = append(em.out, 0x04)
		switch {
		case len(blockItems) == 0:
			em.out = append(em.out, 0x40)
		case len(blockItems) == 1 && blockItems[0].head() == "result" && len(blockItems[0].list) == 2:
			em.out = append(em.out, valTypeByte(blockItems[0].list[1].atom.text))
		default:
			typeIdx, _, _, err := em.c.resolveTypeuse(blockItems, false)
			if err != nil {
				return err
			}
			em.out = appendSleb(em.out, int64(typeIdx))
		}
		em.labels = append(em.labels, label)
		defer em.popLabel()

		if err := em.emitInstrSeq(items[i].list[1:]); err != nil { // (then ...)
			return err
		}
		i++
		if i < len(items) { // (else ...)
			em.out = append(em.out, 0x05)
			if err := em.emitInstrSeq(items[i].list[1:]); err != nil {
				return err
			}
		}
		em.out = append(em.out, 0x0b)
		return nil
	}

	info, ok := instrTable[name]
	if !ok {
		return fmt.Errorf("unknown operator %q", name)
	}

	// immediates come first in text, then folded operand expressions,
	// but operands are emitted BEFORE the op in binary
	immStart := 0
	immEnd, err := em.scanImmediates(items, immStart, info)
	if err != nil {
		return err
	}
	for i := immEnd; i < len(items); i++ {
		if err := em.emitFolded(&items[i]); err != nil {
			return err
		}
	}
	_, err = em.emitOp(name, info, items, immStart)
	return err
}

// scanImmediates returns the index just past the op's immediates without
// emitting anything (checker has already validated them)
func (em *emitter) scanImmediates(items []node, i int, info instrInfo) (int, error) {
	atom := func(j int) *token {
		if j < len(items) && !items[j].isList() {
			return items[j].atom
		}
		return nil
	}
	switch info.imm {
	case immNone, immMemFill, immMemCopy:
		return i, nil
	case immLabel, immFunc, immLocal, immGlobal, immConstI32, immConstI64, immConstF32, immConstF64:
		return i + 1, nil
	case immBrTable:
		for {
			t := atom(i)
			if t == nil || (t.kind != tokNum && t.kind != tokID) {
				return i, nil
			}
			i++
		}
	case immCallIndirect:
		if t := atom(i); t != nil && (t.kind == tokNum || t.kind == tokID) {
			i++
		}
		for i < len(items) {
			if h := items[i].head(); h == "type" || h == "param" || h == "result" {
				i++
			} else {
				break
			}
		}
		return i, nil
	case immMemarg:
		if t := atom(i); t != nil && strings.HasPrefix(t.text, "offset=") {
			i++
		}
		if t := atom(i); t != nil && strings.HasPrefix(t.text, "align=") {
			i++
		}
		return i, nil
	}
	return 0, errors.New("internal: unhandled immediate kind")
}

// emitOp emits opcode + immediates for a non-structured instruction,
// returning the index just past the immediates
func (em *emitter) emitOp(name string, info instrInfo, items []node, i int) (int, error) {
	op, ok := opcodes[name]
	if !ok {
		return 0, fmt.Errorf("no opcode for %q", name)
	}
	if op.prefixed {
		em.out = append(em.out, 0xfc)
		em.out = appendUleb(em.out, uint64(op.code))
	} else {
		em.out = append(em.out, op.code)
	}

	atom := func(j int) *token {
		if j < len(items) && !items[j].isList() {
			return items[j].atom
		}
		return nil
	}

	switch info.imm {
	case immNone:
		if name == "memory.size" || name == "memory.grow" {
			em.out = append(em.out, 0x00)
		}
		return i, nil

	case immMemFill:
		em.out = append(em.out, 0x00) // memory index
		return i, nil

	case immMemCopy:
		em.out = append(em.out, 0x00, 0x00) // dst, src memory indices
		return i, nil

	case immLabel:
		depth, err := em.resolveLabel(atom(i))
		if err != nil {
			return 0, err
		}
		em.out = appendUleb(em.out, uint64(depth))
		return i + 1, nil

	case immBrTable:
		var depths []uint64
		for {
			t := atom(i)
			if t == nil || (t.kind != tokNum && t.kind != tokID) {
				break
			}
			d, err := em.resolveLabel(t)
			if err != nil {
				return 0, err
			}
			depths = append(depths, uint64(d))
			i++
		}
		em.out = appendUleb(em.out, uint64(len(depths)-1))
		for _, d := range depths[:len(depths)-1] {
			em.out = appendUleb(em.out, d)
		}
		em.out = appendUleb(em.out, depths[len(depths)-1])
		return i, nil

	case immFunc:
		idx, err := em.c.resolveIdx(&items[i], 0x00)
		if err != nil {
			return 0, err
		}
		em.out = appendUleb(em.out, uint64(idx))
		return i + 1, nil

	case immLocal:
		t := atom(i)
		var idx int
		if t.kind == tokNum {
			v, err := parseUint(t.text, 32)
			if err != nil {
				return 0, err
			}
			idx = int(v)
		} else {
			var ok bool
			idx, ok = em.def.localNames[t.text]
			if !ok {
				return 0, fmt.Errorf("unknown local %s", t.text)
			}
		}
		em.out = appendUleb(em.out, uint64(idx))
		return i + 1, nil

	case immGlobal:
		idx, err := em.c.resolveIdx(&items[i], 0x03)
		if err != nil {
			return 0, err
		}
		em.out = appendUleb(em.out, uint64(idx))
		return i + 1, nil

	case immCallIndirect:
		tableIdx := 0
		if t := atom(i); t != nil && (t.kind == tokNum || t.kind == tokID) {
			idx, err := em.c.resolveIdx(&items[i], 0x01)
			if err != nil {
				return 0, err
			}
			tableIdx = idx
			i++
		}
		end := i
		for end < len(items) {
			if h := items[end].head(); h == "type" || h == "param" || h == "result" {
				end++
			} else {
				break
			}
		}
		typeIdx, _, _, err := em.c.resolveTypeuse(items[i:end], false)
		if err != nil {
			return 0, err
		}
		em.out = appendUleb(em.out, uint64(typeIdx))
		em.out = appendUleb(em.out, uint64(tableIdx))
		return end, nil

	case immMemarg:
		offset := uint64(0)
		align := info.maxAlign
		if t := atom(i); t != nil && strings.HasPrefix(t.text, "offset=") {
			v, err := parseUint(t.text[len("offset="):], 32)
			if err != nil {
				return 0, err
			}
			offset = v
			i++
		}
		if t := atom(i); t != nil && strings.HasPrefix(t.text, "align=") {
			v, err := parseUint(t.text[len("align="):], 32)
			if err != nil {
				return 0, err
			}
			align = v
			i++
		}
		em.out = appendUleb(em.out, uint64(log2(align)))
		em.out = appendUleb(em.out, offset)
		return i, nil

	case immConstI32:
		v, err := parseInt(atom(i).text, 32)
		if err != nil {
			return 0, err
		}
		em.out = appendSleb(em.out, int64(int32(uint32(v))))
		return i + 1, nil

	case immConstI64:
		v, err := parseInt(atom(i).text, 64)
		if err != nil {
			return 0, err
		}
		em.out = appendSleb(em.out, int64(v))
		return i + 1, nil

	case immConstF32:
		bits, err := parseFloatBits(atom(i).text, 32)
		if err != nil {
			return 0, err
		}
		em.out = append(em.out, byte(bits), byte(bits>>8), byte(bits>>16), byte(bits>>24))
		return i + 1, nil

	case immConstF64:
		bits, err := parseFloatBits(atom(i).text, 64)
		if err != nil {
			return 0, err
		}
		for s := 0; s < 64; s += 8 {
			em.out = append(em.out, byte(bits>>s))
		}
		return i + 1, nil
	}
	return 0, errors.New("internal: unhandled immediate kind")
}

func (em *emitter) resolveLabel(t *token) (int, error) {
	if t.kind == tokNum {
		v, err := parseUint(t.text, 32)
		return int(v), err
	}
	for i := len(em.labels) - 1; i >= 0; i-- {
		if em.labels[i] == t.text {
			return len(em.labels) - 1 - i, nil
		}
	}
	return 0, fmt.Errorf("unknown label %s", t.text)
}

func log2(v uint64) uint64 {
	n := uint64(0)
	for v > 1 {
		v >>= 1
		n++
	}
	return n
}

// parseFloatBits parses a float literal into its IEEE-754 bit pattern
func parseFloatBits(text string, bits uint) (uint64, error) {
	if err := checkFloat(text, bits); err != nil {
		return 0, err
	}

	s := text
	neg := false
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}

	var v uint64
	switch {
	case s == "inf":
		if bits == 32 {
			v = 0x7f800000
		} else {
			v = 0x7ff0000000000000
		}
	case s == "nan":
		if bits == 32 {
			v = 0x7fc00000
		} else {
			v = 0x7ff8000000000000
		}
	case strings.HasPrefix(s, "nan:0x"):
		payload, err := strconv.ParseUint(strings.ReplaceAll(s[6:], "_", ""), 16, 64)
		if err != nil {
			return 0, err
		}
		if bits == 32 {
			v = 0x7f800000 | payload
		} else {
			v = 0x7ff0000000000000 | payload
		}
	default:
		clean := strings.ReplaceAll(s, "_", "")
		if strings.HasPrefix(clean, "0x") && !strings.ContainsAny(clean, "pP") {
			clean += "p0"
		}
		f, err := strconv.ParseFloat(clean, int(bits))
		if err != nil {
			return 0, err
		}
		if bits == 32 {
			v = uint64(math.Float32bits(float32(f)))
		} else {
			v = math.Float64bits(f)
		}
	}

	if neg {
		if bits == 32 {
			v |= 0x80000000
		} else {
			v |= 0x8000000000000000
		}
	}
	return v, nil
}
