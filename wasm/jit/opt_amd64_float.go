//go:build (darwin || linux) && amd64

package jit

import "math"

// Float code generation for the optimizing tier on amd64: values live in
// XMM3-XMM7 (XMM0-2 stage), the instruction selection ports the baseline
// float lowering with free register choice, and the slot representation
// stays bit-identical (f32 patterns zero-extended).

func isFloatBinOp(sub byte) bool {
	return sub >= 0x5b && sub <= 0x66 ||
		sub >= 0x92 && sub <= 0x98 || sub >= 0xa0 && sub <= 0xa6
}

func isFloatUnOp(sub byte) bool {
	return sub >= 0x8b && sub <= 0x91 || sub >= 0x99 && sub <= 0x9f ||
		sub >= 0xa8 && sub <= 0xbb && sub != 0xac && sub != 0xad ||
		sub >= 0xe0 && sub <= 0xe7
}

// ssePre selects the scalar prefix by width.
func ssePre(dbl bool) byte {
	if dbl {
		return 0xF2
	}
	return 0xF3
}

func (g *optGen) emitFBin(ins *irInstr, idx int) error {
	a := &g.a
	sub := ins.sub

	// comparisons: UCOMIS with unordered-false predicates; the compare
	// itself fuses into a following branch
	if sub >= 0x5b && sub <= 0x66 {
		d := sub >= 0x61
		rel := sub - 0x5b
		if d {
			rel = sub - 0x61
		}
		vn := g.readF(ins.a, xmm0)
		vm := g.readF(ins.b, xmm1)
		// lt/le swap operands so unordered comes out false via A/AE
		var cc byte
		switch rel {
		case 2:
			a.Ucomis(d, vm, vn)
			cc = ccA
		case 4:
			a.Ucomis(d, vm, vn)
			cc = ccAE
		case 3:
			a.Ucomis(d, vn, vm)
			cc = ccA
		case 5:
			a.Ucomis(d, vn, vm)
			cc = ccAE
		default:
			a.Ucomis(d, vn, vm)
		}
		if rel >= 2 { // strict flag predicates fuse directly
			if g.branchFeeds(g.fn, idx, ins.dst) {
				g.setPend(ins.dst, 'f', cc, 0, false)
				return nil
			}
			dr, commit := g.dst(ins.dst, rAX)
			a.Setcc(cc, rAX)
			a.MovzxB(rAX)
			g.viaAX(dr)
			commit()
			return nil
		}
		// eq/ne need the parity fixup; no branch fusion
		dr, commit := g.dst(ins.dst, rAX)
		if rel == 0 {
			a.Setcc(ccE, rAX)
			a.Setcc(ccNP, rCX)
			a.AndByteAL()
		} else {
			a.Setcc(ccNE, rAX)
			a.Setcc(ccP, rCX)
			a.OrByteAL()
		}
		a.MovzxB(rAX)
		g.viaAX(dr)
		commit()
		return nil
	}

	d := sub >= 0xa0
	rel := sub - 0x92
	if d {
		rel = sub - 0xa0
	}
	switch rel {
	case 0, 1, 2, 3: // add sub mul div (two-address: dst = dst op src)
		ops := [4]byte{0x58, 0x5C, 0x59, 0x5E}
		vn := g.readF(ins.a, xmm0)
		vm := g.readF(ins.b, xmm1)
		vd, commit := g.dstF(ins.dst, xmm0)
		switch {
		case vd == vn: // accumulate in place (the common coalesced shape)
			a.sse(ssePre(d), ops[rel], vd, vm)
		case vd != vm:
			a.sse(0xF2, 0x10, vd, vn) // MOVSD vd, vn
			a.sse(ssePre(d), ops[rel], vd, vm)
		default: // vd aliases the right operand: stage
			a.sse(0xF2, 0x10, xmm0, vn)
			a.sse(ssePre(d), ops[rel], xmm0, vm)
			a.sse(0xF2, 0x10, vd, xmm0)
		}
		commit()
	case 4, 5: // min/max with canonical NaN and signed-zero handling
		isMin := rel == 4
		canon := uint64(0x7fc00000)
		if d {
			canon = 0x7ff8000000000000
		}
		sseOp := byte(0x5D)
		if !isMin {
			sseOp = 0x5F
		}
		gpOp := byte(0x09)
		if !isMin {
			gpOp = 0x21
		}
		vn := g.readF(ins.a, xmm0)
		vm := g.readF(ins.b, xmm1)
		a.Ucomis(d, vn, vm)
		atNaN := a.Len()
		a.Jcc(ccP, 0)
		atOp := a.Len()
		a.Jcc(ccNE, 0)
		a.MovqRX(rAX, vn) // equal: merge the sign bits in the integer file
		a.MovqRX(rCX, vm)
		a.BinRR(true, gpOp, rAX, rCX)
		done1 := a.Len()
		a.Jmp(0)
		a.PatchJcc(atOp)
		if vn != xmm0 {
			a.sse(0xF2, 0x10, xmm0, vn)
		}
		a.sse(ssePre(d), sseOp, xmm0, vm)
		a.MovqRX(rAX, xmm0)
		done2 := a.Len()
		a.Jmp(0)
		a.PatchJcc(atNaN)
		a.MovImm64(rAX, canon)
		a.PatchJmp(done1)
		a.PatchJmp(done2)
		dr, commit := g.dst(ins.dst, rAX)
		g.viaAX(dr)
		commit()
	case 6: // copysign in the integer domain
		mag, sign := uint64(0x7fffffff), uint64(0x80000000)
		if d {
			mag, sign = 0x7fffffffffffffff, 0x8000000000000000
		}
		n := g.read(ins.a, rAX)
		if n != rAX {
			a.BinRR(true, 0x89, rAX, n)
		}
		m := g.read(ins.b, rCX)
		if m != rCX {
			a.BinRR(true, 0x89, rCX, m)
		}
		a.MovImm64(rDX, mag)
		a.BinRR(true, 0x21, rAX, rDX)
		a.MovImm64(rDX, sign)
		a.BinRR(true, 0x21, rCX, rDX)
		a.BinRR(true, 0x09, rAX, rCX)
		dr, commit := g.dst(ins.dst, rAX)
		g.viaAX(dr)
		commit()
	}
	return nil
}

