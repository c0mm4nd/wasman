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

const irNop = irRet + 1
const irBinImm = irRet + 2 // dst = a <sub> imm

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
