//go:build (darwin || linux) && amd64

package jit

import "fmt"

// bin3 emits d = n <op> m on the two-address machine: d takes n first,
// then the op folds m in. When d aliases m, AX carries the computation.
func (g *optGen) bin3(w bool, op byte, d, n, m int) int {
	a := &g.a
	if d == m && d != n {
		if n != rAX {
			a.BinRR(true, 0x89, rAX, n)
		}
		a.BinRR(w, op, rAX, m)
		return rAX
	}
	if d != n {
		a.BinRR(true, 0x89, d, n)
	}
	a.BinRR(w, op, d, m)
	return d
}

// viaAX moves a computed AX value into d when they differ.
func (g *optGen) viaAX(d int) {
	if d != rAX {
		g.a.BinRR(true, 0x89, d, rAX)
	}
}

func (g *optGen) emitBin(ins *irInstr) error {
	a := &g.a
	n := g.read(ins.a, rAX)
	m := g.read(ins.b, rCX)
	d, commit := g.dst(ins.dst, rAX)
	op := ins.sub
	switch op {
	case 0x6a, 0x6b, 0x6c, 0x71, 0x72, 0x73, 0x7c, 0x7d, 0x7e,
		0x83, 0x84, 0x85:
		w := op >= 0x7c
		var alu byte
		mul := false
		switch op {
		case 0x6a, 0x7c:
			alu = 0x01
		case 0x6b, 0x7d:
			alu = 0x29
		case 0x6c, 0x7e:
			mul = true
		case 0x71, 0x83:
			alu = 0x21
		case 0x72, 0x84:
			alu = 0x09
		case 0x73, 0x85:
			alu = 0x31
		}
		var r int
		if mul {
			// IMUL is reg,rm (dst-first): stage through AX when d aliases m
			if d == m && d != n {
				a.BinRR(true, 0x89, rAX, n)
				a.Imul(w, rAX, m)
				r = rAX
			} else {
				if d != n {
					a.BinRR(true, 0x89, d, n)
				}
				a.Imul(w, d, m)
				r = d
			}
		} else {
			r = g.bin3(w, alu, d, n, m)
		}
		// i32 results stay zero-extended (the 32-bit write): the loose i32
		// contract needs only the low half
		if !w && op != 0x6a && op != 0x6b && op != 0x6c {
			a.MovRR32(r, r)
		}
		g.viaFrom(d, r)

	case 0x74, 0x75, 0x76, 0x86, 0x87, 0x88, 0x77, 0x78, 0x89, 0x8a:
		// count into CX, value into AX (immune to pool aliasing)
		if m != rCX {
			a.BinRR(true, 0x89, rCX, m)
		}
		if n != rAX {
			a.BinRR(true, 0x89, rAX, n)
		}
		switch op {
		case 0x74:
			a.ShiftCL(false, 4, rAX)
		case 0x75:
			a.ShiftCL(false, 7, rAX)
		case 0x76:
			a.ShiftCL(false, 5, rAX)
		case 0x86:
			a.ShiftCL(true, 4, rAX)
		case 0x87:
			a.ShiftCL(true, 7, rAX)
		case 0x88:
			a.ShiftCL(true, 5, rAX)
		case 0x77:
			a.RotCL(false, 0, rAX)
		case 0x78:
			a.RotCL(false, 1, rAX)
		case 0x89:
			a.RotCL(true, 0, rAX)
		case 0x8a:
			a.RotCL(true, 1, rAX)
		}
		g.viaAX(d)

	case 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f,
		0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5a:
		a.BinRR(op >= 0x51, 0x39, n, m)
		a.Setcc(amdCond(op), rAX)
		a.MovzxB(rAX)
		g.viaAX(d)

	case 0x6d, 0x6e, 0x6f, 0x70, 0x7f, 0x80, 0x81, 0x82:
		g.emitDivRem(op, n, m)
		if op == 0x6f || op == 0x70 || op == 0x81 || op == 0x82 {
			if d != rDX {
				a.BinRR(true, 0x89, d, rDX)
			}
		} else {
			g.viaAX(d)
		}

	default:
		return fmt.Errorf("%w: bin opcode %#x", ErrUnsupported, op)
	}
	commit()
	return nil
}

// viaFrom moves r into d when the computation landed elsewhere.
func (g *optGen) viaFrom(d, r int) {
	if d != r {
		g.a.BinRR(true, 0x89, d, r)
	}
}

// emitDivRem ports the baseline division lowering: dividend in AX,
// divisor staged into CX, quotient in AX / remainder in DX.
func (g *optGen) emitDivRem(op byte, n, m int) {
	a := &g.a
	w := op >= 0x7f
	if m != rCX {
		a.BinRR(true, 0x89, rCX, m)
	}
	if n != rAX {
		a.BinRR(true, 0x89, rAX, n)
	}
	a.TestRR(w, rCX)
	ok := a.Len()
	a.Jcc(ccNE, 0)
	a.MovImm32AX(StatusDivZero)
	a.Ret()
	a.PatchJcc(ok)
	signed := op == 0x6d || op == 0x6f || op == 0x7f || op == 0x81
	if signed {
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
		if op == 0x6d || op == 0x7f {
			a.MovImm32AX(StatusIntOverflow)
			a.Ret()
		} else { // rem_s of MinInt by -1 is 0
			a.XorDX(true)
			done := a.Len()
			a.Jmp(0)
			a.PatchJcc(ok1)
			a.PatchJcc(ok2)
			g.divCore(op, w)
			a.PatchJmp(done)
			return
		}
		a.PatchJcc(ok1)
		a.PatchJcc(ok2)
	}
	g.divCore(op, w)
}

