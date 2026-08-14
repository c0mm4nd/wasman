//go:build (darwin || linux) && arm64

package jit

import "fmt"

// Compile translates a function body to native code, or ErrUnsupported if it
// uses constructs outside the template subset (the caller then falls back to
// the interpreter). The operand stack lives at static offsets from the stack
// base register: heights are tracked at compile time (the body has already
// passed validation, so they are well-defined), branches become native jumps
// with result-slot moves, and the runtime stack index is written only in the
// epilogues.
func Compile(fd *FuncDesc) (*Compiled, error) {
	c := &compiler{fd: fd}
	if err := c.run(); err != nil {
		return nil, err
	}
	code, err := AllocExec(c.a.Bytes())
	if err != nil {
		return nil, err
	}
	return &Compiled{Code: code, MaxHeight: c.maxH, CallSites: c.sites}, nil
}

type ctl struct {
	kind      byte // 0x02 block, 0x03 loop, 0x04 if
	entryH    int  // height below the block's params
	paramN    int
	resultN   int
	start     int // code offset of the loop head
	elsePatch int // offset of the pending CBZ for an if (-1 when resolved)
	brPatches []int
}

type compiler struct {
	fd      *FuncDesc
	a       Asm
	h       int // static operand-stack height (slots)
	maxH    int
	ctl     []ctl
	unreach bool
	skip    int   // nesting depth of blocks opened inside unreachable code
	oob     []int // B.HI placeholders jumping to the shared OOB trap stub
	op0     int   // PC of the opcode being compiled
	sites   []CallSite
}

func (c *compiler) push() int {
	s := c.h
	c.h++
	if c.h > c.maxH {
		c.maxH = c.h
	}
	return s
}

func (c *compiler) pop() int {
	c.h--
	return c.h
}

// slot load/store at a static height
func (c *compiler) ldr(r uint32, slot int) { c.a.LdrImm(r, RegStack, uint32(slot*8)) }
func (c *compiler) str(r uint32, slot int) { c.a.StrImm(r, RegStack, uint32(slot*8)) }

// moveResults copies the top n slots down to [base, base+n) (a branch's
// stack unwind); no-op when they already coincide.
func (c *compiler) moveResults(base, n int) {
	src := c.h - n
	if src == base {
		return
	}
	for k := 0; k < n; k++ {
		c.ldr(RegT0, src+k)
		c.str(RegT0, base+k)
	}
}

// emitBr emits the unwind + jump for a branch to relative depth d.
func (c *compiler) emitBr(d int) {
	fi := len(c.ctl) - 1 - d
	f := &c.ctl[fi]
	if f.kind == 0x03 { // loop: params travel, jump backwards to the head
		c.moveResults(f.entryH, f.paramN)
		c.a.B(f.start - c.a.Len())
		return
	}
	c.moveResults(f.entryH, f.resultN)
	f.brPatches = append(f.brPatches, c.a.Len())
	c.a.B(0) // patched at the block's end
}

func (c *compiler) run() error {
	c.a.Prologue()
	// the function body is an implicit block returning the results
	c.ctl = append(c.ctl, ctl{kind: 0x02, resultN: c.fd.NumRets, elsePatch: -1})
	body := c.fd.Body

	for pc := 0; pc < len(body); pc++ {
		op := body[pc]
		c.op0 = pc // opcode PC, for side tables keyed by position (br_table)
		imm := uint64(0)
		if opHasImm[op] {
			imm = c.fd.Imms[pc]
		}
		// advance past immediates with the engine's pre-decoded table; an
		// opcode with no PcEnd entry that is not known to be immediate-free
		// cannot be skipped safely (e.g. inside unreachable code), so bail
		if e := c.fd.PcEnd[pc]; e != 0 {
			pc = int(e)
		} else if !opSingleByte[op] {
			return fmt.Errorf("%w: cannot skip opcode %#x", ErrUnsupported, op)
		}

		// unreachable code: skip everything but the control skeleton
		if c.unreach {
			switch op {
			case 0x02, 0x03, 0x04:
				c.skip++
				continue
			case 0x05:
				if c.skip > 0 {
					continue
				}
			case 0x0b:
				if c.skip > 0 {
					c.skip--
					continue
				}
			default:
				continue
			}
		}

		if err := c.emit(op, imm); err != nil {
			return err
		}
		if len(c.ctl) == 0 { // the implicit block closed: function end
			return c.finish()
		}
	}
	if len(c.ctl) == 1 {
		// the engine strips the body's final `end`; falling off the end of
		// the byte stream closes the implicit block
		if err := c.emit(0x0b, 0); err != nil {
			return err
		}
		return c.finish()
	}
	return fmt.Errorf("%w: body ended with %d open blocks", ErrUnsupported, len(c.ctl))
}

