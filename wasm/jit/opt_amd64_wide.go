//go:build (darwin || linux) && amd64

package jit

// Inline wide-integer intrinsics on amd64: pointers stay as raw indices in
// CX/DX/R11 and every access goes through [R9 + ptr + disp] addressing;
// MOV leaves the flags alone, so carry/borrow chains interleave freely
// with limb loads. AX streams limbs. Partial overlap of operand spans is
// unspecified (exact aliasing is well-defined).

// wideChk bounds-checks a pointer vreg for a w-byte access and leaves the
// zero-extended index in dest (CX, DX or R11).
func (g *optGen) wideChk(v int, w uint32, dest int) {
	a := &g.a
	p := g.read(v, rAX)
	a.MovRR32(dest, p)                           // also zero-extends the loose i32 upper half
	a.modDisp32(true, 0x8D, rAX, dest, int32(w)) // LEA rax, [dest+w]
	a.BinRR(true, 0x39, rAX, rR10)
	g.oob = append(g.oob, a.Len())
	a.Jcc(ccA, 0)
}

func (g *optGen) emitWide(ins *irInstr) {
	if k, w256 := WideOpKind(uint16(ins.sub)); k == WideMul {
		if w256 {
			g.emitMul256(ins)
		} else {
			g.emitMul128(ins)
		}
		return
	}
	a := &g.a
	kind, wide256 := WideOpKind(uint16(ins.sub))
	limbs := 2
	w := uint32(16)
	if wide256 {
		limbs, w = 4, 32
	}

	switch kind {
	case WideAdd, WideSub, WideAnd, WideOr, WideXor:
		g.wideChk(ins.a, w, rR11) // dst
		g.wideChk(ins.b, w, rCX)  // a
		g.wideChk(ins.c, w, rDX)  // b
		var first, rest byte
		switch kind {
		case WideAdd:
			first, rest = 0x03, 0x13 // ADD, ADC
		case WideSub:
			first, rest = 0x2B, 0x1B // SUB, SBB
		case WideAnd:
			first, rest = 0x23, 0x23
		case WideOr:
			first, rest = 0x0B, 0x0B
		case WideXor:
			first, rest = 0x33, 0x33
		}
		for i := 0; i < limbs; i++ {
			op := rest
			if i == 0 {
				op = first
			}
			a.memSIBd(true, 0x8B, rAX, rR9, rCX, int32(i*8))
			a.memSIBd(true, op, rAX, rR9, rDX, int32(i*8))
			a.memSIBd(true, 0x89, rAX, rR9, rR11, int32(i*8))
		}

	case WideNot:
		g.wideChk(ins.a, w, rR11)
		g.wideChk(ins.b, w, rCX)
		for i := 0; i < limbs; i++ {
			a.memSIBd(true, 0x8B, rAX, rR9, rCX, int32(i*8))
			a.bytes(0x48, 0xF7, 0xD0) // NOT RAX
			a.memSIBd(true, 0x89, rAX, rR9, rR11, int32(i*8))
		}

	case WideIsZero:
		g.wideChk(ins.a, w, rCX)
		a.memSIBd(true, 0x8B, rAX, rR9, rCX, 0)
		for i := 1; i < limbs; i++ {
			a.memSIBd(true, 0x0B, rAX, rR9, rCX, int32(i*8)) // OR
		}
		a.TestRR(true, rAX)
		a.Setcc(ccE, rAX)
		a.MovzxB(rAX)
		d, commit := g.dst(ins.dst, rAX)
		g.viaAX(d)
		commit()

	case WideCmpU, WideCmpS:
		// top-down compare with early exit: the top limb decides with
		// signed or unsigned predicates, lower limbs are always unsigned
		g.wideChk(ins.a, w, rCX)
		g.wideChk(ins.b, w, rDX)
		var topDiff, lowDiff, dones []int
		for i := limbs - 1; i >= 0; i-- {
			a.memSIBd(true, 0x8B, rAX, rR9, rCX, int32(i*8))
			a.memSIBd(true, 0x3B, rAX, rR9, rDX, int32(i*8)) // CMP rax, m
			at := a.Len()
			a.Jcc(ccNE, 0)
			if i == limbs-1 {
				topDiff = append(topDiff, at)
			} else {
				lowDiff = append(lowDiff, at)
			}
		}
		a.BinRR(true, 0x31, rAX, rAX) // equal: 0
		dones = append(dones, a.Len())
		a.Jmp(0)
		gtCC := byte(ccA)
		if kind == WideCmpS {
			gtCC = ccG
		}
		for _, at := range topDiff {
			a.PatchJcc(at)
		}
		gt1 := a.Len()
		a.Jcc(gtCC, 0)
		minus1 := a.Len()
		a.Jmp(0)
		for _, at := range lowDiff {
			a.PatchJcc(at)
		}
		gt2 := a.Len()
		a.Jcc(ccA, 0)
		a.PatchJmp(minus1)
		a.MovImm64(rAX, ^uint64(0)) // -1
		dones = append(dones, a.Len())
		a.Jmp(0)
		a.PatchJcc(gt1)
		a.PatchJcc(gt2)
		a.MovImm32(rAX, 1)
		for _, at := range dones {
			a.PatchJmp(at)
		}
		d, commit := g.dst(ins.dst, rAX)
		g.viaAX(d)
		commit()
	}
}

