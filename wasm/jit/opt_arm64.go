//go:build (darwin || linux) && arm64

package jit

import (
	"fmt"
	"os"
)

// arm64 code generation for the optimizing tier. Machine registers R8-R15
// hold register-allocated vregs; R6/R7/R16 stage memory-homed operands and
// intermediates (they never overlap the allocatable pool).

const optNumRegs = 8

const optScratch2 = 16 // third scratch, outside the pool and the ABI set

// CompileOpt lowers, allocates and generates native code for fd.
func CompileOpt(fd *FuncDesc) (*Compiled, error) {
	fe := &irFrontend{fd: fd}
	if err := fe.lower(); err != nil {
		return nil, err
	}
	fn := &fe.fn
	fn.peephole()
	al := fn.allocate(optNumRegs)
	if os.Getenv("WASMAN_OPT_DEBUG") == "1" {
		for i, ins := range fn.code {
			fmt.Printf("IR%02d op=%d sub=%#x dst=%d a=%d b=%d imm=%d\n", i, ins.op, ins.sub, ins.dst, ins.a, ins.b, ins.imm)
		}
		for v, ci := range al.compact {
			fmt.Printf("  v%d -> reg=%d spill=%d\n", v, al.loc[ci].reg, al.loc[ci].spill)
		}
	}
	g := &optGen{fn: fn, al: al}
	if err := g.gen(); err != nil {
		return nil, err
	}
	code, err := AllocExec(g.a.Bytes())
	if err != nil {
		return nil, err
	}
	return &Compiled{Code: code, MaxHeight: fn.maxH + al.spillSlots,
		CallSites: fn.sites}, nil
}

type optPatch struct {
	at     int
	target int // IR index
	kind   byte
	reg    uint32 // cbz register
	cond   uint32 // bcond condition
}

type optGen struct {
	fn      *irFunc
	al      *allocation
	a       Asm
	irOff   []int
	patches []optPatch
	oob     []int
	// pending fusion of a flag/zero test into the following branch
	pendV    int
	pendKind byte // 'f' flags+cond, 'z' cbnz on a register
	pendCond uint32
	pendReg  uint32
	pendW    bool
}

func (g *optGen) loc(v int) (int16, bool) {
	ci, ok := g.al.compact[v]
	if !ok {
		return -1, false
	}
	return g.al.loc[ci].reg, true
}

// homeAddr returns the base register and byte offset of v's memory home.
func (g *optGen) homeAddr(v int) (uint32, uint32) {
	kind, idx := g.fn.homeOf(v)
	switch kind {
	case homeLocal:
		return RegLocals, uint32(idx * 8)
	case homeStack:
		return RegStack, uint32(idx * 8)
	default:
		ci := g.al.compact[v]
		return RegStack, uint32(g.al.loc[ci].spill * 8)
	}
}

// read makes v available in a register, staging from memory into scratch.
func (g *optGen) read(v int, scratch uint32) uint32 {
	if r, ok := g.loc(v); ok && r >= 0 {
		return uint32(8 + r)
	}
	base, off := g.homeAddr(v)
	g.a.LdrImm(scratch, base, off)
	return scratch
}

// dst returns the register to compute v into plus a commit step for
// memory-homed destinations.
func (g *optGen) dst(v int, scratch uint32) (uint32, func()) {
	if r, ok := g.loc(v); ok && r >= 0 {
		return uint32(8 + r), func() {}
	}
	base, off := g.homeAddr(v)
	return scratch, func() { g.a.StrImm(scratch, base, off) }
}

func (g *optGen) branchTo(target int, emit func(rel int)) {
	if target < len(g.irOff) && g.irOff[target] >= 0 {
		emit(g.irOff[target] - g.a.Len())
		return
	}
	g.patches = append(g.patches, optPatch{at: g.a.Len(), target: target, kind: 'b'})
	emit(0)
}

