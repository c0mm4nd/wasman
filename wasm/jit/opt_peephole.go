//go:build (darwin || linux) && (arm64 || amd64)

package jit

// peephole runs before register allocation, while every value still has a
// clean single definition:
//
//   1. immediate folding: an irConst consumed once by the next instruction
//      folds into it (irBinImm) when the target has an immediate form, so
//      the constant neither costs a register nor a materialization
//   2. move coalescing: a producer whose temp result is copied once by the
//      next instruction into another vreg writes that vreg directly
//
// Both patterns are adjacent-instruction rewrites — exactly what lowering
// stack code produces — and nops keep the IR indices (branch targets)
// stable.

const irNop = irWide + 1
const irBinImm = irWide + 2 // dst = a <sub> imm

// foldableImm reports whether sub has an immediate form for imm.
func foldableImm(sub byte, imm uint64) bool {
	switch sub {
	case 0x6a, 0x6b, 0x7c, 0x7d: // add/sub, both widths
		return imm < 4096
	case 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f,
		0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5a: // compares
		return imm < 4096
	}
	return false
}

func (fn *irFunc) peephole() {
	// last textual use per vreg (single pass; defs count as uses so a
	// redefinition blocks the rewrite)
	last := map[int]int{}
	for i := range fn.code {
		ins := &fn.code[i]
		ins.uses(func(v int) { last[v] = i })
		if ins.dst >= 0 {
			last[ins.dst] = i
		}
	}
	// a vreg is rewritable only if it is a pure temporary: locals and
	// boundary registers can be re-read by later merges/branches that the
	// textual scan cannot see
	temp := func(v int) bool { return v >= fn.nlocals+maxOptHeight }

	for i := 0; i+1 < len(fn.code); i++ {
		a, b := &fn.code[i], &fn.code[i+1]

		// 1. immediate folding: const -> next irBin's rhs
		if a.op == irConst && temp(a.dst) && last[a.dst] == i+1 &&
			b.op == irBin && b.b == a.dst && b.a != a.dst &&
			foldableImm(b.sub, a.imm) {
			b.op = irBinImm
			b.imm = a.imm
			b.b = -1
			a.op = irNop
			a.dst = -1
			continue
		}

		// 2. move coalescing: producer temp copied once into a vreg
		if b.op == irMov && b.a == a.dst && a.dst >= 0 && temp(a.dst) &&
			last[a.dst] == i+1 {
			switch a.op {
			case irConst, irBin, irBinImm, irUn, irLoad, irSelect, irMemSize:
				a.dst = b.dst
				b.op = irNop
				b.dst = -1
				b.a = -1
			}
		}
	}
}

// rotatableHead recognizes the canonical while-loop head at IR index h:
//
//	h:   t = <test>            (eqz or a comparison)
//	h+1: br_if_not t -> h+3    (skip the exit branch while looping)
//	h+2: br -> exit
//
// A back edge targeting h can then duplicate the test and branch straight
// to the body (h+3), turning two branches per iteration into one; the
// fallthrough goes to the exit like the original head. Returns the exit's
// IR target and ok.
func (fn *irFunc) rotatableHead(h int, lastUse func(int) int) (exit int, ok bool) {
	if h+2 >= len(fn.code) {
		return 0, false
	}
	h0, h1, h2 := &fn.code[h], &fn.code[h+1], &fn.code[h+2]
	switch h0.op {
	case irUn:
		if h0.sub != 0x45 && h0.sub != 0x50 {
			return 0, false
		}
	case irBin, irBinImm:
		if !isCmpOp(h0.sub) {
			return 0, false
		}
	default:
		return 0, false
	}
	if h1.op != irBrIfNot || h1.a != h0.dst || int(h1.imm) != h+3 ||
		lastUse(h0.dst) != h+1 || h2.op != irBr {
		return 0, false
	}
	return int(h2.imm), true
}
