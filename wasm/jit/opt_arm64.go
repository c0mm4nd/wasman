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

// optFloatSupported gates frontend acceptance of float opcodes.
const optFloatSupported = true

// optNumFRegs sizes the float pool: V2-V7 (V0/V1 stage, popcnt uses V0).
const optNumFRegs = 6

const optScratch2 = 16 // third scratch, outside the pool and the ABI set

// CompileOpt lowers, allocates and generates native code for fd.
func CompileOpt(fd *FuncDesc) (*Compiled, error) {
	fe := &irFrontend{fd: fd}
	if err := fe.lower(); err != nil {
		return nil, err
	}
	fn := &fe.fn
	fn.peephole()
	al := fn.allocate2(optNumRegs, optNumFRegs)
	if os.Getenv("WASMAN_WIDE_DEBUG") == "1" {
		for _, i2 := range fn.code {
			if i2.op == irWide {
				println("IRWIDE id", int(i2.sub))
			}
		}
	}
	if os.Getenv("WASMAN_OPT_DEBUG") == "1" {
		for i, ins := range fn.code {
			fmt.Printf("IR%02d op=%d sub=%#x dst=%d a=%d b=%d imm=%d\n", i, ins.op, ins.sub, ins.dst, ins.a, ins.b, ins.imm)
		}
		for v, ci := range al.compact {
			fmt.Printf("  v%d -> reg=%d spill=%d\n", v, al.loc[ci].reg, al.loc[ci].spill)
		}
	}
	if fn.nlocals*8 >= 4096 {
		return nil, ErrUnsupported // locals offset must fit an imm12
	}
	for _, ins := range fn.code {
		if ins.op == irCallNative {
			// call-site offsets (callee locals at the frame end) must stay
			// within the scaled-imm12 addressing range
			if (fn.maxH+al.spillSlots+2+520)*8 >= 32760 {
				return nil, ErrUnsupported
			}
			break
		}
	}
	g := &optGen{fn: fn, fd: fd, al: al}
	for _, i2 := range fn.code {
		if i2.op == irWide {
			if k, w256 := WideOpKind(uint16(i2.sub)); k == WideMul && w256 {
				g.frameExtra = 4
				break
			}
		}
	}
	if err := g.gen(); err != nil {
		return nil, err
	}
	code, err := AllocExec(g.a.Bytes())
	if err != nil {
		return nil, err
	}
	return &Compiled{Code: code, MaxHeight: fn.maxH + al.spillSlots,
		CallSites: fn.sites, NativeABI: true,
		FrameSlots: fn.nlocals + fn.maxH + al.spillSlots + 1 + g.frameExtra,
		LocalSlots: fn.nlocals}, nil
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
	fd      *FuncDesc
	al      *allocation
	a       Asm
	irOff   []int
	lasts   map[int]int
	patches []optPatch
	oob     []int
	// frameExtra reserves scratch slots above the link slot (wide-integer
	// multiplication temporaries)
	frameExtra int
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

// vlocOf fetches the full location record.
func (g *optGen) vlocOf(v int) (vloc, bool) {
	ci, ok := g.al.compact[v]
	if !ok {
		return vloc{reg: -1}, false
	}
	return g.al.loc[ci], true
}

// read makes v available in an integer register, staging from memory or
// bridging from the float file into scratch.
func (g *optGen) read(v int, scratch uint32) uint32 {
	if l, ok := g.vlocOf(v); ok && l.reg >= 0 {
		if l.freg {
			g.a.FMovFromFP(true, scratch, uint32(2+l.reg))
			return scratch
		}
		return uint32(8 + l.reg)
	}
	base, off := g.homeAddr(v)
	g.a.LdrImm(scratch, base, off)
	return scratch
}

// dst returns the integer register to compute v into plus a commit step for
// memory homes and float-file destinations.
func (g *optGen) dst(v int, scratch uint32) (uint32, func()) {
	if l, ok := g.vlocOf(v); ok && l.reg >= 0 {
		if l.freg {
			fr := uint32(2 + l.reg)
			return scratch, func() { g.a.FMovToFP(true, fr, scratch) }
		}
		return uint32(8 + l.reg), func() {}
	}
	base, off := g.homeAddr(v)
	return scratch, func() { g.a.StrImm(scratch, base, off) }
}

// readF makes v available in a float register (V0/V1 stage).
func (g *optGen) readF(v int, vscratch uint32) uint32 {
	if l, ok := g.vlocOf(v); ok && l.reg >= 0 {
		if l.freg {
			return uint32(2 + l.reg)
		}
		g.a.FMovToFP(true, vscratch, uint32(8+l.reg))
		return vscratch
	}
	base, off := g.homeAddr(v)
	g.a.LdrF(vscratch, base, off)
	return vscratch
}

// dstF returns the float register to compute v into plus its commit.
func (g *optGen) dstF(v int, vscratch uint32) (uint32, func()) {
	if l, ok := g.vlocOf(v); ok && l.reg >= 0 {
		if l.freg {
			return uint32(2 + l.reg), func() {}
		}
		gr := uint32(8 + l.reg)
		return vscratch, func() { g.a.FMovFromFP(true, gr, vscratch) }
	}
	base, off := g.homeAddr(v)
	return vscratch, func() { g.a.StrF(vscratch, base, off) }
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

	g.framePrologue(true)
	// register-allocated locals load once here
	for j := 0; j < fn.nlocals; j++ {
		if l, ok := g.vlocOf(j); ok && l.reg >= 0 {
			if l.freg {
				a.LdrF(uint32(2+l.reg), RegLocals, uint32(j*8))
			} else {
				a.LdrImm(uint32(8+l.reg), RegLocals, uint32(j*8))
			}
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
			if isFloatBinOp(ins.sub) {
				if err := g.emitFBin(ins, idx); err != nil {
					return err
				}
				break
			}
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
			if isFloatUnOp(ins.sub) {
				if err := g.emitFUn(ins); err != nil {
					return err
				}
				break
			}
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
			var fastEnd []int
			if site.Kind == SiteCallIndirect && site.TableIdx == 0 &&
				g.fd.NativeFuncs != nil && g.fd.TypeSigIDs != nil {
				fastEnd = g.emitIndirectFast(site)
			}
			a.StrImm(RegStack, RegCtx, 8) // ctx.Sp = my frame pointer
			a.MovImm64(RegT0, uint64(g.fd.SelfIdx)<<32|ins.imm)
			a.StrImm(RegT0, RegCtx, 40)
			st := uint32(StatusCall)
			if site.Kind == SiteCallIndirect {
				st = StatusCallIndirect
			}
			a.Movz(RegStatus, st, 0)
			g.trampRet()
			for _, at := range fastEnd {
				a.PatchB(at)
			}
			site.Cont = a.Len()
			// resumed via the shim: R1 already holds this frame's base
			g.framePrologue(false)
		case irCallNative:
			g.emitNativeCall(ins)
		case irWide:
			g.emitWide(ins)
		case irGlobalGet: // cells are *uint64: double indirection
			a.LdrImm(optScratch2, RegCtx, 48)
			a.LdrImm(optScratch2, optScratch2, uint32(ins.imm*8))
			d, commit := g.dst(ins.dst, RegT0)
			a.LdrImm(d, optScratch2, 0)
			commit()
		case irGlobalSet:
			v := g.read(ins.a, RegT0)
			a.LdrImm(optScratch2, RegCtx, 48)
			a.LdrImm(optScratch2, optScratch2, uint32(ins.imm*8))
			a.StrImm(v, optScratch2, 0)
		case irTrap:
			a.Movz(RegStatus, uint32(ins.sub), 0)
			g.trampRet()
		case irRet:
			g.frameEpilogue()
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
		a.Movz(RegStatus, StatusMemOOB, 0)
		g.trampRet()
	}
	return nil
}

// trampRet leaves generated code for the trampoline continuation: the link
// register may hold a native return address, so the exit loads the address
// the entry shim recorded.
func (g *optGen) trampRet() {
	g.a.LdrImm(30, RegCtx, 64) // Ctx.TrampRet
	g.a.Ret()
}

// linkSlot is the frame slot holding the saved return address.
func (g *optGen) linkSlot() int { return g.fn.maxH + g.al.spillSlots }

// framePrologue establishes the in-stack frame: R1 arrives as this frame's
// stack base (from a native caller or the entry shim). The full form runs
// the capacity/depth checks, saves the return address and zeroes declared
// locals; continuations only rebuild the derived registers.
func (g *optGen) framePrologue(full bool) {
	a := &g.a
	need := g.fn.nlocals
	if full {
		end := (g.linkSlot() + 1 + g.frameExtra) * 8
		if end < 4096 {
			a.AddImm(RegT0, RegStack, uint32(end))
		} else {
			a.MovImm64(RegT0, uint64(end))
			a.AddRegX(RegT0, RegStack, RegT0)
		}
		a.LdrImm(RegT1, RegCtx, 56) // Ctx.StackLimit
		a.CmpRegX(RegT0, RegT1)
		ok := a.Len()
		a.Bcond(condLS, 0)
		a.Movz(RegStatus, StatusExhausted, 0)
		g.trampRet()
		a.PatchBcond(ok, condLS)
		if lim := g.fd.DepthLimit; lim != 0 {
			a.LdrImm(RegT0, RegCtx, 80) // Ctx.Depth
			a.AddImm(RegT0, RegT0, 1)
			a.StrImm(RegT0, RegCtx, 80)
			a.MovImm64(RegT1, lim)
			a.CmpRegX(RegT0, RegT1)
			ok2 := a.Len()
			a.Bcond(condLO, 0)
			a.Movz(RegStatus, StatusExhausted, 0)
			g.trampRet()
			a.PatchBcond(ok2, condLO)
		}
		a.StrImm(30, RegStack, uint32(g.linkSlot()*8)) // save LR
	}
	if need > 0 {
		a.SubImm(RegLocals, RegStack, uint32(need*8))
	} else {
		a.word(0xAA0003E0 | RegStack<<16 | RegLocals) // MOV R3, R1
	}
	if full {
		for i := g.fd.NumParams; i < need; i++ {
			a.StrImm(31, RegLocals, uint32(i*8)) // STR XZR: zero declared
		}
	}
	a.LdrImm(RegMem, RegCtx, 24)
	a.LdrImm(RegMemLen, RegCtx, 32)
}

// frameEpilogue returns to whoever entered: results already sit at slots
// [0, nrets); the status/frame-pointer stores only matter when the caller
// is the trampoline, and are harmless on native returns.
func (g *optGen) frameEpilogue() {
	a := &g.a
	if g.fd.DepthLimit != 0 {
		a.LdrImm(RegT0, RegCtx, 80)
		a.SubImm(RegT0, RegT0, 1)
		a.StrImm(RegT0, RegCtx, 80)
	}
	a.StrImm(RegStack, RegCtx, 8) // ctx.Sp = frame pointer
	a.Movz(RegStatus, StatusOK, 0)
	// indirect-call return ABI: R7 reports this frame's locals extent so a
	// dynamic caller can rebase without knowing the callee statically
	a.Movz(RegT1, uint32(g.fn.nlocals*8), 0)
	a.LdrImm(30, RegStack, uint32(g.linkSlot()*8))
	a.Ret()
}

// emitIndirectFast tries a call_indirect natively: bounds, null and
// signature checks against the per-instance mirror, then a direct jump to
// a native-ABI target. Any failure falls through to the host-exit slow
// path, which reproduces the exact trap semantics and handles non-native
// callees. Returns patches that skip the exit code on the fast path.
func (g *optGen) emitIndirectFast(site *CallSite) []int {
	a := &g.a
	sig := g.fn.typeSigs[site.TypeIdx]
	expect := g.fd.TypeSigIDs[site.TypeIdx]
	if expect >= 4096 {
		return nil
	}
	sp := site.SpBefore
	n := sig.In
	frameEnd := (g.linkSlot() + 1 + g.frameExtra) * 8
	var exits []int
	// mirror present?
	a.LdrImm(optScratch2, RegCtx, 88)
	exits = append(exits, a.Len())
	a.Cbz(optScratch2, 0)
	// bounds: idx < len
	a.LdrImm(RegT0, RegStack, uint32((sp-1)*8))
	a.Uxtw(RegT0, RegT0)
	a.LdrImm(RegT1, optScratch2, 0)
	a.CmpRegX(RegT0, RegT1)
	exits = append(exits, a.Len())
	a.Bcond(condHS, 0)
	// entry & meta
	a.AddRegLsl(optScratch2, optScratch2, RegT0, 4)
	a.LdrImm(RegT1, optScratch2, 16) // native entry (0: null or not native)
	exits = append(exits, a.Len())
	a.Cbz(RegT1, 0)
	a.LdrImm(optScratch2, optScratch2, 8) // sigID<<32 | needBytes
	a.LsrImmX(RegT0, optScratch2, 32)
	a.CmpImmX(RegT0, expect)
	exits = append(exits, a.Len())
	a.Bcond(condNE, 0)
	// arguments into the callee locals area at this frame's end
	for i := 0; i < n; i++ {
		a.LdrImm(RegT0, RegStack, uint32((sp-1-n+i)*8))
		a.StrImm(RegT0, RegStack, uint32(frameEnd+i*8))
	}
	a.Uxtw(optScratch2, optScratch2) // needBytes
	a.AddImm(RegStack, RegStack, uint32(frameEnd))
	a.AddRegX(RegStack, RegStack, optScratch2)
	a.word(0xD63F0000 | RegT1<<5) // BLR
	// return ABI: R7 = callee needBytes
	a.word(0xCB000000 | RegT1<<16 | RegStack<<5 | RegStack) // SUB R1, R1, R7
	a.SubImm(RegStack, RegStack, uint32(frameEnd))
	g.framePrologue(false)
	// results back to the caller stack top
	for i := 0; i < sig.Out; i++ {
		a.AddRegX(optScratch2, RegStack, RegT1)
		a.LdrImm(RegT0, optScratch2, uint32(frameEnd+i*8))
		a.StrImm(RegT0, RegStack, uint32((sp-1-n+i)*8))
	}
	done := a.Len()
	a.B(0) // skip the slow path
	// failed checks land here, on the host exit; CBZ and B.cond both keep
	// their 19-bit offset at bit 5, so one fixup form serves either
	for _, at := range exits {
		w := binWordAt(&g.a, at)
		g.a.setWord(at, w|(uint32((a.Len()-at)/4)&0x7ffff)<<5)
	}
	return []int{done}
}

func binWordAt(a *Asm, at int) uint32 {
	return uint32(a.buf[at]) | uint32(a.buf[at+1])<<8 | uint32(a.buf[at+2])<<16 | uint32(a.buf[at+3])<<24
}

// emitNativeCall performs a direct call: the callee frame starts at this
// frame's end (everything below — spills, linkage — stays live across the
// call), arguments copy into the callee's locals and results copy back to
// the caller's stack top. Live-across values are memory-homed by
// allocation, so only the derived registers need rebuilding afterwards.
func (g *optGen) emitNativeCall(ins *irInstr) {
	a := &g.a
	idx := int(ins.imm >> 32)
	sp := int(uint32(ins.imm))
	sig := g.fn.sigs[idx]
	frameEnd := (g.linkSlot() + 1 + g.frameExtra) * 8 // callee locals start here
	// arguments: caller stack top -> callee locals
	for i := 0; i < sig.In; i++ {
		a.LdrImm(RegT0, RegStack, uint32((sp-sig.In+i)*8))
		a.StrImm(RegT0, RegStack, uint32(frameEnd+i*8))
	}
	off := frameEnd + sig.Locals*8
	g.addSubR1(off, true)
	a.LdrImm(RegT0, RegCtx, 72) // Ctx.Funcs
	a.LdrImm(RegT0, RegT0, uint32(idx*8))
	a.word(0xD63F0000 | RegT0<<5) // BLR
	g.addSubR1(off, false)
	g.framePrologue(false) // rebuild R3-R5 (memory may have grown)
	// results: callee stack base -> caller stack top
	for i := 0; i < sig.Out; i++ {
		a.LdrImm(RegT0, RegStack, uint32(off+i*8))
		a.StrImm(RegT0, RegStack, uint32((sp-sig.In+i)*8))
	}
}

func (g *optGen) addSubR1(off int, add bool) {
	a := &g.a
	if off == 0 {
		return
	}
	if off < 4096 {
		if add {
			a.AddImm(RegStack, RegStack, uint32(off))
		} else {
			a.SubImm(RegStack, RegStack, uint32(off))
		}
		return
	}
	a.MovImm64(RegT0, uint64(off))
	if add {
		a.AddRegX(RegStack, RegStack, RegT0)
	} else {
		a.word(0xCB000000 | RegT0<<16 | RegStack<<5 | RegStack) // SUB
	}
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
	if g.lasts == nil {
		g.lasts = g.fn.lastUses()
	}
	if at, ok := g.lasts[v]; ok {
		return at
	}
	return -1
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
