//go:build (darwin || linux) && (arm64 || amd64)

package jit

import "fmt"

// The optimizing tier lowers the wasm stack machine to flat three-address
// code over virtual registers before code generation:
//
//   - local j lives in the fixed vreg j for the whole function
//   - wasm stack position i lives in the fixed vreg nlocals+i at every
//     block boundary (branches move values into these "boundary registers",
//     which replaces SSA phi nodes with register moves)
//   - block-internal values get fresh temporaries
//   - local.get pushes an alias of the local's vreg instead of a copy;
//     aliases are materialized only if the local is reassigned while the
//     alias is still on the stack
//
// Control flow becomes jumps between IR indices (backpatched exactly like
// the machine-code tiers), and loop extents are recorded so liveness can
// extend ranges across back edges.

type irOp uint8

const (
	irMov        irOp = iota // dst = a
	irConst                  // dst = imm
	irBin                    // dst = a <sub> b (sub: wasm opcode)
	irUn                     // dst = <sub> a
	irLoad                   // dst = mem[a + imm] (sub: wasm opcode)
	irStore                  // mem[a + imm] = b (sub: wasm opcode)
	irMemSize                // dst = pages
	irSelect                 // dst = a if c != 0 else b
	irBr                     // jump to imm (IR index, patched)
	irBrIf                   // if a != 0 jump to imm
	irBrIfNot                // if a == 0 jump to imm
	irCallExit               // host exit; imm = call-site id
	irCallNative             // direct native call; imm = funcIdx<<32 | spBefore
	irTrap                   // sub = status
	irRet                    // return; results already canonicalized
	irGlobalGet              // dst = *globals[imm]
	irGlobalSet              // *globals[imm] = a
)

type irInstr struct {
	op   irOp
	sub  byte
	dst  int // vreg, -1 when none
	a, b int // operand vregs, -1 when unused
	c    int // select condition
	imm  uint64
}

// irFunc is the frontend's output.
type irFunc struct {
	code    []irInstr
	nlocals int
	ntemp   int      // total vregs: [0,nlocals) locals, then boundary+temps
	maxH    int      // max wasm stack height (memory-stack slots at exits)
	loops   [][2]int // [start, end) IR ranges of loops, for liveness
	sites   []CallSite
	nrets   int
	sigs    []FuncSig // function index space arities (native call layout)
}

func (fn *irFunc) callSig(idx int) FuncSig { return fn.sigs[idx] }

// lastUses computes the final IR index touching each vreg in one pass
// (codegen consults it per instruction; scanning there would be O(n^2)).
func (fn *irFunc) lastUses() map[int]int {
	last := make(map[int]int)
	for i := range fn.code {
		ins := &fn.code[i]
		if ins.a >= 0 {
			last[ins.a] = i
		}
		if ins.b >= 0 {
			last[ins.b] = i
		}
		if ins.c >= 0 {
			last[ins.c] = i
		}
		if ins.dst >= 0 {
			last[ins.dst] = i
		}
	}
	return last
}

const maxOptHeight = 512 // wasm stack height cap for the fixed boundary map

// stackEnt models one wasm stack slot during lowering.
type stackEnt struct {
	v     int // vreg holding the value
	local int // >= 0 when v aliases that local's vreg
}

type irCtl struct {
	kind      byte
	entryH    int
	paramN    int
	resultN   int
	startIR   int // loop header IR index
	loopIdx   int // index into fn.loops for loops
	elsePatch int
	brPatches []int
}

type irFrontend struct {
	fd      *FuncDesc
	fn      irFunc
	stack   []stackEnt
	ctl     []irCtl
	unreach bool
	skip    int
	op0     int // PC of the opcode being lowered (side-table lookups)
}

func (f *irFrontend) stackReg(i int) int { return f.fn.nlocals + i }

func (f *irFrontend) newTemp() int {
	v := f.fn.nlocals + maxOptHeight + f.fn.ntemp
	f.fn.ntemp++
	return v
}

func (f *irFrontend) emit(i irInstr) int {
	f.fn.code = append(f.fn.code, i)
	return len(f.fn.code) - 1
}

func (f *irFrontend) push(v, local int) {
	f.stack = append(f.stack, stackEnt{v: v, local: local})
	if len(f.stack) > f.fn.maxH {
		f.fn.maxH = len(f.stack)
	}
}

func (f *irFrontend) pop() stackEnt {
	e := f.stack[len(f.stack)-1]
	f.stack = f.stack[:len(f.stack)-1]
	return e
}

// canonicalize moves every live stack slot into its boundary vreg (done
// before every branch, merge point and host exit).
func (f *irFrontend) canonicalize() {
	for i := range f.stack {
		if r := f.stackReg(i); f.stack[i].v != r {
			f.emit(irInstr{op: irMov, dst: r, a: f.stack[i].v, b: -1, c: -1})
			f.stack[i] = stackEnt{v: r, local: -1}
		}
	}
}

