//go:build (darwin || linux) && arm64

package jit

import "fmt"

// Register-parameterized operation emitters for the optimizing tier: the
// same instruction selection as the baseline tier, but sources and
// destinations are arbitrary registers instead of stack slots.

func (g *optGen) emitBin(ins *irInstr) error {
	a := &g.a
	n := g.read(ins.a, RegT0)
	m := g.read(ins.b, RegT1)
	d, commit := g.dst(ins.dst, RegT0)
	op := ins.sub
	switch op {
	// i32 results stay zero-extended (the W-form write): the i32 contract
	// is loose — every consumer truncates to 32 bits — so the interpreter's
	// per-op sign extension is not re-created here
	case 0x6a:
		a.word(0x0B000000 | m<<16 | n<<5 | d)
	case 0x6b:
		a.word(0x4B000000 | m<<16 | n<<5 | d)
	case 0x6c:
		a.word(0x1B007C00 | m<<16 | n<<5 | d)
	case 0x71:
		a.word(0x0A000000 | m<<16 | n<<5 | d)
	case 0x72:
		a.word(0x2A000000 | m<<16 | n<<5 | d)
	case 0x73:
		a.word(0x4A000000 | m<<16 | n<<5 | d)
	case 0x74:
		a.word(0x1AC02000 | m<<16 | n<<5 | d)
	case 0x75:
		a.word(0x1AC02800 | m<<16 | n<<5 | d)
	case 0x76:
		a.word(0x1AC02400 | m<<16 | n<<5 | d)
	case 0x7c:
		a.word(0x8B000000 | m<<16 | n<<5 | d)
	case 0x7d:
		a.word(0xCB000000 | m<<16 | n<<5 | d)
	case 0x7e:
		a.word(0x9B007C00 | m<<16 | n<<5 | d)
	case 0x83:
		a.word(0x8A000000 | m<<16 | n<<5 | d)
	case 0x84:
		a.word(0xAA000000 | m<<16 | n<<5 | d)
	case 0x85:
		a.word(0xCA000000 | m<<16 | n<<5 | d)
	case 0x86:
		a.word(0x9AC02000 | m<<16 | n<<5 | d)
	case 0x87:
		a.word(0x9AC02800 | m<<16 | n<<5 | d)
	case 0x88:
		a.word(0x9AC02400 | m<<16 | n<<5 | d)

	case 0x77, 0x89: // rotl: negated rotr through a scratch
		w := op == 0x89
		a.Neg(w, optScratch2, m)
		a.Rorv(w, d, n, optScratch2)
	case 0x78, 0x8a:
		a.Rorv(op == 0x8a, d, n, m)

	case 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f:
		a.CmpRegW(n, m)
		a.Cset(d, cmpCond[op-0x46])
	case 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5a:
		a.CmpRegX(n, m)
		a.Cset(d, cmpCond[op-0x51])

	case 0x6d, 0x6e, 0x6f, 0x70, 0x7f, 0x80, 0x81, 0x82:
		g.emitDivRem(op, d, n, m)

	default:
		return fmt.Errorf("%w: bin opcode %#x", ErrUnsupported, op)
	}
	commit()
	return nil
}

// emitDivRem ports the baseline division lowering with explicit registers
// (n dividend, m divisor, d result; optScratch2 stages constants).
func (g *optGen) emitDivRem(op byte, d, n, m uint32) {
	a := &g.a
	w := op >= 0x7f
	if w {
		a.CmpImmX(m, 0)
	} else {
		a.CmpImmW(m, 0)
	}
	a.Bcond(condNE, 12)
	a.Movz(RegStatus, StatusDivZero, 0)
	a.Ret()
	switch op {
	case 0x6d, 0x7f: // div_s with the MinInt/-1 overflow trap
		a.CmnImm(w, m, 1)
		a.Bcond(condNE, 24)
		if w {
			a.Movz(optScratch2, 0x8000, 3)
			a.CmpRegX(n, optScratch2)
		} else {
			a.Movz(optScratch2, 0x8000, 1)
			a.CmpRegW(n, optScratch2)
		}
		a.Bcond(condNE, 12)
		a.Movz(RegStatus, StatusIntOverflow, 0)
		a.Ret()
		a.Sdiv(w, d, n, m)
		if !w {
			a.Sxtw(d, d)
		}
	case 0x6e, 0x80:
		a.Udiv(w, d, n, m)
	case 0x6f, 0x81:
		a.Sdiv(w, optScratch2, n, m)
		a.Msub(w, d, optScratch2, m, n)
		if !w {
			a.Sxtw(d, d)
		}
	case 0x70, 0x82:
		a.Udiv(w, optScratch2, n, m)
		a.Msub(w, d, optScratch2, m, n)
	}
}