func (g *optGen) gen() error {
	fn := g.fn
	g.irOff = make([]int, len(fn.code)+1)
	for i := range g.irOff {
		g.irOff[i] = -1
	}
	g.pendV = -1
	a := &g.a

	a.Prologue()
	// register-allocated locals load once here (args were copied into the
	// frame's locals array; declared locals arrive zeroed)
	for j := 0; j < fn.nlocals; j++ {
		if r, ok := g.loc(j); ok && r >= 0 {
			a.LdrImm(uint32(8+r), RegLocals, uint32(j*8))
		}
	}

	for idx := 0; idx < len(fn.code); idx++ {
		g.irOff[idx] = a.Len()
		ins := &fn.code[idx]
		switch ins.op {
		case irMov:
			if ins.dst == ins.a {
				break
			}
			d, commit := g.dst(ins.dst, RegT0)
			s := g.read(ins.a, RegT0)
			if d != s {
				a.word(0xAA0003E0 | s<<16 | d) // MOV Xd, Xs
			}
			commit()
		case irConst:
			d, commit := g.dst(ins.dst, RegT0)
			a.MovImm64(d, ins.imm)
			commit()
		case irNop:
		case irBin:
			// comparison feeding the next branch fuses into flags
			if isCmpOp(ins.sub) && g.branchFeeds(fn, idx, ins.dst) {
				n := g.read(ins.a, RegT0)
				m := g.read(ins.b, RegT1)
				g.emitCmpFlags(ins.sub, n, m)
				g.setPend(ins.dst, 'f', cmpCondOf(ins.sub), 0, false)
				break
			}
			if err := g.emitBin(ins); err != nil {
				return err
			}
		case irBinImm:
			if isCmpOp(ins.sub) && g.branchFeeds(fn, idx, ins.dst) {
				n := g.read(ins.a, RegT0)
				g.emitCmpImmFlags(ins.sub, n, uint32(ins.imm))
				g.setPend(ins.dst, 'f', cmpCondOf(ins.sub), 0, false)
				break
			}
			if err := g.emitBinImm(ins); err != nil {
				return err
			}
		case irUn:
			// eqz feeding the next branch becomes a bare cbnz: br_if-not on
			// (x == 0) branches exactly when x != 0
			if (ins.sub == 0x45 || ins.sub == 0x50) && g.branchFeeds(fn, idx, ins.dst) {
				r := g.read(ins.a, RegT0)
				g.setPend(ins.dst, 'z', 0, r, ins.sub == 0x50)
				break
			}
			if err := g.emitUn(ins); err != nil {
				return err
			}
		case irLoad, irStore:
			g.emitMem(ins)
		case irMemSize:
			d, commit := g.dst(ins.dst, RegT0)
			a.LsrImmX(d, RegMemLen, 16)
			commit()
		case irSelect:
			cnd := g.read(ins.c, optScratch2)
			a.CmpImmX(cnd, 0)
			v1 := g.read(ins.a, RegT0)
			v2 := g.read(ins.b, RegT1)
			d, commit := g.dst(ins.dst, RegT0)
			a.Csel(d, v1, v2, condNE)
			commit()
		case irBr:
			if g.rotateBackEdge(fn, idx, int(ins.imm)) {
				break
			}
			g.branchTo(int(ins.imm), a.B)
		case irBrIfNot:
			if g.pendV == ins.a && g.pendKind == 'z' { // fused eqz: cbnz
				g.pendV = -1
				t := int(ins.imm)
				if t < len(g.irOff) && g.irOff[t] >= 0 {
					a.Cbnz(g.pendW, g.pendReg, g.irOff[t]-a.Len())
				} else {
					k := byte('n')
					if g.pendW {
						k = 'N'
					}
					g.patches = append(g.patches, optPatch{at: a.Len(), target: t, kind: k, reg: g.pendReg})
					a.Cbnz(g.pendW, g.pendReg, 0)
				}
				break
			}
			if g.pendV == ins.a { // fused compare: branch on the inversion
				g.pendV = -1
				t := int(ins.imm)
				inv := g.pendCond ^ 1
				if t < len(g.irOff) && g.irOff[t] >= 0 {
					a.Bcond(inv, g.irOff[t]-a.Len())
				} else {
					g.patches = append(g.patches, optPatch{at: a.Len(), target: t, kind: 'c', cond: inv})
					a.Bcond(inv, 0)
				}
				break
			}
			r := g.read(ins.a, RegT0)
			t := int(ins.imm)
			if t < len(g.irOff) && g.irOff[t] >= 0 {
				a.Cbz(r, g.irOff[t]-a.Len())
			} else {
				g.patches = append(g.patches, optPatch{at: a.Len(), target: t, kind: 'z', reg: r})
				a.Cbz(r, 0)
			}
		case irCallExit:
			site := &fn.sites[ins.imm]
			a.MovImm64(RegSp, uint64(site.SpBefore))
			a.StrImm(RegSp, RegCtx, 8)
			a.MovImm64(RegT0, ins.imm)
			a.StrImm(RegT0, RegCtx, 40)
			st := uint32(StatusCall)
			if site.Kind == SiteCallIndirect {
				st = StatusCallIndirect
			}
			a.Movz(RegCtx, st, 0)
			a.Ret()
			site.Cont = a.Len()
			a.Prologue()
		case irTrap:
			a.Movz(RegCtx, uint32(ins.sub), 0)
			a.Ret()
		case irRet:
			a.MovImm64(RegSp, uint64(fn.nrets))
			a.Epilogue(StatusOK)
		}
		if ins.op != irBin && ins.op != irBinImm && ins.op != irUn {
			g.pendV = -1
		}
	}
	g.irOff[len(fn.code)] = a.Len()

	for _, p := range g.patches {
		rel := g.irOff[p.target] - p.at
		switch p.kind {
		case 'b':
			a.setWord(p.at, 0x14000000|uint32(rel/4)&0x3ffffff)
		case 'c':
			a.setWord(p.at, 0x54000000|(uint32(rel/4)&0x7ffff)<<5|p.cond)
		case 'z':
			a.setWord(p.at, 0xB4000000|(uint32(rel/4)&0x7ffff)<<5|p.reg)
		case 'n': // cbnz w
			a.setWord(p.at, 0x35000000|(uint32(rel/4)&0x7ffff)<<5|p.reg)
		case 'N': // cbnz x
			a.setWord(p.at, 0xB5000000|(uint32(rel/4)&0x7ffff)<<5|p.reg)
		}
	}
	if len(g.oob) > 0 {
		for _, at := range g.oob {
			a.PatchBcond(at, condHI)
		}
		a.Movz(RegCtx, StatusMemOOB, 0)
		a.Ret()
	}
	return nil
}