// mulMem emits MUL qword [R9 + idx + disp] (RDX:RAX = RAX * m64).
func (a *Asm) mulMem(idx int, disp int32) {
	a.bytes(rex(true, 4, idx, rR9), 0xF7, 0xA4, byte(idx&7)<<3|byte(rR9&7))
	a.u32(uint32(disp))
}

// imulMem emits IMUL RAX, [R9 + idx + disp] (low 64 bits, RDX untouched).
func (a *Asm) imulMem(idx int, disp int32) {
	a.bytes(rex(true, rAX, idx, rR9), 0x0F, 0xAF, 0x84, byte(idx&7)<<3|byte(rR9&7))
	a.u32(uint32(disp))
}

// adcMemImm8 emits ADC qword [SI + off], imm8 (carry propagation in place).
func (a *Asm) adcMemImm8(off int32, v byte) {
	a.bytes(rex(true, 0, 0, rSI), 0x83, 0x90|byte(rSI&7))
	a.u32(uint32(off))
	a.bytes(v)
}

// rmwMem emits op qword [SI + off], reg (01 ADD, 11 ADC, 89 MOV).
func (a *Asm) rmwMem(op byte, reg int, off int32) {
	a.modDisp32(true, op, reg, rSI, off)
}

// emitMul128: lo = a0*b0 (MUL), hi = rdx + a0*b1 + a1*b0 (IMUL keeps RDX
// free). R8/R10 borrow as accumulators and are re-derived afterwards.
func (g *optGen) emitMul128(ins *irInstr) {
	a := &g.a
	g.wideChk(ins.a, 16, rDX)               // dst: bounds only (rDX reused below)
	g.wideChk(ins.b, 16, rCX)               // a
	g.wideChk(ins.c, 16, rR11)              // b
	a.memSIBd(true, 0x8B, rAX, rR9, rCX, 0) // a0
	a.mulMem(rR11, 0)                       // rdx:rax = a0*b0
	a.BinRR(true, 0x89, rR10, rAX)          // lo -> r10
	a.BinRR(true, 0x89, rR8, rDX)           // hi -> r8
	a.memSIBd(true, 0x8B, rAX, rR9, rCX, 0) // a0
	a.imulMem(rR11, 8)                      // * b1 (low)
	a.BinRR(true, 0x01, rR8, rAX)
	a.memSIBd(true, 0x8B, rAX, rR9, rCX, 8) // a1
	a.imulMem(rR11, 0)                      // * b0 (low)
	a.BinRR(true, 0x01, rR8, rAX)
	p := g.read(ins.a, rAX) // derive the destination
	a.MovRR32(rDX, p)
	a.memSIBd(true, 0x89, rR10, rR9, rDX, 0)
	a.memSIBd(true, 0x89, rR8, rR9, rDX, 8)
	// restore the borrowed derived registers
	a.modDisp32(true, 0x8D, rR8, rSI, int32(-g.fn.nlocals*8))
	a.LdCtx(rR10, 32)
}

// emitMul256: the low 256 bits accumulate in the frame's wide-mul scratch
// slots with ADD/ADC read-modify-write chains (MOV preserves flags), so
// only AX/DX and the two source indices are live; the destination derives
// last and may alias either source.
func (g *optGen) emitMul256(ins *irInstr) {
	a := &g.a
	tmp := int32((g.linkSlot() + 1) * 8)
	g.wideChk(ins.a, 32, rDX)  // dst: bounds only
	g.wideChk(ins.b, 32, rCX)  // a
	g.wideChk(ins.c, 32, rR11) // b
	a.BinRR(true, 0x31, rAX, rAX)
	for k := 0; k < 4; k++ {
		a.rmwMem(0x89, rAX, tmp+int32(k*8)) // zero the scratch
	}
	for k := 0; k < 4; k++ {
		for i := 0; i <= k && i < 4; i++ {
			j := k - i
			a.memSIBd(true, 0x8B, rAX, rR9, rCX, int32(i*8))
			if k == 3 { // top column: only the low half matters
				a.imulMem(rR11, int32(j*8))
				a.rmwMem(0x01, rAX, tmp+int32(k*8))
				continue
			}
			a.mulMem(rR11, int32(j*8))
			a.rmwMem(0x01, rAX, tmp+int32(k*8))   // ADD lo
			a.rmwMem(0x11, rDX, tmp+int32(k*8+8)) // ADC hi
			for n := k + 2; n < 4; n++ {
				a.adcMemImm8(tmp+int32(n*8), 0) // ripple the carry
			}
		}
	}
	p := g.read(ins.a, rAX) // derive the destination and copy out
	a.MovRR32(rDX, p)
	for k := 0; k < 4; k++ {
		a.modDisp32(true, 0x8B, rAX, rSI, tmp+int32(k*8))
		a.memSIBd(true, 0x89, rAX, rR9, rDX, int32(k*8))
	}
}