func (g *optGen) emitFUn(ins *irInstr) error {
	a := &g.a
	sub := ins.sub
	switch {
	case sub == 0x8b || sub == 0x99 || sub == 0x8c || sub == 0x9a:
		// abs/neg in the integer domain
		d := sub == 0x99 || sub == 0x9a
		var mask uint64
		var op byte
		if sub == 0x8b || sub == 0x99 { // abs: clear sign
			op = 0x21
			mask = 0x7fffffff
			if d {
				mask = 0x7fffffffffffffff
			}
		} else { // neg: flip sign
			op = 0x31
			mask = 0x80000000
			if d {
				mask = 0x8000000000000000
			}
		}
		n := g.read(ins.a, rAX)
		if n != rAX {
			a.BinRR(true, 0x89, rAX, n)
		}
		a.MovImm64(rCX, mask)
		a.BinRR(true, op, rAX, rCX)
		dr, commit := g.dst(ins.dst, rAX)
		g.viaAX(dr)
		commit()

	case sub >= 0x8d && sub <= 0x91 || sub >= 0x9b && sub <= 0x9f:
		d := sub >= 0x9b
		rel := sub - 0x8d
		if d {
			rel = sub - 0x9b
		}
		vn := g.readF(ins.a, xmm0)
		vd, commit := g.dstF(ins.dst, xmm0)
		if rel == 4 {
			a.sse(ssePre(d), 0x51, vd, vn) // SQRT
		} else {
			modes := [4]byte{2, 1, 3, 0}
			a.Rounds(d, vd, vn, modes[rel])
		}
		g.f32fix(d, vd)
		commit()

	case sub == 0xb6: // f32.demote_f64
		vn := g.readF(ins.a, xmm0)
		vd, commit := g.dstF(ins.dst, xmm0)
		a.sse(0xF2, 0x5A, vd, vn)
		g.f32fix(false, vd)
		commit()
	case sub == 0xbb: // f64.promote_f32
		vn := g.readF(ins.a, xmm0)
		vd, commit := g.dstF(ins.dst, xmm0)
		a.sse(0xF3, 0x5A, vd, vn)
		commit()

	case sub >= 0xb2 && sub <= 0xb5 || sub >= 0xb7 && sub <= 0xba: // int -> float
		d := sub >= 0xb7
		rel := sub - 0xb2
		if d {
			rel = sub - 0xb7
		}
		signed := rel == 0 || rel == 2
		from64 := rel >= 2
		n := g.read(ins.a, rAX)
		if n != rAX {
			a.BinRR(true, 0x89, rAX, n)
		}
		vd, commit := g.dstF(ins.dst, xmm0)
		switch {
		case !from64 && signed:
			a.Movsxd(rAX, rAX)
			a.Cvtsi2f(d, true, vd, rAX)
		case !from64:
			a.MovRR32(rAX, rAX)
			a.Cvtsi2f(d, true, vd, rAX)
		case signed:
			a.Cvtsi2f(d, true, vd, rAX)
		default: // u64: halve-and-double for the high range
			a.TestRR(true, rAX)
			atBig := a.Len()
			a.Jcc(ccS, 0)
			a.Cvtsi2f(d, true, vd, rAX)
			done := a.Len()
			a.Jmp(0)
			a.PatchJcc(atBig)
			a.BinRR(true, 0x89, rCX, rAX)
			a.AndImm32(true, rCX, 1)
			a.ShiftImm(true, 5, rAX, 1)
			a.BinRR(true, 0x09, rAX, rCX)
			a.Cvtsi2f(d, true, vd, rAX)
			a.sse(ssePre(d), 0x58, vd, vd)
			a.PatchJmp(done)
		}
		g.f32fix(d, vd)
		commit()

	case sub >= 0xa8 && sub <= 0xab || sub >= 0xae && sub <= 0xb1:
		g.emitFTrunc(ins)

	case sub >= 0xe0 && sub <= 0xe7:
		g.emitFTruncSat(ins)
	}
	return nil
}