// rotateBackEdge rewrites a back edge to a rotatable loop head: the head's
// test runs again at the bottom and branches straight to the body, so the
// steady state takes one branch per iteration instead of two.
func (g *optGen) rotateBackEdge(fn *irFunc, idx, h int) bool {
	if h >= idx { // only backward edges
		return false
	}
	exit, ok := fn.rotatableHead(h, g.lastUse)
	if !ok {
		return false
	}
	if os.Getenv("WASMAN_OPT_DEBUG") == "1" {
		println("ROTATED loop at IR", h)
	}
	a := &g.a
	h0 := &fn.code[h]
	body := g.irOff[h+3] // always emitted before a backward edge
	switch h0.op {
	case irUn: // eqz: continue while the operand is nonzero
		r := g.read(h0.a, RegT0)
		a.Cbnz(h0.sub == 0x50, r, body-a.Len())
	case irBin:
		n := g.read(h0.a, RegT0)
		m := g.read(h0.b, RegT1)
		g.emitCmpFlags(h0.sub, n, m)
		a.Bcond(cmpCondOf(h0.sub)^1, body-a.Len())
	case irBinImm:
		n := g.read(h0.a, RegT0)
		g.emitCmpImmFlags(h0.sub, n, uint32(h0.imm))
		a.Bcond(cmpCondOf(h0.sub)^1, body-a.Len())
	}
	g.branchTo(exit, a.B)
	return true
}

// branchFeeds reports whether code[idx]'s result is consumed solely by an
// immediately following conditional branch (the fusion precondition).
func (g *optGen) branchFeeds(fn *irFunc, idx, dst int) bool {
	if idx+1 >= len(fn.code) {
		return false
	}
	nx := &fn.code[idx+1]
	return nx.op == irBrIfNot && nx.a == dst && g.lastUse(dst) == idx+1
}

func (g *optGen) setPend(v int, kind byte, cond, reg uint32, w bool) {
	g.pendV, g.pendKind, g.pendCond, g.pendReg, g.pendW = v, kind, cond, reg, w
}

// emitCmpImmFlags emits the flags-setting compare against an immediate.
func (g *optGen) emitCmpImmFlags(sub byte, n, imm uint32) {
	if sub >= 0x51 {
		g.a.CmpImmX(n, imm)
	} else {
		g.a.CmpImmW(n, imm)
	}
}

// lastUse reports the final IR index touching v.
func (g *optGen) lastUse(v int) int {
	last := -1
	for i := range g.fn.code {
		ins := &g.fn.code[i]
		if ins.a == v || ins.b == v || ins.c == v || ins.dst == v {
			last = i
		}
	}
	return last
}

func isCmpOp(sub byte) bool {
	return sub >= 0x46 && sub <= 0x4f || sub >= 0x51 && sub <= 0x5a
}

func cmpCondOf(sub byte) uint32 {
	if sub >= 0x51 {
		return cmpCond[sub-0x51]
	}
	return cmpCond[sub-0x46]
}

// emitCmpFlags emits just the flags-setting compare.
func (g *optGen) emitCmpFlags(sub byte, n, m uint32) {
	if sub >= 0x51 {
		g.a.CmpRegX(n, m)
	} else {
		g.a.CmpRegW(n, m)
	}
}