func (g *optGen) divCore(op byte, w bool) {
	a := &g.a
	signed := op == 0x6d || op == 0x6f || op == 0x7f || op == 0x81
	if signed {
		if w {
			a.Cqo()
		} else {
			a.Cdq()
		}
		a.DivCX(w, 7)
	} else {
		a.XorDX(true)
		a.DivCX(w, 6)
	}
	if !w {
		if op == 0x6d {
			a.Movsxd(rAX, rAX)
		} else if op == 0x6f {
			a.Movsxd(rDX, rDX)
		} else if op == 0x6e {
			a.MovRR32(rAX, rAX)
		} else if op == 0x70 {
			a.MovRR32(rDX, rDX)
		}
	}
}

func (g *optGen) emitBinImm(ins *irInstr) error {
	a := &g.a
	n := g.read(ins.a, rAX)
	d, commit := g.dst(ins.dst, rAX)
	imm := uint32(ins.imm)
	switch ins.sub {
	case 0x6a, 0x6b, 0x7c, 0x7d:
		w := ins.sub >= 0x7c
		if d != n {
			a.BinRR(true, 0x89, d, n)
		}
		sub := byte(0)
		if ins.sub == 0x6b || ins.sub == 0x7d {
			sub = 5
		}
		a.ArithImm32(w, sub, d, imm)
	default:
		if !isCmpOp(ins.sub) {
			return fmt.Errorf("%w: immediate form %#x", ErrUnsupported, ins.sub)
		}
		a.CmpImm32(ins.sub >= 0x51, n, imm)
		a.Setcc(amdCond(ins.sub), rAX)
		a.MovzxB(rAX)
		g.viaAX(d)
	}
	commit()
	return nil
}

func (g *optGen) emitUn(ins *irInstr) error {
	a := &g.a
	s := g.read(ins.a, rAX)
	d, commit := g.dst(ins.dst, rAX)
	switch ins.sub {
	case 0x45, 0x50:
		a.TestRR(ins.sub == 0x50, s)
		a.Setcc(ccE, rAX)
		a.MovzxB(rAX)
		g.viaAX(d)
	case 0x67, 0x79: // clz
		w := ins.sub == 0x79
		bits := uint32(31)
		if w {
			bits = 63
		}
		a.TestRR(w, s)
		z := a.Len()
		a.Jcc(ccE, 0)
		a.Bsr(w, rDX, s)
		a.MovImm32(rAX, bits)
		a.BinRR(w, 0x29, rAX, rDX)
		done := a.Len()
		a.Jmp(0)
		a.PatchJcc(z)
		a.MovImm32(rAX, bits+1)
		a.PatchJmp(done)
		g.viaAX(d)
	case 0x68, 0x7a: // ctz
		w := ins.sub == 0x7a
		bits := uint32(32)
		if w {
			bits = 64
		}
		a.TestRR(w, s)
		z := a.Len()
		a.Jcc(ccE, 0)
		a.Bsf(w, rAX, s)
		done := a.Len()
		a.Jmp(0)
		a.PatchJcc(z)
		a.MovImm32(rAX, bits)
		a.PatchJmp(done)
		g.viaAX(d)
	case 0x69, 0x7b:
		a.Popcnt(ins.sub == 0x7b, d, s)
	case 0xa7, 0xad:
		a.MovRR32(d, s)
	case 0xac, 0xc4:
		a.Movsxd(d, s)
	case 0xc0:
		a.MovsxB(false, d, s)
	case 0xc1:
		a.MovsxW(false, d, s)
	case 0xc2:
		a.MovsxB(true, d, s)
	case 0xc3:
		a.MovsxW(true, d, s)
	default:
		return fmt.Errorf("%w: un opcode %#x", ErrUnsupported, ins.sub)
	}
	commit()
	return nil
}

// emitMem ports the bounds-checked access: the effective address builds in
// AX, CX carries the limit check, DX stages load results.
func (g *optGen) emitMem(ins *irInstr) {
	a := &g.a
	m := memAccess[ins.sub-0x28]
	addr := g.read(ins.a, rAX)
	a.MovRR32(rAX, addr) // zero-extended address copy (works for AX too)
	end := ins.imm + uint64(m.width)
	if end <= 0x7fffffff {
		a.BinRR(true, 0x89, rCX, rAX)
		a.ArithImm32(true, 0, rCX, uint32(end))
	} else {
		a.MovImm64(rCX, end)
		a.BinRR(true, 0x01, rCX, rAX)
	}
	a.BinRR(true, 0x39, rCX, rR10)
	g.oob = append(g.oob, a.Len())
	a.Jcc(ccA, 0)
	if ins.imm != 0 {
		if ins.imm <= 0x7fffffff {
			a.ArithImm32(true, 0, rAX, uint32(ins.imm))
		} else {
			a.MovImm64(rCX, ins.imm)
			a.BinRR(true, 0x01, rAX, rCX)
		}
	}
	if ins.op == irLoad {
		a.memSIB(m.pre, m.wide, m.op, rDX, rAX)
		d, commit := g.dst(ins.dst, rDX)
		if d != rDX {
			a.BinRR(true, 0x89, d, rDX)
		}
		commit()
		return
	}
	v := g.read(ins.b, rDX)
	a.memSIB(m.pre, m.wide, m.op, v, rAX)
}
