//go:build (darwin || linux) && amd64

package jit

import "fmt"

// Compile translates a function body to native amd64 code, or ErrUnsupported
// if it uses constructs outside the template subset (the caller then falls
// back to the interpreter). Same driver as the arm64 backend: the operand
// stack lives at static offsets from SI, heights are tracked at compile time
// and branches become native jumps with result-slot moves.
func Compile(fd *FuncDesc) (*Compiled, error) {
	c := &compiler{fd: fd}
	if err := c.run(); err != nil {
		return nil, err
	}
	code, err := AllocExec(c.a.Bytes())
	if err != nil {
		return nil, err
	}
	return &Compiled{Code: code, MaxHeight: c.maxH}, nil
}

type ctl struct {
	kind      byte // 0x02 block, 0x03 loop, 0x04 if
	entryH    int  // height below the block's params
	paramN    int
	resultN   int
	start     int // code offset of the loop head
	elsePatch int // offset of the pending JZ for an if (-1 when resolved)
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
	oob     []int // JA placeholders jumping to the shared OOB trap stub
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

// moveResults copies the top n slots down to [base, base+n).
func (c *compiler) moveResults(base, n int) {
	src := c.h - n
	if src == base {
		return
	}
	for k := 0; k < n; k++ {
		c.a.LdSlot(rAX, src+k)
		c.a.StSlot(rAX, base+k)
	}
}

// emitBr emits the unwind + jump for a branch to relative depth d.
func (c *compiler) emitBr(d int) {
	fi := len(c.ctl) - 1 - d
	f := &c.ctl[fi]
	if f.kind == 0x03 { // loop: params travel, jump backwards to the head
		c.moveResults(f.entryH, f.paramN)
		c.a.Jmp(f.start)
		return
	}
	c.moveResults(f.entryH, f.resultN)
	f.brPatches = append(f.brPatches, c.a.Len())
	c.a.Jmp(0) // patched at the block's end
}

func (c *compiler) prologue() {
	c.a.LdCtx(rSI, 0)   // stack base
	c.a.LdCtx(rR8, 16)  // locals
	c.a.LdCtx(rR9, 24)  // memory base
	c.a.LdCtx(rR10, 32) // memory length
}

func (c *compiler) epilogueOK(sp int) {
	c.a.MovImm64(rCX, uint64(sp))
	c.a.StCtx(rCX, 8)
	c.a.MovImm32AX(StatusOK)
	c.a.Ret()
}

func (c *compiler) trap(status uint32) {
	c.a.MovImm32AX(status)
	c.a.Ret()
}

// finish emits the success epilogue and the shared out-of-bounds trap stub.
func (c *compiler) finish() error {
	c.epilogueOK(c.h)
	if len(c.oob) > 0 {
		for _, at := range c.oob {
			c.a.PatchJcc(at)
		}
		c.trap(StatusMemOOB)
	}
	return nil
}

func (c *compiler) run() error {
	c.prologue()
	// the function body is an implicit block returning the results
	c.ctl = append(c.ctl, ctl{kind: 0x02, resultN: c.fd.NumRets, elsePatch: -1})
	body := c.fd.Body

	for pc := 0; pc < len(body); pc++ {
		op := body[pc]
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

// opSingleByte / opHasImm are shared with the arm64 backend conceptually;
// duplicated here because each backend is self-contained per GOARCH.
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

var opHasImm = buildOpHasImm()

func buildOpHasImm() (t [256]bool) {
	for _, op := range []byte{
		0x02, 0x03, 0x04, // block/loop/if (packed param/result counts)
		0x0c, 0x0d, // br, br_if
		0x20, 0x21, 0x22, // locals
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

// condition codes (Jcc/SETcc low nibble)
const (
	ccE  = 0x4
	ccNE = 0x5
	ccB  = 0x2
	ccAE = 0x3
	ccA  = 0x7
	ccBE = 0x6
	ccL  = 0xC
	ccGE = 0xD
	ccG  = 0xF
	ccLE = 0xE
)

// wasm comparison opcode -> amd64 condition (order 0x46../0x51..:
// eq ne lt_s lt_u gt_s gt_u le_s le_u ge_s ge_u)
var cmpCond = [10]byte{ccE, ccNE, ccL, ccB, ccG, ccA, ccLE, ccBE, ccGE, ccAE}

func (c *compiler) emit(op byte, imm uint64) error {
	a := &c.a
	switch op {
	case 0x01: // nop

	case 0x00: // unreachable
		c.trap(StatusUnreachable)
		c.unreach = true

	case 0x02, 0x03: // block, loop
		p, r := int(imm>>32), int(imm&0xffffffff)
		c.ctl = append(c.ctl, ctl{kind: op, entryH: c.h - p, paramN: p, resultN: r,
			start: a.Len(), elsePatch: -1})

	case 0x04: // if
		p, r := int(imm>>32), int(imm&0xffffffff)
		a.LdSlot(rAX, c.pop())
		a.TestRR(true, rAX)
		f := ctl{kind: op, entryH: c.h - p, paramN: p, resultN: r, elsePatch: a.Len()}
		a.Jcc(ccE, 0) // patched to the else branch (or end)
		c.ctl = append(c.ctl, f)

	case 0x05: // else
		f := &c.ctl[len(c.ctl)-1]
		f.brPatches = append(f.brPatches, a.Len())
		a.Jmp(0) // then-branch jumps over the else code; patched at end
		a.PatchJcc(f.elsePatch)
		f.elsePatch = -1
		c.h = f.entryH + f.paramN
		c.unreach = false // the false path lands here

	case 0x0b: // end
		f := c.ctl[len(c.ctl)-1]
		c.ctl = c.ctl[:len(c.ctl)-1]
		if f.elsePatch >= 0 { // if without else: false path falls through
			a.PatchJcc(f.elsePatch)
		}
		for _, at := range f.brPatches {
			a.PatchJmp(at)
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
		a.LdSlot(rAX, c.pop())
		a.TestRR(true, rAX)
		at := a.Len()
		a.Jcc(ccE, 0) // skip the branch when the condition is false
		c.emitBr(int(imm))
		a.PatchJcc(at)

	case 0x0f: // return
		c.moveResults(0, c.fd.NumRets)
		c.epilogueOK(c.fd.NumRets)
		c.unreach = true

	case 0x1a: // drop
		c.pop()

	case 0x1b: // select
		a.LdSlot(rDX, c.pop()) // cond
		a.LdSlot(rCX, c.pop()) // v2
		a.LdSlot(rAX, c.pop()) // v1
		a.TestRR(true, rDX)
		a.CmoveRR(rAX, rCX) // cond == 0 -> v2
		a.StSlot(rAX, c.push())

	case 0x20: // local.get
		a.LdLocal(rAX, int(imm))
		a.StSlot(rAX, c.push())
	case 0x21: // local.set
		a.LdSlot(rAX, c.pop())
		a.StLocal(rAX, int(imm))
	case 0x22: // local.tee
		a.LdSlot(rAX, c.h-1)
		a.StLocal(rAX, int(imm))

	case 0x41, 0x42, 0x43, 0x44: // consts (bits pre-decoded)
		a.MovImm64(rAX, imm)
		a.StSlot(rAX, c.push())

	case 0x45: // i32.eqz
		a.LdSlot(rAX, c.h-1)
		a.TestRR(false, rAX)
		a.Setcc(ccE, rAX)
		a.MovzxB(rAX)
		a.StSlot(rAX, c.h-1)
	case 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f: // i32 cmp
		a.LdSlot(rCX, c.pop())
		a.LdSlot(rAX, c.pop())
		a.BinRR(false, 0x39, rAX, rCX) // CMP EAX, ECX
		a.Setcc(cmpCond[op-0x46], rAX)
		a.MovzxB(rAX)
		a.StSlot(rAX, c.push())
	case 0x50: // i64.eqz
		a.LdSlot(rAX, c.h-1)
		a.TestRR(true, rAX)
		a.Setcc(ccE, rAX)
		a.MovzxB(rAX)
		a.StSlot(rAX, c.h-1)
	case 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5a: // i64 cmp
		a.LdSlot(rCX, c.pop())
		a.LdSlot(rAX, c.pop())
		a.BinRR(true, 0x39, rAX, rCX)
		a.Setcc(cmpCond[op-0x51], rAX)
		a.MovzxB(rAX)
		a.StSlot(rAX, c.push())

	// i32 binops (result extension mirrors the interpreter)
	case 0x6a, 0x6b, 0x6c, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76:
		a.LdSlot(rCX, c.pop()) // v2
		a.LdSlot(rAX, c.pop()) // v1
		switch op {
		case 0x6a:
			a.BinRR(false, 0x01, rAX, rCX) // ADD
			a.Movsxd(rAX, rAX)
		case 0x6b:
			a.BinRR(false, 0x29, rAX, rCX) // SUB
			a.Movsxd(rAX, rAX)
		case 0x6c:
			a.Imul(false, rAX, rCX)
			a.Movsxd(rAX, rAX)
		case 0x71:
			a.BinRR(false, 0x21, rAX, rCX) // AND (32-bit op zero-extends)
		case 0x72:
			a.BinRR(false, 0x09, rAX, rCX) // OR
		case 0x73:
			a.BinRR(false, 0x31, rAX, rCX) // XOR
		case 0x74:
			a.ShiftCL(false, 4, rAX) // SHL (count in CL, mod 32)
		case 0x75:
			a.ShiftCL(false, 7, rAX) // SAR
			a.Movsxd(rAX, rAX)
		case 0x76:
			a.ShiftCL(false, 5, rAX) // SHR
		}
		a.StSlot(rAX, c.push())

	// i64 binops
	case 0x7c, 0x7d, 0x7e, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88:
		a.LdSlot(rCX, c.pop())
		a.LdSlot(rAX, c.pop())
		switch op {
		case 0x7c:
			a.BinRR(true, 0x01, rAX, rCX)
		case 0x7d:
			a.BinRR(true, 0x29, rAX, rCX)
		case 0x7e:
			a.Imul(true, rAX, rCX)
		case 0x83:
			a.BinRR(true, 0x21, rAX, rCX)
		case 0x84:
			a.BinRR(true, 0x09, rAX, rCX)
		case 0x85:
			a.BinRR(true, 0x31, rAX, rCX)
		case 0x86:
			a.ShiftCL(true, 4, rAX)
		case 0x87:
			a.ShiftCL(true, 7, rAX)
		case 0x88:
			a.ShiftCL(true, 5, rAX)
		}
		a.StSlot(rAX, c.push())

	// i32/i64 div and rem: divisor-zero (and signed-overflow) checks trap
	// inline via a skip-over pattern
	case 0x6d, 0x6e, 0x6f, 0x70, 0x7f, 0x80, 0x81, 0x82:
		w := op >= 0x7f
		a.LdSlot(rCX, c.pop()) // v2
		a.LdSlot(rAX, c.pop()) // v1
		a.TestRR(w, rCX)
		at := a.Len()
		a.Jcc(ccNE, 0) // skip the trap when the divisor is nonzero
		c.trap(StatusDivZero)
		a.PatchJcc(at)
		signed := op == 0x6d || op == 0x6f || op == 0x7f || op == 0x81
		if signed {
			// MinInt / -1: div_s traps, rem_s must yield 0 (IDIV would fault)
			a.CmpImm32(w, rCX, 0xffffffff)
			ok1 := a.Len()
			a.Jcc(ccNE, 0)
			if w {
				a.MovImm64(rDX, 0x8000000000000000)
				a.BinRR(true, 0x39, rAX, rDX)
			} else {
				a.CmpImm32(false, rAX, 0x80000000)
			}
			ok2 := a.Len()
			a.Jcc(ccNE, 0)
			if op == 0x6d || op == 0x7f { // div_s: trap
				c.trap(StatusIntOverflow)
			} else { // rem_s: result 0
				a.XorDX(true)
				a.StSlot(rDX, c.h)
				done := a.Len()
				a.Jmp(0)
				a.PatchJcc(ok1)
				a.PatchJcc(ok2)
				c.emitDiv(op, w)
				a.PatchJmp(done)
				c.push()
				break
			}
			a.PatchJcc(ok1)
			a.PatchJcc(ok2)
		}
		c.emitDiv(op, w)
		c.push()

	case 0x67, 0x79: // clz: 31/63 - bsr, or width when zero
		w := op == 0x79
		bits := uint32(31)
		if w {
			bits = 63
		}
		a.LdSlot(rAX, c.h-1)
		a.TestRR(w, rAX)
		z := a.Len()
		a.Jcc(ccE, 0)
		a.Bsr(w, rDX, rAX)
		a.MovImm32(rAX, bits)
		a.BinRR(w, 0x29, rAX, rDX) // SUB
		done := a.Len()
		a.Jmp(0)
		a.PatchJcc(z)
		a.MovImm32(rAX, bits+1)
		a.PatchJmp(done)
		a.StSlot(rAX, c.h-1)
	case 0x68, 0x7a: // ctz: bsf, or width when zero
		w := op == 0x7a
		bits := uint32(32)
		if w {
			bits = 64
		}
		a.LdSlot(rAX, c.h-1)
		a.TestRR(w, rAX)
		z := a.Len()
		a.Jcc(ccE, 0)
		a.Bsf(w, rAX, rAX)
		done := a.Len()
		a.Jmp(0)
		a.PatchJcc(z)
		a.MovImm32(rAX, bits)
		a.PatchJmp(done)
		a.StSlot(rAX, c.h-1)
	case 0x69, 0x7b: // popcnt
		w := op == 0x7b
		a.LdSlot(rAX, c.h-1)
		a.Popcnt(w, rAX, rAX)
		a.StSlot(rAX, c.h-1)

	case 0x77, 0x78, 0x89, 0x8a: // rotl, rotr
		w := op >= 0x89
		a.LdSlot(rCX, c.pop())
		a.LdSlot(rAX, c.pop())
		if op == 0x77 || op == 0x89 {
			a.RotCL(w, 0, rAX) // ROL
		} else {
			a.RotCL(w, 1, rAX) // ROR
		}
		a.StSlot(rAX, c.push())

	case 0xc0: // i32.extend8_s (32-bit dst zero-extends to 64)
		a.LdSlot(rAX, c.h-1)
		a.MovsxB(false, rAX, rAX)
		a.StSlot(rAX, c.h-1)
	case 0xc1: // i32.extend16_s
		a.LdSlot(rAX, c.h-1)
		a.MovsxW(false, rAX, rAX)
		a.StSlot(rAX, c.h-1)
	case 0xc2: // i64.extend8_s
		a.LdSlot(rAX, c.h-1)
		a.MovsxB(true, rAX, rAX)
		a.StSlot(rAX, c.h-1)
	case 0xc3: // i64.extend16_s
		a.LdSlot(rAX, c.h-1)
		a.MovsxW(true, rAX, rAX)
		a.StSlot(rAX, c.h-1)
	case 0xc4: // i64.extend32_s
		a.LdSlot(rAX, c.h-1)
		a.Movsxd(rAX, rAX)
		a.StSlot(rAX, c.h-1)

	// conversions
	case 0xa7: // i32.wrap_i64
		a.LdSlot(rAX, c.h-1)
		a.MovRR32(rAX, rAX)
		a.StSlot(rAX, c.h-1)
	case 0xac: // i64.extend_i32_s
		a.LdSlot(rAX, c.h-1)
		a.Movsxd(rAX, rAX)
		a.StSlot(rAX, c.h-1)
	case 0xad: // i64.extend_i32_u
		a.LdSlot(rAX, c.h-1)
		a.MovRR32(rAX, rAX)
		a.StSlot(rAX, c.h-1)

	// loads: pop addr, bounds-check off+addr+width, push extended value
	case 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f,
		0x30, 0x31, 0x32, 0x33, 0x34, 0x35:
		m := memAccess[op-0x28]
		a.LdSlot(rAX, c.pop())
		c.memAddr(imm, m.width)
		a.memSIB(m.pre, m.wide, m.op, rDX, rAX)
		a.StSlot(rDX, c.push())

	// stores: pop value then addr, bounds-check, store truncated
	case 0x36, 0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e:
		m := memAccess[op-0x28]
		a.LdSlot(rDX, c.pop()) // value
		a.LdSlot(rAX, c.pop()) // addr
		c.memAddr(imm, m.width)
		a.memSIB(m.pre, m.wide, m.op, rDX, rAX)

	case 0x3f: // memory.size (pages)
		a.BinRR(true, 0x89, rAX, rR10) // MOV RAX, R10
		a.ShiftImm(true, 5, rAX, 16)   // SHR RAX, 16
		a.StSlot(rAX, c.push())

	default:
		return fmt.Errorf("%w: opcode %#x", ErrUnsupported, op)
	}
	return nil
}

// emitDiv emits the DIV/IDIV core with the result stored at the next slot
// height (operands already in AX/CX, checks already done).
func (c *compiler) emitDiv(op byte, w bool) {
	a := &c.a
	switch op {
	case 0x6d, 0x7f: // div_s
		if w {
			a.Cqo()
		} else {
			a.Cdq()
		}
		a.DivCX(w, 7)
		if !w {
			a.Movsxd(rAX, rAX)
		}
		a.StSlot(rAX, c.h)
	case 0x6e, 0x80: // div_u
		a.XorDX(true)
		a.DivCX(w, 6)
		a.StSlot(rAX, c.h)
	case 0x6f, 0x81: // rem_s
		if w {
			a.Cqo()
		} else {
			a.Cdq()
		}
		a.DivCX(w, 7)
		if !w {
			a.Movsxd(rDX, rDX)
		}
		a.StSlot(rDX, c.h)
	case 0x70, 0x82: // rem_u
		a.XorDX(true)
		a.DivCX(w, 6)
		a.StSlot(rDX, c.h)
	}
}

// memAccess describes loads/stores 0x28..0x3e: access width, legacy prefix,
// REX.W and opcode bytes for the [R9 + reg] form.
var memAccess = [23]struct {
	width uint32
	pre   []byte
	wide  bool
	op    []byte
}{
	{4, nil, false, []byte{0x8B}},          // i32.load        MOV r32
	{8, nil, true, []byte{0x8B}},           // i64.load        MOV r64
	{4, nil, false, []byte{0x8B}},          // f32.load
	{8, nil, true, []byte{0x8B}},           // f64.load
	{1, nil, false, []byte{0x0F, 0xBE}},    // i32.load8_s     MOVSX r32, m8
	{1, nil, false, []byte{0x0F, 0xB6}},    // i32.load8_u     MOVZX r32, m8
	{2, nil, false, []byte{0x0F, 0xBF}},    // i32.load16_s    MOVSX r32, m16
	{2, nil, false, []byte{0x0F, 0xB7}},    // i32.load16_u    MOVZX r32, m16
	{1, nil, true, []byte{0x0F, 0xBE}},     // i64.load8_s     MOVSX r64, m8
	{1, nil, false, []byte{0x0F, 0xB6}},    // i64.load8_u
	{2, nil, true, []byte{0x0F, 0xBF}},     // i64.load16_s
	{2, nil, false, []byte{0x0F, 0xB7}},    // i64.load16_u
	{4, nil, true, []byte{0x63}},           // i64.load32_s    MOVSXD
	{4, nil, false, []byte{0x8B}},          // i64.load32_u
	{4, nil, false, []byte{0x89}},          // i32.store       MOV m32
	{8, nil, true, []byte{0x89}},           // i64.store       MOV m64
	{4, nil, false, []byte{0x89}},          // f32.store
	{8, nil, true, []byte{0x89}},           // f64.store
	{1, nil, false, []byte{0x88}},          // i32.store8      MOV m8
	{2, []byte{0x66}, false, []byte{0x89}}, // i32.store16   MOV m16
	{1, nil, false, []byte{0x88}},          // i64.store8
	{2, []byte{0x66}, false, []byte{0x89}}, // i64.store16
	{4, nil, false, []byte{0x89}},          // i64.store32
}

// memAddr turns the u32 address in RAX into a bounds-checked effective
// address (RAX = offset + addr, trapping when RAX+width > MemLen in R10).
func (c *compiler) memAddr(offset uint64, width uint32) {
	a := &c.a
	a.MovRR32(rAX, rAX) // address operand is unsigned i32
	end := offset + uint64(width)
	if end <= 0x7fffffff {
		a.BinRR(true, 0x89, rCX, rAX) // MOV RCX, RAX
		a.AddImm32(rCX, uint32(end))
	} else {
		a.MovImm64(rCX, end)
		a.BinRR(true, 0x01, rCX, rAX) // ADD RCX, RAX
	}
	a.BinRR(true, 0x39, rCX, rR10) // CMP RCX, R10
	c.oob = append(c.oob, a.Len())
	a.Jcc(ccA, 0) // patched to the shared trap stub
	if offset != 0 {
		if offset <= 0x7fffffff {
			a.AddImm32(rAX, uint32(offset))
		} else {
			a.MovImm64(rCX, offset)
			a.BinRR(true, 0x01, rAX, rCX)
		}
	}
}