// finish emits the success epilogue and the shared out-of-bounds trap stub.
func (c *compiler) finish() error {
	c.a.MovImm64(RegSp, uint64(c.h))
	c.a.Epilogue(StatusOK)
	if len(c.oob) > 0 {
		for _, at := range c.oob {
			c.a.PatchBcond(at, condHI)
		}
		c.a.Movz(RegCtx, StatusMemOOB, 0)
		c.a.Ret()
	}
	return nil
}

// opSingleByte marks opcodes known to carry no immediates, so unreachable
// code containing them can be skipped without a PcEnd entry.
var opSingleByte = buildOpSingleByte()

func buildOpSingleByte() (t [256]bool) {
	for _, op := range []byte{0x00, 0x01, 0x05, 0x0b, 0x0f, 0x1a, 0x1b} {
		t[op] = true
	}
	for op := 0x45; op <= 0xc4; op++ { // numeric, comparison and conversion ops
		t[op] = true
	}
	return
}

// opHasImm marks opcodes whose pre-decoded immediate sits in Imms/PcEnd.
var opHasImm = buildOpHasImm()

func buildOpHasImm() (t [256]bool) {
	for _, op := range []byte{
		0x02, 0x03, 0x04, // block/loop/if (packed param/result counts)
		0x0c, 0x0d, // br, br_if
		0x10, 0x11, // call, call_indirect
		0x20, 0x21, 0x22, 0x23, 0x24, // locals, globals
		0x41, 0x42, 0x43, 0x44, // consts
		0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, // loads (offsets)
		0x30, 0x31, 0x32, 0x33, 0x34, 0x35,
		0x36, 0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e, // stores
		0x3f, // memory.size
	} {
		t[op] = true
	}
	return
}

// condition codes
const (
	condEQ = 0
	condNE = 1
	condHS = 2
	condLO = 3
	condHI = 8
	condLS = 9
	condGE = 10
	condLT = 11
	condGT = 12
	condLE = 13
)

// wasm comparison opcode -> arm64 condition (order 0x46.. / 0x51..:
// eq ne lt_s lt_u gt_s gt_u le_s le_u ge_s ge_u)
var cmpCond = [10]uint32{condEQ, condNE, condLT, condLO, condGT, condHI, condLE, condLS, condGE, condHS}

