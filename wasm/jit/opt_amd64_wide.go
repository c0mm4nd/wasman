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
