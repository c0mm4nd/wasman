//go:build (darwin || linux) && arm64

package jit

// Inline wide-integer intrinsics: calls to the built-in u128/u256 host
// operations compile to native carry chains over linear memory. Scratches:
// R16/R17/R2 hold effective addresses (R2 only matters at exits, which
// rewrite it), R6/R7 stream limbs. Partial overlap of the operand spans
// is unspecified (exact aliasing — dst == a or dst == b — is well-defined,
// as limb i is stored before limb i+1 loads).

const wideScratch3 = 2 // RegSp's register: free between host exits

// wideEff bounds-checks the pointer vreg v for a w-byte access and leaves
// the effective address in dest.
func (g *optGen) wideEff(v int, w uint32, dest uint32) {
	a := &g.a
	p := g.read(v, RegT0)
	a.Uxtw(RegT0, p) // i32 pointers carry a loose upper half
	a.AddImm(RegT1, RegT0, w)
	a.CmpRegX(RegT1, RegMemLen)
	g.oob = append(g.oob, a.Len())
	a.Bcond(condHI, 0)
	a.AddRegX(dest, RegMem, RegT0)
}

func (g *optGen) emitWide(ins *irInstr) {
	a := &g.a
	id := int(ins.sub)
	kind := (id - 1) & 0xf
	limbs := 2
	w := uint32(16)
	if id > 16 {
		limbs, w = 4, 32
	}

	switch kind {
	case WideAdd, WideSub, WideAnd, WideOr, WideXor:
		g.wideEff(ins.a, w, optScratch2)  // dst
		g.wideEff(ins.b, w, 17)           // a
		g.wideEff(ins.c, w, wideScratch3) // b
		for i := 0; i < limbs; i++ {
			a.LdrImm(RegT0, 17, uint32(i*8))
			a.LdrImm(RegT1, wideScratch3, uint32(i*8))
			switch kind {
			case WideAdd:
				if i == 0 {
					a.word(0xAB000000 | RegT1<<16 | RegT0<<5 | RegT0) // ADDS
				} else {
					a.word(0xBA000000 | RegT1<<16 | RegT0<<5 | RegT0) // ADCS
				}
			case WideSub:
				if i == 0 {
					a.word(0xEB000000 | RegT1<<16 | RegT0<<5 | RegT0) // SUBS
				} else {
					a.word(0xFA000000 | RegT1<<16 | RegT0<<5 | RegT0) // SBCS
				}
			case WideAnd:
				a.word(0x8A000000 | RegT1<<16 | RegT0<<5 | RegT0)
			case WideOr:
				a.word(0xAA000000 | RegT1<<16 | RegT0<<5 | RegT0)
			case WideXor:
				a.word(0xCA000000 | RegT1<<16 | RegT0<<5 | RegT0)
			}
			a.StrImm(RegT0, optScratch2, uint32(i*8))
		}

	case WideNot:
		g.wideEff(ins.a, w, optScratch2)
		g.wideEff(ins.b, w, 17)
		for i := 0; i < limbs; i++ {
			a.LdrImm(RegT0, 17, uint32(i*8))
			a.word(0xAA2003E0 | RegT0<<16 | RegT0) // MVN
			a.StrImm(RegT0, optScratch2, uint32(i*8))
		}

	case WideIsZero:
		g.wideEff(ins.a, w, 17)
		a.LdrImm(RegT0, 17, 0)
		for i := 1; i < limbs; i++ {
			a.LdrImm(RegT1, 17, uint32(i*8))
			a.word(0xAA000000 | RegT1<<16 | RegT0<<5 | RegT0) // ORR
		}
		a.CmpImmX(RegT0, 0)
		d, commit := g.dst(ins.dst, RegT0)
		a.Cset(d, condEQ)
		commit()

	case WideCmpU, WideCmpS:
		g.wideEff(ins.a, w, 17)
		g.wideEff(ins.b, w, wideScratch3)
		if kind == WideCmpS {
			a.MovImm64(optScratch2, 0x8000000000000000)
		}
		// carry after a full-width SUBS/SBCS chain means "left >= right";
		// two directed chains give (a>=b) - (b>=a) = -1, 0 or 1
		chain := func(l, r uint32, dest uint32) {
			for i := 0; i < limbs; i++ {
				a.LdrImm(RegT0, l, uint32(i*8))
				a.LdrImm(RegT1, r, uint32(i*8))
				if kind == WideCmpS && i == limbs-1 {
					// flipping the sign bits makes unsigned order signed
					a.word(0xCA000000 | optScratch2<<16 | RegT0<<5 | RegT0)
					a.word(0xCA000000 | optScratch2<<16 | RegT1<<5 | RegT1)
				}
				if i == 0 {
					a.CmpRegX(RegT0, RegT1) // SUBS discarding the result
				} else {
					a.word(0xFA00001F | RegT1<<16 | RegT0<<5) // SBCS xzr
				}
			}
			a.Cset(dest, condHS)
		}
		// the second chain reuses the limb scratches, so the first result
		// must survive outside them: a pool destination holds it directly,
		// a memory-homed one parks it in its own home slot
		if l, ok := g.vlocOf(ins.dst); ok && l.reg >= 0 && !l.freg {
			d := uint32(8 + l.reg)
			chain(17, wideScratch3, d)
			chain(wideScratch3, 17, RegT1)
			a.word(0xCB000000 | RegT1<<16 | d<<5 | d) // SUB d, aGEb, bGEa
		} else {
			base, off := g.homeAddr(ins.dst)
			chain(17, wideScratch3, RegT0)
			a.StrImm(RegT0, base, off)
			chain(wideScratch3, 17, RegT1)
			a.LdrImm(RegT0, base, off)
			a.word(0xCB000000 | RegT1<<16 | RegT0<<5 | RegT0)
			a.StrImm(RegT0, base, off)
		}
	}
}