// resetStack rebuilds the model at a merge point: height h, all canonical.
func (f *irFrontend) resetStack(h int) {
	f.stack = f.stack[:0]
	for i := 0; i < h; i++ {
		f.stack = append(f.stack, stackEnt{v: f.stackReg(i), local: -1})
	}
	if h > f.fn.maxH {
		f.fn.maxH = h
	}
}

// flushAliases materializes stack aliases of local l before it changes.
func (f *irFrontend) flushAliases(l int) {
	for i := range f.stack {
		if f.stack[i].local == l {
			t := f.newTemp()
			f.emit(irInstr{op: irMov, dst: t, a: l, b: -1, c: -1})
			f.stack[i] = stackEnt{v: t, local: -1}
		}
	}
}

// emitBrIR lowers a branch to relative depth d: move the carried values
// into the target's boundary registers, then jump.
func (f *irFrontend) emitBrIR(d int) {
	t := &f.ctl[len(f.ctl)-1-d]
	n := t.resultN
	if t.kind == 0x03 {
		n = t.paramN
	}
	// move the top n values down to [entryH, entryH+n) boundary registers;
	// go through temporaries only when source/target ranges could overlap
	src := len(f.stack) - n
	for k := 0; k < n; k++ {
		e := f.stack[src+k]
		if r := f.stackReg(t.entryH + k); e.v != r {
			f.emit(irInstr{op: irMov, dst: r, a: e.v, b: -1, c: -1})
		}
	}
	if t.kind == 0x03 {
		f.emit(irInstr{op: irBr, dst: -1, a: -1, b: -1, c: -1, imm: uint64(t.startIR)})
		return
	}
	t.brPatches = append(t.brPatches, f.emit(irInstr{op: irBr, dst: -1, a: -1, b: -1, c: -1}))
}

func (f *irFrontend) patchBr(at int) { f.fn.code[at].imm = uint64(len(f.fn.code)) }

