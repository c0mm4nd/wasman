//go:build (darwin || linux) && amd64

package jit

import (
	"fmt"
	"os"
)

// amd64 code generation for the optimizing tier. Pool registers BX, R11,
// R12, R13, R15 hold allocated vregs (R14 is the goroutine register, BP
// frames Go tracebacks — both untouched); AX/CX/DX stage memory-homed
// operands and serve the fixed-register instructions (shifts via CL,
// division via AX:DX).

const optNumRegs = 5

var optPool = [optNumRegs]int{3, 11, 12, 13, 15} // BX, R11, R12, R13, R15

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
}

type optGen struct {
	fn      *irFunc
	al      *allocation
	a       Asm
	irOff   []int
	patches []optPatch
	oob     []int
	// pending fusion of a test into the following branch
	pendV    int
	pendKind byte // 'f' flags+cc, 'z' test-and-jnz
	pendCC   byte
	pendReg  int
	pendW    bool
}

func (g *optGen) loc(v int) (int16, bool) {
	ci, ok := g.al.compact[v]
	if !ok {
		return -1, false
	}
	return g.al.loc[ci].reg, true
}

func (g *optGen) homeAddr(v int) (int, int32) {
	kind, idx := g.fn.homeOf(v)
	switch kind {
	case homeLocal:
		return rR8, int32(idx * 8)
	case homeStack:
		return rSI, int32(idx * 8)
	default:
		ci := g.al.compact[v]
		return rSI, g.al.loc[ci].spill * 8
	}
}

// read makes v available in a register, staging from memory into scratch.
func (g *optGen) read(v int, scratch int) int {
	if r, ok := g.loc(v); ok && r >= 0 {
		return optPool[r]
	}
	base, off := g.homeAddr(v)
	g.a.modDisp32(true, 0x8B, scratch, base, off)
	return scratch
}

func (g *optGen) dst(v int, scratch int) (int, func()) {
	if r, ok := g.loc(v); ok && r >= 0 {
		return optPool[r], func() {}
	}
	base, off := g.homeAddr(v)
	return scratch, func() { g.a.modDisp32(true, 0x89, scratch, base, off) }
}

func (g *optGen) prologue() {
	g.a.LdCtx(rSI, 0)
	g.a.LdCtx(rR8, 16)
	g.a.LdCtx(rR9, 24)
	g.a.LdCtx(rR10, 32)
}

func (g *optGen) jumpTo(target int, emitJ func(int), kind byte) {
	if target < len(g.irOff) && g.irOff[target] >= 0 {
		emitJ(g.irOff[target])
		return
	}
	g.patches = append(g.patches, optPatch{at: g.a.Len(), target: target, kind: kind})
	emitJ(g.a.Len() + 16) // placeholder; rewritten by the patch pass
}