func (c *compiler) emit(op byte, imm uint64) error {
	a := &c.a
	switch op {
	case 0x01: // nop

	case 0x00: // unreachable
		a.Movz(RegCtx, StatusUnreachable, 0)
		a.Ret()
		c.unreach = true

	case 0x02, 0x03: // block, loop
		p, r := int(imm>>32), int(imm&0xffffffff)
		c.ctl = append(c.ctl, ctl{kind: op, entryH: c.h - p, paramN: p, resultN: r,
			start: a.Len(), elsePatch: -1})

	case 0x04: // if
		p, r := int(imm>>32), int(imm&0xffffffff)
		c.ldr(RegT0, c.pop())
		f := ctl{kind: op, entryH: c.h - p, paramN: p, resultN: r, elsePatch: a.Len()}
		a.Cbz(RegT0, 0) // patched to the else branch (or end)
		c.ctl = append(c.ctl, f)

	case 0x05: // else
		f := &c.ctl[len(c.ctl)-1]
		f.brPatches = append(f.brPatches, a.Len())
		a.B(0) // then-branch jumps over the else code; patched at end
		a.PatchCbz(f.elsePatch, RegT0)
		f.elsePatch = -1
		c.h = f.entryH + f.paramN
		c.unreach = false // the false path lands here

	case 0x0b: // end
		f := c.ctl[len(c.ctl)-1]
		c.ctl = c.ctl[:len(c.ctl)-1]
		if f.elsePatch >= 0 { // if without else: false path falls through
			a.PatchCbz(f.elsePatch, RegT0)
		}
		for _, at := range f.brPatches {
			a.PatchB(at)
		}
		c.h = f.entryH + f.resultN
		if c.unreach {
			// reachable again only if something jumps here
			c.unreach = len(f.brPatches) == 0 && f.elsePatch < 0
		}

	case 0x0c: // br
		c.emitBr(int(imm))
		c.unreach = true

	case 0x0d: // br_if
		c.ldr(RegT0, c.pop())
		at := a.Len()
		a.Cbz(RegT0, 0) // skip the branch when the condition is false
		c.emitBr(int(imm))
		a.PatchCbz(at, RegT0)

	case 0x0f: // return
		c.moveResults(0, c.fd.NumRets)
		a.MovImm64(RegSp, uint64(c.fd.NumRets))
		a.Epilogue(StatusOK)
		c.unreach = true

	case 0x1a: // drop
		c.pop()

	case 0x1b: // select
		c.ldr(RegT2, c.pop()) // cond
		c.ldr(RegT1, c.pop()) // v2
		c.ldr(RegT0, c.pop()) // v1
		a.CmpImmX(RegT2, 0)
		a.Csel(RegT0, RegT0, RegT1, condNE)
		c.str(RegT0, c.push())

	// calls exit to the host (which runs the callee through the ordinary
	// call machinery — interpreter, JIT or host function — and re-enters at
	// the continuation, whose fresh prologue reloads every base pointer)
	case 0x10, 0x11:
		site := CallSite{SpBefore: c.h, Indirect: op == 0x11}
		var sig FuncSig
		if op == 0x10 {
			if int(imm) >= len(c.fd.FuncSigs) {
				return fmt.Errorf("%w: call target %d", ErrUnsupported, imm)
			}
			site.FuncIdx = uint32(imm)
			sig = c.fd.FuncSigs[imm]
			c.h -= sig.In
		} else {
			ti, tbl := uint32(imm>>32), uint32(imm)
			if int(ti) >= len(c.fd.TypeSigs) {
				return fmt.Errorf("%w: call_indirect type %d", ErrUnsupported, ti)
			}
			site.TypeIdx, site.TableIdx = ti, tbl
			sig = c.fd.TypeSigs[ti]
			c.h -= sig.In + 1 // the table index rides on top of the args
		}
		c.h += sig.Out
		if c.h > c.maxH {
			c.maxH = c.h
		}
		site.SpAfter = c.h
		a.MovImm64(RegSp, uint64(site.SpBefore))
		a.StrImm(RegSp, RegCtx, 8) // Ctx.Sp
		a.MovImm64(RegT0, uint64(len(c.sites)))
		a.StrImm(RegT0, RegCtx, 40) // Ctx.TrapInfo = site id
		if op == 0x10 {
			a.Movz(RegCtx, StatusCall, 0)
		} else {
			a.Movz(RegCtx, StatusCallIndirect, 0)
		}
		a.Ret()
		site.Cont = a.Len()
		a.Prologue() // the continuation reloads stack/memory bases
		c.sites = append(c.sites, site)

	case 0x0e: // br_table: compare chain over the pre-decoded plan
		plan, ok := c.fd.BrTables[c.op0]
		if !ok || len(plan.Targets) > 512 {
			return fmt.Errorf("%w: br_table", ErrUnsupported)
		}
		c.ldr(RegT0, c.pop())
		patches := make([]int, len(plan.Targets))
		for i := range plan.Targets {
			a.CmpImmW(RegT0, uint32(i))
			patches[i] = a.Len()
			a.Bcond(condEQ, 0) // patched to this index's branch code
		}
		c.emitBr(int(plan.Def))
		for i, depth := range plan.Targets {
			a.PatchBcond(patches[i], condEQ)
			c.emitBr(int(depth))
		}
		c.unreach = true

	case 0x23: // global.get (cells are *uint64, so double indirection)
		a.LdrImm(RegT2, RegCtx, 48) // Ctx.Globals
		a.LdrImm(RegT2, RegT2, uint32(imm*8))
		a.LdrImm(RegT0, RegT2, 0)
		c.str(RegT0, c.push())
	case 0x24: // global.set
		c.ldr(RegT0, c.pop())
		a.LdrImm(RegT2, RegCtx, 48)
		a.LdrImm(RegT2, RegT2, uint32(imm*8))
		a.StrImm(RegT0, RegT2, 0)

	case 0x20: // local.get
		a.LdrImm(RegT0, RegLocals, uint32(imm*8))
		c.str(RegT0, c.push())
	case 0x21: // local.set
		c.ldr(RegT0, c.pop())
		a.StrImm(RegT0, RegLocals, uint32(imm*8))
	case 0x22: // local.tee
		c.ldr(RegT0, c.h-1)
		a.StrImm(RegT0, RegLocals, uint32(imm*8))

	case 0x41, 0x42, 0x43, 0x44: // consts (bits pre-decoded)
		a.MovImm64(RegT0, imm)
		c.str(RegT0, c.push())

	case 0x45: // i32.eqz
		c.ldr(RegT0, c.h-1)
		a.CmpImmW(RegT0, 0)
		a.Cset(RegT0, condEQ)
		c.str(RegT0, c.h-1)
	case 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f: // i32 cmp
		c.ldr(RegT1, c.pop())
		c.ldr(RegT0, c.pop())
		a.CmpRegW(RegT0, RegT1)
		a.Cset(RegT0, cmpCond[op-0x46])
		c.str(RegT0, c.push())
	case 0x50: // i64.eqz
		c.ldr(RegT0, c.h-1)
		a.CmpImmX(RegT0, 0)
		a.Cset(RegT0, condEQ)
		c.str(RegT0, c.h-1)
	case 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5a: // i64 cmp
		c.ldr(RegT1, c.pop())
		c.ldr(RegT0, c.pop())
		a.CmpRegX(RegT0, RegT1)
		a.Cset(RegT0, cmpCond[op-0x51])
		c.str(RegT0, c.push())

	// i32 binops (result extension mirrors the interpreter)
	case 0x6a, 0x6b, 0x6c, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76:
		c.ldr(RegT1, c.pop()) // v2
		c.ldr(RegT0, c.pop()) // v1
		switch op {
		case 0x6a:
			a.word(0x0B000000 | RegT1<<16 | RegT0<<5 | RegT0) // ADD W
			a.Sxtw(RegT0, RegT0)
		case 0x6b:
			a.word(0x4B000000 | RegT1<<16 | RegT0<<5 | RegT0) // SUB W
			a.Sxtw(RegT0, RegT0)
		case 0x6c:
			a.word(0x1B007C00 | RegT1<<16 | RegT0<<5 | RegT0) // MUL W
			a.Sxtw(RegT0, RegT0)
		case 0x71:
			a.word(0x0A000000 | RegT1<<16 | RegT0<<5 | RegT0) // AND W
			a.Uxtw(RegT0, RegT0)
		case 0x72:
			a.word(0x2A000000 | RegT1<<16 | RegT0<<5 | RegT0) // ORR W
			a.Uxtw(RegT0, RegT0)
		case 0x73:
			a.word(0x4A000000 | RegT1<<16 | RegT0<<5 | RegT0) // EOR W
			a.Uxtw(RegT0, RegT0)
		case 0x74:
			a.word(0x1AC02000 | RegT1<<16 | RegT0<<5 | RegT0) // LSLV W
			a.Uxtw(RegT0, RegT0)
		case 0x75:
			a.word(0x1AC02800 | RegT1<<16 | RegT0<<5 | RegT0) // ASRV W
			a.Sxtw(RegT0, RegT0)
		case 0x76:
			a.word(0x1AC02400 | RegT1<<16 | RegT0<<5 | RegT0) // LSRV W
			a.Uxtw(RegT0, RegT0)
		}
		c.str(RegT0, c.push())

	// i64 binops
	case 0x7c, 0x7d, 0x7e, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88:
		c.ldr(RegT1, c.pop())
		c.ldr(RegT0, c.pop())
		switch op {
		case 0x7c:
			a.word(0x8B000000 | RegT1<<16 | RegT0<<5 | RegT0) // ADD X
		case 0x7d:
			a.word(0xCB000000 | RegT1<<16 | RegT0<<5 | RegT0) // SUB X
		case 0x7e:
			a.word(0x9B007C00 | RegT1<<16 | RegT0<<5 | RegT0) // MUL X
		case 0x83:
			a.word(0x8A000000 | RegT1<<16 | RegT0<<5 | RegT0) // AND X
		case 0x84:
			a.word(0xAA000000 | RegT1<<16 | RegT0<<5 | RegT0) // ORR X
		case 0x85:
			a.word(0xCA000000 | RegT1<<16 | RegT0<<5 | RegT0) // EOR X
		case 0x86:
			a.word(0x9AC02000 | RegT1<<16 | RegT0<<5 | RegT0) // LSLV X
		case 0x87:
			a.word(0x9AC02800 | RegT1<<16 | RegT0<<5 | RegT0) // ASRV X
		case 0x88:
			a.word(0x9AC02400 | RegT1<<16 | RegT0<<5 | RegT0) // LSRV X
		}
		c.str(RegT0, c.push())

	// i32/i64 div and rem: the zero (and signed-overflow) checks trap inline
	// via a skip-over pattern, so no patch bookkeeping is needed
	case 0x6d, 0x6e, 0x6f, 0x70, 0x7f, 0x80, 0x81, 0x82:
		w := op >= 0x7f
		c.ldr(RegT1, c.pop()) // v2
		c.ldr(RegT0, c.pop()) // v1
		if w {
			a.CmpImmX(RegT1, 0)
		} else {
			a.CmpImmW(RegT1, 0)
		}
		a.Bcond(condNE, 12) // skip the trap when the divisor is nonzero
		a.Movz(RegCtx, StatusDivZero, 0)
		a.Ret()
		switch op {
		case 0x6d, 0x7f: // div_s: MinInt / -1 overflows
			a.CmnImm(w, RegT1, 1)
			a.Bcond(condNE, 24) // skip the 5-word overflow check + trap
			if w {
				a.Movz(RegT2, 0x8000, 3) // MinInt64
				a.CmpRegX(RegT0, RegT2)
			} else {
				a.Movz(RegT2, 0x8000, 1) // MinInt32 (W compare)
				a.CmpRegW(RegT0, RegT2)
			}
			a.Bcond(condNE, 12)
			a.Movz(RegCtx, StatusIntOverflow, 0)
			a.Ret()
			a.Sdiv(w, RegT0, RegT0, RegT1)
			if !w {
				a.Sxtw(RegT0, RegT0)
			}
		case 0x6e, 0x80: // div_u
			a.Udiv(w, RegT0, RegT0, RegT1)
		case 0x6f, 0x81: // rem_s (MinInt % -1 is 0 in hardware via msub)
			a.Sdiv(w, RegT2, RegT0, RegT1)
			a.Msub(w, RegT0, RegT2, RegT1, RegT0)
			if !w {
				a.Sxtw(RegT0, RegT0)
			}
		case 0x70, 0x82: // rem_u
			a.Udiv(w, RegT2, RegT0, RegT1)
			a.Msub(w, RegT0, RegT2, RegT1, RegT0)
		}
		c.str(RegT0, c.push())

	case 0x67, 0x79: // clz
		c.ldr(RegT0, c.h-1)
		a.Clz(op == 0x79, RegT0, RegT0)
		c.str(RegT0, c.h-1)
	case 0x68, 0x7a: // ctz = clz(rbit(x))
		c.ldr(RegT0, c.h-1)
		w := op == 0x7a
		a.Rbit(w, RegT0, RegT0)
		a.Clz(w, RegT0, RegT0)
		c.str(RegT0, c.h-1)
	case 0x69, 0x7b: // popcnt (NEON CNT+ADDV)
		c.ldr(RegT0, c.h-1)
		if op == 0x69 {
			a.Uxtw(RegT0, RegT0)
		}
		a.Popcnt(RegT0, RegT0)
		c.str(RegT0, c.h-1)

	case 0x77, 0x78, 0x89, 0x8a: // rotl (via negated rotr), rotr
		w := op >= 0x89
		c.ldr(RegT1, c.pop())
		c.ldr(RegT0, c.pop())
		if op == 0x77 || op == 0x89 { // rotl x,n == rotr x,-n
			a.Neg(w, RegT1, RegT1)
		}
		a.Rorv(w, RegT0, RegT0, RegT1)
		c.str(RegT0, c.push())

	case 0xc0: // i32.extend8_s
		c.ldr(RegT0, c.h-1)
		a.Sxtb(false, RegT0, RegT0)
		c.str(RegT0, c.h-1)
	case 0xc1: // i32.extend16_s
		c.ldr(RegT0, c.h-1)
		a.Sxth(false, RegT0, RegT0)
		c.str(RegT0, c.h-1)
	case 0xc2: // i64.extend8_s
		c.ldr(RegT0, c.h-1)
		a.Sxtb(true, RegT0, RegT0)
		c.str(RegT0, c.h-1)
	case 0xc3: // i64.extend16_s
		c.ldr(RegT0, c.h-1)
		a.Sxth(true, RegT0, RegT0)
		c.str(RegT0, c.h-1)
	case 0xc4: // i64.extend32_s
		c.ldr(RegT0, c.h-1)
		a.Sxtw(RegT0, RegT0)
		c.str(RegT0, c.h-1)

	// conversions
	case 0xa7: // i32.wrap_i64
		c.ldr(RegT0, c.h-1)
		a.Uxtw(RegT0, RegT0)
		c.str(RegT0, c.h-1)
	case 0xac: // i64.extend_i32_s
		c.ldr(RegT0, c.h-1)
		a.Sxtw(RegT0, RegT0)
		c.str(RegT0, c.h-1)
	case 0xad: // i64.extend_i32_u
		c.ldr(RegT0, c.h-1)
		a.Uxtw(RegT0, RegT0)
		c.str(RegT0, c.h-1)

	// loads: pop addr, bounds-check off+addr+width, push extended value
	case 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f,
		0x30, 0x31, 0x32, 0x33, 0x34, 0x35:
		m := memAccess[op-0x28]
		c.ldr(RegT0, c.pop())
		c.memAddr(imm, m.width)
		a.MemOp(m.word, RegT2, RegMem, RegT0)
		c.str(RegT2, c.push())

	// stores: pop value then addr, bounds-check, store truncated
	case 0x36, 0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e:
		m := memAccess[op-0x28]
		c.ldr(RegT2, c.pop()) // value
		c.ldr(RegT0, c.pop()) // addr
		c.memAddr(imm, m.width)
		a.MemOp(m.word, RegT2, RegMem, RegT0)

	case 0x3f: // memory.size (pages; the length register is loop-invariant)
		a.LsrImmX(RegT0, RegMemLen, 16)
		c.str(RegT0, c.push())

	default:
		return fmt.Errorf("%w: opcode %#x", ErrUnsupported, op)
	}
	return nil
}