func (g *optGen) emitUn(ins *irInstr) error {
	a := &g.a
	s := g.read(ins.a, RegT0)
	d, commit := g.dst(ins.dst, RegT0)
	switch ins.sub {
	case 0x45:
		a.CmpImmW(s, 0)
		a.Cset(d, condEQ)
	case 0x50:
		a.CmpImmX(s, 0)
		a.Cset(d, condEQ)
	case 0x67:
		a.Clz(false, d, s)
	case 0x79:
		a.Clz(true, d, s)
	case 0x68:
		a.Rbit(false, d, s)
		a.Clz(false, d, d)
	case 0x7a:
		a.Rbit(true, d, s)
		a.Clz(true, d, d)
	case 0x69:
		a.Uxtw(optScratch2, s)
		a.Popcnt(d, optScratch2)
	case 0x7b:
		a.Popcnt(d, s)
	case 0xa7, 0xad:
		a.Uxtw(d, s)
	case 0xac, 0xc4:
		a.Sxtw(d, s)
	case 0xc0:
		a.Sxtb(false, d, s)
	case 0xc1:
		a.Sxth(false, d, s)
	case 0xc2:
		a.Sxtb(true, d, s)
	case 0xc3:
		a.Sxth(true, d, s)
	default:
		return fmt.Errorf("%w: un opcode %#x", ErrUnsupported, ins.sub)
	}
	commit()
	return nil
}

// emitMem ports the baseline bounds-checked access with explicit registers:
// the effective address builds in R6, R7 carries the limit check and the
// staged store value.
func (g *optGen) emitMem(ins *irInstr) {
	a := &g.a
	m := memAccess[ins.sub-0x28]
	addr := g.read(ins.a, RegT0)
	a.Uxtw(RegT0, addr) // R6 = zero-extended address (copies pool regs too)
	if end := ins.imm + uint64(m.width); end < 4096 {
		a.AddImm(RegT1, RegT0, uint32(end))
	} else {
		a.MovImm64(RegT1, end)
		a.AddRegX(RegT1, RegT0, RegT1)
	}
	a.CmpRegX(RegT1, RegMemLen)
	g.oob = append(g.oob, a.Len())
	a.Bcond(condHI, 0)
	if ins.imm != 0 {
		if ins.imm < 4096 {
			a.AddImm(RegT0, RegT0, uint32(ins.imm))
		} else {
			a.MovImm64(RegT1, ins.imm)
			a.AddRegX(RegT0, RegT0, RegT1)
		}
	}
	if ins.op == irLoad {
		a.MemOp(m.word, RegT1, RegMem, RegT0)
		d, commit := g.dst(ins.dst, RegT1)
		if d != RegT1 {
			a.word(0xAA0003E0 | RegT1<<16 | d) // MOV Xd, X7
		}
		commit()
		return
	}
	v := g.read(ins.b, RegT1)
	a.MemOp(m.word, v, RegMem, RegT0)
}

// emitBinImm lowers folded-immediate forms (add/sub and materialized
// comparisons; the branch-fused compare path never reaches here).
func (g *optGen) emitBinImm(ins *irInstr) error {
	a := &g.a
	n := g.read(ins.a, RegT0)
	d, commit := g.dst(ins.dst, RegT0)
	imm := uint32(ins.imm)
	switch ins.sub {
	case 0x6a:
		a.AddImmW(d, n, imm)
	case 0x6b:
		a.SubImmW(d, n, imm)
	case 0x7c:
		a.AddImm(d, n, imm)
	case 0x7d:
		a.SubImm(d, n, imm)
	default:
		if !isCmpOp(ins.sub) {
			return errBadImmForm
		}
		g.emitCmpImmFlags(ins.sub, n, imm)
		a.Cset(d, cmpCondOf(ins.sub))
	}
	commit()
	return nil
}

var errBadImmForm = fmt.Errorf("%w: immediate form", ErrUnsupported)