func (g *optGen) gen() error {
	fn := g.fn
	g.irOff = make([]int, len(fn.code)+1)
	for i := range g.irOff {
		g.irOff[i] = -1
	}
	g.pendV = -1
	a := &g.a

	g.prologue()
	for j := 0; j < fn.nlocals; j++ {
		if r, ok := g.loc(j); ok && r >= 0 {
			a.modDisp32(true, 0x8B, optPool[r], rR8, int32(j*8))
		}
	}

	for idx := 0; idx < len(fn.code); idx++ {
		g.irOff[idx] = a.Len()
		ins := &fn.code[idx]
		switch ins.op {
		case irNop:
		case irMov:
			if ins.dst == ins.a {
				break
			}
			d, commit := g.dst(ins.dst, rAX)
			s := g.read(ins.a, rAX)
			if d != s {
				a.BinRR(true, 0x89, d, s)
			}
			commit()
		case irConst:
			d, commit := g.dst(ins.dst, rAX)
			a.MovImm64(d, ins.imm)
			commit()
		case irBin:
			if isCmpOp(ins.sub) && g.branchFeeds(fn, idx, ins.dst) {
				n := g.read(ins.a, rAX)
				m := g.read(ins.b, rCX)
				a.BinRR(ins.sub >= 0x51, 0x39, n, m)
				g.setPend(ins.dst, 'f', amdCond(ins.sub), 0, false)
				break
			}
			if err := g.emitBin(ins); err != nil {
				return err
			}
		case irBinImm:
			if isCmpOp(ins.sub) && g.branchFeeds(fn, idx, ins.dst) {
				n := g.read(ins.a, rAX)
				a.CmpImm32(ins.sub >= 0x51, n, uint32(ins.imm))
				g.setPend(ins.dst, 'f', amdCond(ins.sub), 0, false)
				break
			}
			if err := g.emitBinImm(ins); err != nil {
				return err
			}
		case irUn:
			if (ins.sub == 0x45 || ins.sub == 0x50) && g.branchFeeds(fn, idx, ins.dst) {
				r := g.read(ins.a, rAX)
				g.setPend(ins.dst, 'z', 0, r, ins.sub == 0x50)
				break
			}
			if err := g.emitUn(ins); err != nil {
				return err
			}
		case irLoad, irStore:
			g.emitMem(ins)
		case irMemSize:
			d, commit := g.dst(ins.dst, rAX)
			a.BinRR(true, 0x89, d, rR10)
			a.ShiftImm(true, 5, d, 16)
			commit()
		case irSelect:
			cnd := g.read(ins.c, rDX)
			a.TestRR(true, cnd)
			v1 := g.read(ins.a, rAX)
			v2 := g.read(ins.b, rCX)
			d, commit := g.dst(ins.dst, rAX)
			if d != v1 {
				a.BinRR(true, 0x89, d, v1)
			}
			a.CmoveRR(d, v2)
			commit()
		case irBr:
			if g.rotateBackEdge(fn, idx, int(ins.imm)) {
				break
			}
			g.jumpTo(int(ins.imm), a.Jmp, 'b')
		case irBrIfNot:
			if g.pendV == ins.a && g.pendKind == 'z' {
				g.pendV = -1
				a.TestRR(g.pendW, g.pendReg)
				g.jumpTo(int(ins.imm), func(t int) { a.Jcc(ccNE, t) }, 'c')
				break
			}
			if g.pendV == ins.a {
				g.pendV = -1
				inv := g.pendCC ^ 1
				g.jumpTo(int(ins.imm), func(t int) { a.Jcc(inv, t) }, 'c')
				break
			}
			r := g.read(ins.a, rAX)
			a.TestRR(true, r)
			g.jumpTo(int(ins.imm), func(t int) { a.Jcc(ccE, t) }, 'c')
		case irCallExit:
			site := &fn.sites[ins.imm]
			a.MovImm64(rCX, uint64(site.SpBefore))
			a.StCtx(rCX, 8)
			a.MovImm64(rCX, ins.imm)
			a.StCtx(rCX, 40)
			st := uint32(StatusCall)
			if site.Kind == SiteCallIndirect {
				st = StatusCallIndirect
			}
			a.MovImm32AX(st)
			a.Ret()
			site.Cont = a.Len()
			g.prologue()
		case irTrap:
			a.MovImm32AX(uint32(ins.sub))
			a.Ret()
		case irRet:
			a.MovImm64(rCX, uint64(fn.nrets))
			a.StCtx(rCX, 8)
			a.MovImm32AX(StatusOK)
			a.Ret()
		}
		if ins.op != irBin && ins.op != irBinImm && ins.op != irUn {
			g.pendV = -1
		}
	}
	g.irOff[len(fn.code)] = a.Len()

	for _, p := range g.patches {
		switch p.kind {
		case 'b':
			g.a.PatchJmpTo(p.at, g.irOff[p.target])
		case 'c':
			g.a.PatchJccTo(p.at, g.irOff[p.target])
		}
	}
	if len(g.oob) > 0 {
		for _, at := range g.oob {
			a.PatchJcc(at)
		}
		a.MovImm32AX(StatusMemOOB)
		a.Ret()
	}
	return nil
}

// rotateBackEdge rewrites a back edge to a rotatable loop head: the head's
// test runs again at the bottom and branches straight to the body, so the
// steady state takes one branch per iteration instead of two.
func (g *optGen) rotateBackEdge(fn *irFunc, idx, h int) bool {
	if h >= idx {
		return false
	}
	exit, ok := fn.rotatableHead(h, g.lastUse)
	if !ok {
		return false
	}
	a := &g.a
	h0 := &fn.code[h]
	body := g.irOff[h+3]
	switch h0.op {
	case irUn:
		r := g.read(h0.a, rAX)
		a.TestRR(h0.sub == 0x50, r)
		a.Jcc(ccNE, body)
	case irBin:
		n := g.read(h0.a, rAX)
		m := g.read(h0.b, rCX)
		a.BinRR(h0.sub >= 0x51, 0x39, n, m)
		a.Jcc(amdCond(h0.sub)^1, body)
	case irBinImm:
		n := g.read(h0.a, rAX)
		a.CmpImm32(h0.sub >= 0x51, n, uint32(h0.imm))
		a.Jcc(amdCond(h0.sub)^1, body)
	}
	g.jumpTo(exit, a.Jmp, 'b')
	return true
}

func (g *optGen) branchFeeds(fn *irFunc, idx, dst int) bool {
	if idx+1 >= len(fn.code) {
		return false
	}
	nx := &fn.code[idx+1]
	return nx.op == irBrIfNot && nx.a == dst && g.lastUse(dst) == idx+1
}

func (g *optGen) setPend(v int, kind byte, cc byte, reg int, w bool) {
	g.pendV, g.pendKind, g.pendCC, g.pendReg, g.pendW = v, kind, cc, reg, w
}

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

// amdCond maps a wasm comparison opcode to the amd64 condition nibble.
func amdCond(sub byte) byte {
	if sub >= 0x51 {
		return cmpCond[sub-0x51]
	}
	return cmpCond[sub-0x46]
}
