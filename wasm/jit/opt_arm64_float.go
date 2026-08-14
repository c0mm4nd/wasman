//go:build (darwin || linux) && arm64

package jit

import "math"

// Float code generation for the optimizing tier: values live in V2-V7
// (V0/V1 stage memory-homed and cross-file operands), operations reuse the
// baseline tier's instruction selection with free register choice, and the
// slot representation stays bit-identical (f32 patterns zero-extended).

func isFloatBinOp(sub byte) bool {
	return sub >= 0x5b && sub <= 0x66 || // comparisons
		sub >= 0x92 && sub <= 0x98 || sub >= 0xa0 && sub <= 0xa6
}

func isFloatUnOp(sub byte) bool {
	return sub >= 0x8b && sub <= 0x91 || sub >= 0x99 && sub <= 0x9f ||
		sub >= 0xa8 && sub <= 0xbb && sub != 0xac && sub != 0xad ||
		sub >= 0xe0 && sub <= 0xe7 // trunc_sat
}

// float comparison conditions (unordered-false; ne true on unordered)
var fCmpCond = [6]uint32{condEQ, condNE, condMI, condGT, condLS, condGE}

func (g *optGen) emitFBin(ins *irInstr, idx int) error {
	a := &g.a
	sub := ins.sub

	// comparisons: integer result, fusable into a following branch
	if sub >= 0x5b && sub <= 0x66 {
		d := sub >= 0x61
		rel := sub - 0x5b
		if d {
			rel = sub - 0x61
		}
		vn := g.readF(ins.a, 0)
		vm := g.readF(ins.b, 1)
		a.FCmp(d, vn, vm)
		if g.branchFeeds(g.fn, idx, ins.dst) {
			g.setPend(ins.dst, 'f', fCmpCond[rel], 0, false)
			return nil
		}
		dr, commit := g.dst(ins.dst, RegT0)
		a.Cset(dr, fCmpCond[rel])
		commit()
		return nil
	}

	d := sub >= 0xa0
	rel := sub - 0x92
	if d {
		rel = sub - 0xa0
	}
	switch rel {
	case 0, 1, 2, 3: // add sub mul div
		bases := [4]uint32{0x1E202800, 0x1E203800, 0x1E200800, 0x1E201800}
		vn := g.readF(ins.a, 0)
		vm := g.readF(ins.b, 1)
		vd, commit := g.dstF(ins.dst, 0)
		a.FBin3(d, bases[rel], vd, vn, vm)
		commit()
	case 4, 5: // min/max: canonical NaN on any NaN operand
		base := uint32(0x1E205800)
		if rel == 5 {
			base = 0x1E204800
		}
		canon := uint64(0x7fc00000)
		if d {
			canon = 0x7ff8000000000000
		}
		vn := g.readF(ins.a, 0)
		vm := g.readF(ins.b, 1)
		vd, commit := g.dstF(ins.dst, 0)
		a.FCmp(d, vn, vm)
		ordered := a.Len()
		a.Bcond(condVC, 0)
		a.MovImm64(RegT0, canon)
		a.FMovToFP(true, vd, RegT0)
		done := a.Len()
		a.B(0)
		a.PatchBcond(ordered, condVC)
		a.FBin3(d, base, vd, vn, vm)
		a.PatchB(done)
		commit()
	case 6: // copysign in the integer domain
		mag, sign := uint64(0x7fffffff), uint64(0x80000000)
		if d {
			mag, sign = 0x7fffffffffffffff, 0x8000000000000000
		}
		n := g.read(ins.a, RegT0)
		m := g.read(ins.b, RegT1)
		dr, commit := g.dst(ins.dst, RegT0)
		a.MovImm64(optScratch2, mag)
		a.word(0x8A000000 | optScratch2<<16 | n<<5 | dr) // AND
		a.MovImm64(optScratch2, sign)
		a.word(0x8A000000 | optScratch2<<16 | m<<5 | RegT1) // AND -> T1
		a.word(0xAA000000 | RegT1<<16 | dr<<5 | dr)         // ORR
		commit()
	}
	return nil
}