// lower runs the frontend; ErrUnsupported falls back to the baseline tier.
func (f *irFrontend) lower() error {
	fd := f.fd
	f.fn.nlocals = fd.NumLocals
	f.fn.nrets = fd.NumRets
	f.fn.sigs = fd.FuncSigs
	f.ctl = append(f.ctl, irCtl{kind: 0x02, resultN: fd.NumRets, elsePatch: -1})
	body := fd.Body

	for pc := 0; pc < len(body); pc++ {
		op := body[pc]
		f.op0 = pc
		imm := uint64(0)
		if opHasImm[op] {
			imm = fd.Imms[pc]
		}
		if e := fd.PcEnd[pc]; e != 0 {
			pc = int(e)
		} else if !opSingleByte[op] {
			return fmt.Errorf("%w: cannot skip opcode %#x", ErrUnsupported, op)
		}

		if f.unreach {
			switch op {
			case 0x02, 0x03, 0x04:
				f.skip++
				continue
			case 0x05:
				if f.skip > 0 {
					continue
				}
			case 0x0b:
				if f.skip > 0 {
					f.skip--
					continue
				}
			default:
				continue
			}
		}

		if err := f.lowerOp(op, imm); err != nil {
			return err
		}
		if len(f.ctl) == 0 {
			return nil
		}
	}
	if len(f.ctl) == 1 {
		if err := f.lowerOp(0x0b, 0); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("%w: unbalanced blocks", ErrUnsupported)
}

func (f *irFrontend) lowerOp(op byte, imm uint64) error {
	switch op {
	case 0x01: // nop

	case 0x00: // unreachable
		f.emit(irInstr{op: irTrap, sub: StatusUnreachable, dst: -1, a: -1, b: -1, c: -1})
		f.unreach = true

	case 0x02, 0x03: // block, loop
		p, r := int(imm>>32), int(imm&0xffffffff)
		c := irCtl{kind: op, entryH: len(f.stack) - p, paramN: p, resultN: r, elsePatch: -1}
		if op == 0x03 {
			f.canonicalize() // loop entry is a merge point
			c.startIR = len(f.fn.code)
			c.loopIdx = len(f.fn.loops)
			f.fn.loops = append(f.fn.loops, [2]int{c.startIR, -1})
		}
		f.ctl = append(f.ctl, c)

	case 0x04: // if
		cond := f.pop()
		p, r := int(imm>>32), int(imm&0xffffffff)
		f.canonicalize()
		c := irCtl{kind: op, entryH: len(f.stack) - p, paramN: p, resultN: r}
		c.elsePatch = f.emit(irInstr{op: irBrIfNot, dst: -1, a: cond.v, b: -1, c: -1})
		f.ctl = append(f.ctl, c)

	case 0x05: // else
		c := &f.ctl[len(f.ctl)-1]
		if !f.unreach {
			f.canonicalize()
		}
		c.brPatches = append(c.brPatches, f.emit(irInstr{op: irBr, dst: -1, a: -1, b: -1, c: -1}))
		f.patchBr(c.elsePatch)
		c.elsePatch = -1
		f.resetStack(c.entryH + c.paramN)
		f.unreach = false

	case 0x0b: // end
		c := f.ctl[len(f.ctl)-1]
		if !f.unreach {
			// every end is followed by a canonical stack model: blocks and
			// ifs because branches merge here, loops because the fallthrough
			// results must land in their boundary registers
			f.canonicalize()
		}
		f.ctl = f.ctl[:len(f.ctl)-1]
		if c.kind == 0x03 {
			f.fn.loops[c.loopIdx][1] = len(f.fn.code)
		}
		if c.elsePatch >= 0 {
			f.patchBr(c.elsePatch)
		}
		for _, at := range c.brPatches {
			f.patchBr(at)
		}
		if f.unreach {
			f.unreach = len(c.brPatches) == 0 && c.elsePatch < 0 && c.kind != 0x04
		}
		if len(f.ctl) == 0 { // function end
			// the end is a branch target whenever patches landed here, so
			// the return must exist even if the fallthrough was unreachable
			if !f.unreach || len(c.brPatches) > 0 {
				f.emit(irInstr{op: irRet, dst: -1, a: -1, b: -1, c: -1})
			}
			return nil
		}
		f.resetStack(c.entryH + c.resultN)

	case 0x0c: // br
		f.canonicalizeThrough()
		f.emitBrIR(int(imm))
		f.unreach = true

	case 0x0d: // br_if
		cond := f.pop()
		f.canonicalize()
		skip := f.emit(irInstr{op: irBrIfNot, dst: -1, a: cond.v, b: -1, c: -1})
		f.emitBrIR(int(imm))
		f.patchBr(skip)

	case 0x0f: // return
		n := f.fn.nrets
		src := len(f.stack) - n
		for k := 0; k < n; k++ {
			if r := f.stackReg(k); f.stack[src+k].v != r {
				f.emit(irInstr{op: irMov, dst: r, a: f.stack[src+k].v, b: -1, c: -1})
			}
		}
		f.emit(irInstr{op: irRet, dst: -1, a: -1, b: -1, c: -1})
		f.unreach = true

	case 0x1a: // drop
		f.pop()

	case 0x1b: // select
		cond, v2, v1 := f.pop(), f.pop(), f.pop()
		t := f.newTemp()
		f.emit(irInstr{op: irSelect, dst: t, a: v1.v, b: v2.v, c: cond.v})
		f.push(t, -1)

	case 0x0e: // br_table: a compare chain of br_if lowerings
		plan, ok := f.fd.BrTables[f.op0]
		if !ok || len(plan.Targets) > 512 {
			return fmt.Errorf("%w: br_table", ErrUnsupported)
		}
		cond := f.pop()
		f.canonicalize()
		for i, depth := range plan.Targets {
			t := f.newTemp()
			f.emit(irInstr{op: irBinImm, sub: 0x46, dst: t, a: cond.v, b: -1, c: -1, imm: uint64(i)})
			skip := f.emit(irInstr{op: irBrIfNot, dst: -1, a: t, b: -1, c: -1})
			f.emitBrIR(int(depth))
			f.patchBr(skip)
		}
		f.emitBrIR(int(plan.Def))
		f.unreach = true

	case 0x23: // global.get
		t := f.newTemp()
		f.emit(irInstr{op: irGlobalGet, dst: t, a: -1, b: -1, c: -1, imm: imm})
		f.push(t, -1)
	case 0x24: // global.set
		e := f.pop()
		f.emit(irInstr{op: irGlobalSet, dst: -1, a: e.v, b: -1, c: -1, imm: imm})

	case 0x20: // local.get: alias, no copy
		f.push(int(imm), int(imm))
	case 0x21: // local.set
		f.flushAliases(int(imm))
		e := f.pop()
		f.emit(irInstr{op: irMov, dst: int(imm), a: e.v, b: -1, c: -1})
	case 0x22: // local.tee
		f.flushAliases(int(imm))
		e := f.stack[len(f.stack)-1]
		f.emit(irInstr{op: irMov, dst: int(imm), a: e.v, b: -1, c: -1})
		f.stack[len(f.stack)-1] = stackEnt{v: int(imm), local: int(imm)}

	case 0x41, 0x42, 0x43, 0x44: // consts
		t := f.newTemp()
		f.emit(irInstr{op: irConst, dst: t, a: -1, b: -1, c: -1, imm: imm})
		f.push(t, -1)

	case 0x45, 0x50: // eqz
		e := f.pop()
		t := f.newTemp()
		f.emit(irInstr{op: irUn, sub: op, dst: t, a: e.v, b: -1, c: -1})
		f.push(t, -1)

	case 0x67, 0x68, 0x69, 0x79, 0x7a, 0x7b, // clz/ctz/popcnt
		0xa7, 0xac, 0xad, 0xc0, 0xc1, 0xc2, 0xc3, 0xc4: // conversions/extends
		e := f.pop()
		t := f.newTemp()
		f.emit(irInstr{op: irUn, sub: op, dst: t, a: e.v, b: -1, c: -1})
		f.push(t, -1)

	// integer binops and comparisons
	case 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f,
		0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5a,
		0x6a, 0x6b, 0x6c, 0x6d, 0x6e, 0x6f, 0x70, 0x71, 0x72, 0x73,
		0x74, 0x75, 0x76, 0x77, 0x78,
		0x7c, 0x7d, 0x7e, 0x7f, 0x80, 0x81, 0x82, 0x83, 0x84, 0x85,
		0x86, 0x87, 0x88, 0x89, 0x8a:
		v2, v1 := f.pop(), f.pop()
		t := f.newTemp()
		f.emit(irInstr{op: irBin, sub: op, dst: t, a: v1.v, b: v2.v, c: -1})
		f.push(t, -1)

	// loads and stores
	case 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f,
		0x30, 0x31, 0x32, 0x33, 0x34, 0x35:
		addr := f.pop()
		t := f.newTemp()
		f.emit(irInstr{op: irLoad, sub: op, dst: t, a: addr.v, b: -1, c: -1, imm: imm})
		f.push(t, -1)
	case 0x36, 0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e:
		val, addr := f.pop(), f.pop()
		f.emit(irInstr{op: irStore, sub: op, dst: -1, a: addr.v, b: val.v, c: -1, imm: imm})

	case 0x3f: // memory.size
		t := f.newTemp()
		f.emit(irInstr{op: irMemSize, dst: t, a: -1, b: -1, c: -1})
		f.push(t, -1)

	case 0x40: // memory.grow: host exit (pops n, pushes the result)
		f.canonicalize()
		h := len(f.stack)
		site := CallSite{Kind: SiteMemGrow, SpBefore: h, SpAfter: h}
		f.emitCallExit(site, 1, 1)

	case 0x10, 0x11: // calls: native when the target has the native ABI
		var sig FuncSig
		site := CallSite{Kind: SiteCall}
		extra := 0
		if op == 0x10 {
			if int(imm) >= len(f.fd.FuncSigs) {
				return fmt.Errorf("%w: call target %d", ErrUnsupported, imm)
			}
			site.FuncIdx = uint32(imm)
			sig = f.fd.FuncSigs[imm]
			if f.fd.NativeFuncs != nil && f.fd.NativeFuncs[imm] && imm < 4096 {
				f.canonicalize()
				sp := len(f.stack)
				f.emit(irInstr{op: irCallNative, dst: -1, a: -1, b: -1, c: -1,
					imm: imm<<32 | uint64(sp)})
				for i := 0; i < sig.In; i++ {
					f.pop()
				}
				for i := 0; i < sig.Out; i++ {
					f.push(f.stackReg(len(f.stack)), -1)
				}
				return nil
			}
		} else {
			ti, tbl := uint32(imm>>32), uint32(imm)
			if int(ti) >= len(f.fd.TypeSigs) {
				return fmt.Errorf("%w: call_indirect type %d", ErrUnsupported, ti)
			}
			site.Kind = SiteCallIndirect
			site.TypeIdx, site.TableIdx = ti, tbl
			sig = f.fd.TypeSigs[ti]
			extra = 1 // the table index rides on top
		}
		f.canonicalize()
		site.SpBefore = len(f.stack)
		site.SpAfter = site.SpBefore - sig.In - extra + sig.Out
		f.emitCallExit(site, sig.In+extra, sig.Out)

	default:
		return fmt.Errorf("%w: opcode %#x", ErrUnsupported, op)
	}
	return nil
}

// canonicalizeThrough is canonicalize for unconditional branches (the whole
// stack may carry into the target frame).
func (f *irFrontend) canonicalizeThrough() { f.canonicalize() }

// emitCallExit records a host-exit site: the stack is already canonical, so
// values sit in boundary registers matching their memory-slot homes.
func (f *irFrontend) emitCallExit(site CallSite, nin, nout int) {
	id := len(f.fn.sites)
	f.fn.sites = append(f.fn.sites, site)
	f.emit(irInstr{op: irCallExit, dst: -1, a: -1, b: -1, c: -1, imm: uint64(id)})
	for i := 0; i < nin; i++ {
		f.pop()
	}
	for i := 0; i < nout; i++ {
		f.push(f.stackReg(len(f.stack)), -1)
	}
}