// f32fix zero-extends an f32 result: legacy scalar SSE writes only the low
// 32 bits of the destination and preserves whatever the register held.
func (g *optGen) f32fix(dbl bool, vd int) {
	if dbl {
		return
	}
	g.a.MovdRX(rCX, vd)
	g.a.MovdXR(vd, rCX)
}

// emitFTrunc mirrors truncCheck in the float64 domain with register
// operands; traps report through the ordinary status returns.
func (g *optGen) emitFTrunc(ins *irInstr) {
	a := &g.a
	op := ins.sub
	fromF32 := op == 0xa8 || op == 0xa9 || op == 0xae || op == 0xaf
	to64 := op >= 0xae
	signed := op == 0xa8 || op == 0xaa || op == 0xae || op == 0xb0
	b := truncBounds[op]

	vn := g.readF(ins.a, xmm0)
	if fromF32 {
		a.sse(0xF3, 0x5A, xmm0, vn) // promote to f64
		vn = xmm0
	}
	a.Ucomis(true, vn, vn)
	ok := a.Len()
	a.Jcc(ccNP, 0)
	a.MovImm32AX(StatusConvInvalid)
	a.Ret()
	a.PatchJcc(ok)
	a.Rounds(true, xmm1, vn, 3)
	a.MovImm64(rCX, math.Float64bits(b[0]))
	a.MovqXR(xmm2, rCX)
	a.Ucomis(true, xmm1, xmm2)
	okLo := a.Len()
	a.Jcc(ccAE, 0)
	a.MovImm32AX(StatusConvOverflow)
	a.Ret()
	a.PatchJcc(okLo)
	a.MovImm64(rCX, math.Float64bits(b[1]))
	a.MovqXR(xmm2, rCX)
	a.Ucomis(true, xmm1, xmm2)
	okHi := a.Len()
	a.Jcc(ccB, 0)
	a.MovImm32AX(StatusConvOverflow)
	a.Ret()
	a.PatchJcc(okHi)
	g.cvtTrunc(signed, to64)
	dr, commit := g.dst(ins.dst, rAX)
	g.viaAX(dr)
	commit()
}

// cvtTrunc converts the truncated double in xmm1 into RAX per target.
func (g *optGen) cvtTrunc(signed, to64 bool) {
	a := &g.a
	switch {
	case signed && to64:
		a.Cvttf2i(true, true, rAX, xmm1)
	case signed:
		a.Cvttf2i(true, false, rAX, xmm1)
	case !to64:
		a.Cvttf2i(true, true, rAX, xmm1)
		a.MovRR32(rAX, rAX)
	default:
		a.MovImm64(rCX, math.Float64bits(9223372036854775808.0))
		a.MovqXR(xmm2, rCX)
		a.Ucomis(true, xmm1, xmm2)
		atSmall := a.Len()
		a.Jcc(ccB, 0)
		a.sse(0xF2, 0x5C, xmm1, xmm2)
		a.Cvttf2i(true, true, rAX, xmm1)
		a.MovImm64(rCX, 0x8000000000000000)
		a.BinRR(true, 0x01, rAX, rCX)
		done := a.Len()
		a.Jmp(0)
		a.PatchJcc(atSmall)
		a.Cvttf2i(true, true, rAX, xmm1)
		a.PatchJmp(done)
	}
}

// emitFTruncSat clamps instead of trapping (NaN -> 0).
func (g *optGen) emitFTruncSat(ins *irInstr) {
	a := &g.a
	k := ins.sub & 7
	r := satRange[k]
	fromF32 := k&2 == 0
	to64 := k >= 4
	signed := k&1 == 0

	vn := g.readF(ins.a, xmm0)
	if fromF32 {
		a.sse(0xF3, 0x5A, xmm0, vn)
		vn = xmm0
	}
	var dones []int
	a.Ucomis(true, vn, vn)
	ok := a.Len()
	a.Jcc(ccNP, 0)
	a.BinRR(true, 0x31, rAX, rAX)
	dones = append(dones, a.Len())
	a.Jmp(0)
	a.PatchJcc(ok)
	a.Rounds(true, xmm1, vn, 3)
	a.MovImm64(rCX, math.Float64bits(r.lo))
	a.MovqXR(xmm2, rCX)
	a.Ucomis(true, xmm1, xmm2)
	okLo := a.Len()
	a.Jcc(ccAE, 0)
	a.MovImm64(rAX, r.min)
	dones = append(dones, a.Len())
	a.Jmp(0)
	a.PatchJcc(okLo)
	a.MovImm64(rCX, math.Float64bits(r.hi))
	a.MovqXR(xmm2, rCX)
	a.Ucomis(true, xmm1, xmm2)
	okHi := a.Len()
	a.Jcc(ccB, 0)
	a.MovImm64(rAX, r.max)
	dones = append(dones, a.Len())
	a.Jmp(0)
	a.PatchJcc(okHi)
	g.cvtTrunc(signed, to64)
	for _, at := range dones {
		a.PatchJmp(at)
	}
	dr, commit := g.dst(ins.dst, rAX)
	g.viaAX(dr)
	commit()
}