func (g *optGen) emitFUn(ins *irInstr) error {
	a := &g.a
	sub := ins.sub
	switch {
	case sub >= 0x8b && sub <= 0x91 || sub >= 0x99 && sub <= 0x9f:
		d := sub >= 0x99
		rel := sub - 0x8b
		if d {
			rel = sub - 0x99
		}
		vn := g.readF(ins.a, 0)
		vd, commit := g.dstF(ins.dst, 0)
		// abs, neg, ceil, floor, trunc, nearest, sqrt
		bases := [7]uint32{0x1E20C000, 0x1E214000, 0x1E24C000, 0x1E254000,
			0x1E25C000, 0x1E244000, 0x1E21C000}
		a.FUn2(d, bases[rel], vd, vn)
		commit()

	case sub == 0xb6: // f32.demote_f64
		vn := g.readF(ins.a, 0)
		vd, commit := g.dstF(ins.dst, 0)
		a.word(0x1E624000 | vn<<5 | vd) // FCVT S, D
		commit()
	case sub == 0xbb: // f64.promote_f32
		vn := g.readF(ins.a, 0)
		vd, commit := g.dstF(ins.dst, 0)
		a.word(0x1E22C000 | vn<<5 | vd) // FCVT D, S
		commit()

	case sub >= 0xb2 && sub <= 0xb5 || sub >= 0xb7 && sub <= 0xba: // int -> float
		d := sub >= 0xb7
		rel := sub - 0xb2
		if d {
			rel = sub - 0xb7
		}
		signed := rel == 0 || rel == 2
		from64 := rel >= 2
		n := g.read(ins.a, RegT0)
		vd, commit := g.dstF(ins.dst, 0)
		var w uint32
		if signed {
			w = fsel(d, 0x1E620000, 0x1E220000)
		} else {
			w = fsel(d, 0x1E630000, 0x1E230000)
		}
		if from64 {
			w |= 0x80000000
		}
		a.word(w | n<<5 | vd)
		commit()

	case sub >= 0xa8 && sub <= 0xab || sub >= 0xae && sub <= 0xb1: // trapping trunc
		g.emitFTrunc(ins)

	case sub >= 0xe0 && sub <= 0xe7: // trunc_sat: FCVTZ saturates natively
		k := sub & 7
		toI64 := k >= 4
		fromF64 := k&2 != 0
		signed := k&1 == 0
		vn := g.readF(ins.a, 0)
		dr, commit := g.dst(ins.dst, RegT0)
		w := uint32(0x1E380000)
		if !signed {
			w = 0x1E390000
		}
		if fromF64 {
			w |= 0x00400000
		}
		if toI64 {
			w |= 0x80000000
		}
		a.word(w | vn<<5 | dr)
		commit()
	}
	return nil
}

// emitFTrunc mirrors the interpreter's truncCheck in the float64 domain:
// NaN traps as invalid conversion, trunc(v) outside [lo, hi) as overflow.
func (g *optGen) emitFTrunc(ins *irInstr) {
	a := &g.a
	op := ins.sub
	fromF32 := op == 0xa8 || op == 0xa9 || op == 0xae || op == 0xaf
	to64 := op >= 0xae
	signed := op == 0xa8 || op == 0xaa || op == 0xae || op == 0xb0
	b := truncBounds[op]

	vn := g.readF(ins.a, 0)
	if fromF32 {
		a.word(0x1E22C000 | vn<<5 | 0) // FCVT D0, S(vn): promote
		vn = 0
	}
	a.FCmp(true, vn, vn)
	ok := a.Len()
	a.Bcond(condVC, 0)
	a.Movz(RegStatus, StatusConvInvalid, 0)
	g.trampRet()
	a.PatchBcond(ok, condVC)
	a.FUn2(true, 0x1E25C000, 1, vn) // FRINTZ D1
	a.MovImm64(optScratch2, math.Float64bits(b[0]))
	a.FMovToFP(true, 0, optScratch2)
	a.FCmp(true, 1, 0)
	okLo := a.Len()
	a.Bcond(condGE, 0)
	a.Movz(RegStatus, StatusConvOverflow, 0)
	g.trampRet()
	a.PatchBcond(okLo, condGE)
	a.MovImm64(optScratch2, math.Float64bits(b[1]))
	a.FMovToFP(true, 0, optScratch2)
	a.FCmp(true, 1, 0)
	okHi := a.Len()
	a.Bcond(condMI, 0)
	a.Movz(RegStatus, StatusConvOverflow, 0)
	g.trampRet()
	a.PatchBcond(okHi, condMI)
	dr, commit := g.dst(ins.dst, RegT0)
	w := uint32(0x1E780000) // FCVTZS from D
	if !signed {
		w = 0x1E790000
	}
	if to64 {
		w |= 0x80000000
	}
	a.word(w | 1<<5 | dr)
	commit()
}