// memAccess describes loads/stores 0x28..0x3e: access width and the
// register-offset instruction encoding ([Xn + Xm]).
var memAccess = [23]struct {
	width uint32
	word  uint32
}{
	{4, 0xB8606800}, // i32.load        LDR  W
	{8, 0xF8606800}, // i64.load        LDR  X
	{4, 0xB8606800}, // f32.load        LDR  W
	{8, 0xF8606800}, // f64.load        LDR  X
	{1, 0x38E06800}, // i32.load8_s     LDRSB W (sign to 32, zero to 64)
	{1, 0x38606800}, // i32.load8_u     LDRB  W
	{2, 0x78E06800}, // i32.load16_s    LDRSH W
	{2, 0x78606800}, // i32.load16_u    LDRH  W
	{1, 0x38A06800}, // i64.load8_s     LDRSB X
	{1, 0x38606800}, // i64.load8_u     LDRB  W
	{2, 0x78A06800}, // i64.load16_s    LDRSH X
	{2, 0x78606800}, // i64.load16_u    LDRH  W
	{4, 0xB8A06800}, // i64.load32_s    LDRSW X
	{4, 0xB8606800}, // i64.load32_u    LDR   W
	{4, 0xB8206800}, // i32.store       STR  W
	{8, 0xF8206800}, // i64.store       STR  X
	{4, 0xB8206800}, // f32.store       STR  W
	{8, 0xF8206800}, // f64.store       STR  X
	{1, 0x38206800}, // i32.store8      STRB
	{2, 0x78206800}, // i32.store16     STRH
	{1, 0x38206800}, // i64.store8      STRB
	{2, 0x78206800}, // i64.store16     STRH
	{4, 0xB8206800}, // i64.store32     STR  W
}

// memAddr turns the u32 address in RegT0 into a bounds-checked effective
// address (RegT0 = offset + addr, trapping when RegT0+width > MemLen).
func (c *compiler) memAddr(offset uint64, width uint32) {
	a := &c.a
	a.Uxtw(RegT0, RegT0) // address operand is unsigned i32
	if end := offset + uint64(width); end < 4096 {
		a.AddImm(RegT1, RegT0, uint32(end))
	} else {
		a.MovImm64(RegT1, end)
		a.AddRegX(RegT1, RegT0, RegT1)
	}
	a.CmpRegX(RegT1, RegMemLen)
	c.oob = append(c.oob, a.Len())
	a.Bcond(condHI, 0) // patched to the shared trap stub
	if offset != 0 {
		if offset < 4096 {
			a.AddImm(RegT0, RegT0, uint32(offset))
		} else {
			a.MovImm64(RegT1, offset)
			a.AddRegX(RegT0, RegT0, RegT1)
		}
	}
}
