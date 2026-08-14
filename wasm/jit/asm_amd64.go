//go:build (darwin || linux) && amd64

package jit

import "encoding/binary"

// Asm is a minimal amd64 instruction emitter for the template compiler.
type Asm struct{ buf []byte }

func (a *Asm) bytes(bs ...byte) { a.buf = append(a.buf, bs...) }

func (a *Asm) u32(v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	a.buf = append(a.buf, b[:]...)
}

func (a *Asm) u64(v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	a.buf = append(a.buf, b[:]...)
}

// Bytes returns the emitted code.
func (a *Asm) Bytes() []byte { return a.buf }

// Len returns the current length in bytes.
func (a *Asm) Len() int { return len(a.buf) }

// register numbers (low 3 bits; bit 3 signalled via REX)
const (
	rAX  = 0
	rCX  = 1
	rDX  = 2
	rSP  = 4
	rSI  = 6
	rDI  = 7
	rR8  = 8
	rR9  = 9
	rR10 = 10
	rR11 = 11
)

// rex builds a REX prefix. w: 64-bit operand, r/x/b: extension bits for the
// modrm reg field, SIB index and modrm rm/base.
func rex(w bool, r, x, b int) byte {
	v := byte(0x40)
	if w {
		v |= 8
	}
	if r >= 8 {
		v |= 4
	}
	if x >= 8 {
		v |= 2
	}
	if b >= 8 {
		v |= 1
	}
	return v
}

// modRegReg emits op with modrm mode 11 (register-register).
func (a *Asm) modRegReg(w bool, op byte, reg, rm int) {
	a.bytes(rex(w, reg, 0, rm), op, 0xC0|byte(reg&7)<<3|byte(rm&7))
}

// modDisp32 emits op with modrm mode 10 (register-[base+disp32]).
// base must not be RSP/R12 (needs SIB) or RBP/R13 (fine with mode 10).
func (a *Asm) modDisp32(w bool, op byte, reg, base int, disp int32) {
	a.bytes(rex(w, reg, 0, base), op, 0x80|byte(reg&7)<<3|byte(base&7))
	a.u32(uint32(disp))
}

// LdSlot emits MOV reg, [SI + slot*8].
func (a *Asm) LdSlot(reg, slot int) { a.modDisp32(true, 0x8B, reg, rSI, int32(slot*8)) }

// StSlot emits MOV [SI + slot*8], reg.
func (a *Asm) StSlot(reg, slot int) { a.modDisp32(true, 0x89, reg, rSI, int32(slot*8)) }

// LdLocal / StLocal access [R8 + idx*8].
func (a *Asm) LdLocal(reg, idx int) { a.modDisp32(true, 0x8B, reg, rR8, int32(idx*8)) }
func (a *Asm) StLocal(reg, idx int) { a.modDisp32(true, 0x89, reg, rR8, int32(idx*8)) }

// LdCtx / StCtx access [DI + off] (the Ctx fields).
func (a *Asm) LdCtx(reg int, off int32) { a.modDisp32(true, 0x8B, reg, rDI, off) }
func (a *Asm) StCtx(reg int, off int32) { a.modDisp32(true, 0x89, reg, rDI, off) }

// MovImm64 emits MOV reg, imm64.
func (a *Asm) MovImm64(reg int, v uint64) {
	a.bytes(rex(true, 0, 0, reg), 0xB8|byte(reg&7))
	a.u64(v)
}

// BinRR emits a register-register ALU op (01 add, 29 sub, 21 and, 09 or,
// 31 xor, 39 cmp): op reg2 into reg1 (dst, src ordering of the /r form).
func (a *Asm) BinRR(w bool, op byte, dst, src int) { a.modRegReg(w, op, src, dst) }

// Imul emits IMUL dst, src (0F AF /r).
func (a *Asm) Imul(w bool, dst, src int) {
	a.bytes(rex(w, dst, 0, src), 0x0F, 0xAF, 0xC0|byte(dst&7)<<3|byte(src&7))
}

// MovRR32 emits MOV dst32, src32, zeroing the upper half.
func (a *Asm) MovRR32(dst, src int) { a.modRegReg(false, 0x8B, dst, src) }

// Movsxd emits MOVSXD dst64, src32 (sign-extend).
func (a *Asm) Movsxd(dst, src int) { a.modRegReg(true, 0x63, dst, src) }

// ShiftCL emits a shift by CL: sub 4 shl, 5 shr, 7 sar.
func (a *Asm) ShiftCL(w bool, sub byte, reg int) {
	a.bytes(rex(w, 0, 0, reg), 0xD3, 0xC0|sub<<3|byte(reg&7))
}

// TestRR emits TEST reg, reg.
func (a *Asm) TestRR(w bool, reg int) { a.modRegReg(w, 0x85, reg, reg) }

// Setcc emits SETcc AL..; cc is the 0F 9x low nibble. Only AX/CX/DX targets.
func (a *Asm) Setcc(cc byte, reg int) {
	a.bytes(0x0F, 0x90|cc, 0xC0|byte(reg&7))
}

// MovzxB emits MOVZX reg32, reg8 (AX/CX/DX only).
func (a *Asm) MovzxB(reg int) {
	a.bytes(0x0F, 0xB6, 0xC0|byte(reg&7)<<3|byte(reg&7))
}

// Jmp emits JMP rel32 to an absolute code offset (or a placeholder).
func (a *Asm) Jmp(target int) {
	a.bytes(0xE9)
	a.u32(uint32(int32(target - (a.Len() + 4))))
}

// Jcc emits Jcc rel32 (cc as in Setcc) to an absolute offset/placeholder.
func (a *Asm) Jcc(cc byte, target int) {
	a.bytes(0x0F, 0x80|cc)
	a.u32(uint32(int32(target - (a.Len() + 4))))
}

// PatchJmp rewrites the rel32 of the JMP emitted at byte offset `at`.
func (a *Asm) PatchJmp(at int) {
	binary.LittleEndian.PutUint32(a.buf[at+1:], uint32(int32(a.Len()-(at+5))))
}

// PatchJcc rewrites the rel32 of the Jcc emitted at byte offset `at`.
func (a *Asm) PatchJcc(at int) {
	binary.LittleEndian.PutUint32(a.buf[at+2:], uint32(int32(a.Len()-(at+6))))
}

// Ret emits RET.
func (a *Asm) Ret() { a.bytes(0xC3) }

// MovImm32AX emits MOV EAX, imm32 (the status result).
func (a *Asm) MovImm32AX(v uint32) { a.bytes(0xB8); a.u32(v) }

// memSIB emits opcode bytes with modrm mode 00 and a SIB of [R9 + idx].
func (a *Asm) memSIB(pre []byte, wide bool, op []byte, reg, idx int) {
	r := rex(wide, reg, idx, rR9)
	a.buf = append(a.buf, pre...)
	if r != 0x40 || wide || reg >= 8 || idx >= 8 {
		a.bytes(r)
	}
	a.buf = append(a.buf, op...)
	a.bytes(byte(reg&7)<<3|0x04, 0x00|byte(idx&7)<<3|byte(rR9&7)) // mod00 + SIB scale1
}

// CmoveRR emits CMOVE dst, src (move when ZF set).
func (a *Asm) CmoveRR(dst, src int) {
	a.bytes(rex(true, dst, 0, src), 0x0F, 0x44, 0xC0|byte(dst&7)<<3|byte(src&7))
}

// AddImm32 emits ADD reg, imm32 (sign-extended to 64 bits).
func (a *Asm) AddImm32(reg int, v uint32) {
	a.bytes(rex(true, 0, 0, reg), 0x81, 0xC0|byte(reg&7))
	a.u32(v)
}

// ShiftImm emits a shift by an immediate count (sub as in ShiftCL).
func (a *Asm) ShiftImm(w bool, sub byte, reg int, n byte) {
	a.bytes(rex(w, 0, 0, reg), 0xC1, 0xC0|sub<<3|byte(reg&7), n)
}

// Cdq / Cqo sign-extend AX into DX before IDIV.
func (a *Asm) Cdq() { a.bytes(0x99) }
func (a *Asm) Cqo() { a.bytes(0x48, 0x99) }

// DivCX emits DIV/IDIV by CX (sub 6 div, 7 idiv), operating on DX:AX.
func (a *Asm) DivCX(w bool, sub byte) {
	a.bytes(rex(w, 0, 0, rCX), 0xF7, 0xC0|sub<<3|byte(rCX&7))
}

// XorDX zeroes (E)DX.
func (a *Asm) XorDX(w bool) { a.modRegReg(w, 0x31, rDX, rDX) }

// Bsr / Bsf emit bit scans (undefined dst when src is zero; guard first).
func (a *Asm) Bsr(w bool, dst, src int) {
	a.bytes(rex(w, dst, 0, src), 0x0F, 0xBD, 0xC0|byte(dst&7)<<3|byte(src&7))
}
func (a *Asm) Bsf(w bool, dst, src int) {
	a.bytes(rex(w, dst, 0, src), 0x0F, 0xBC, 0xC0|byte(dst&7)<<3|byte(src&7))
}

// Popcnt emits POPCNT dst, src (SSE4.2 baseline, like other Go JITs).
func (a *Asm) Popcnt(w bool, dst, src int) {
	a.bytes(0xF3, rex(w, dst, 0, src), 0x0F, 0xB8, 0xC0|byte(dst&7)<<3|byte(src&7))
}

// RotCL emits ROL/ROR by CL (sub 0 rol, 1 ror).
func (a *Asm) RotCL(w bool, sub byte, reg int) {
	a.bytes(rex(w, 0, 0, reg), 0xD3, 0xC0|sub<<3|byte(reg&7))
}

// MovsxB / MovsxW sign-extend the low 8/16 bits of a register in place.
func (a *Asm) MovsxB(w bool, dst, src int) {
	a.bytes(rex(w, dst, 0, src), 0x0F, 0xBE, 0xC0|byte(dst&7)<<3|byte(src&7))
}
func (a *Asm) MovsxW(w bool, dst, src int) {
	a.bytes(rex(w, dst, 0, src), 0x0F, 0xBF, 0xC0|byte(dst&7)<<3|byte(src&7))
}

// CmpImm32 emits CMP reg, imm32 (sign-extended when w).
func (a *Asm) CmpImm32(w bool, reg int, v uint32) {
	a.bytes(rex(w, 0, 0, reg), 0x81, 0xC0|7<<3|byte(reg&7))
	a.u32(v)
}

// MovImm32 emits MOV reg32, imm32 (zero-extending).
func (a *Asm) MovImm32(reg int, v uint32) {
	if reg >= 8 {
		a.bytes(rex(false, 0, 0, reg))
	}
	a.bytes(0xB8 | byte(reg&7))
	a.u32(v)
}

// --- SSE scalar float support (xmm0-2 only, so no REX for the xmm side) ---

// sse emits pre 0F op modrm for xmm-xmm forms (pre 0 = no prefix).
func (a *Asm) sse(pre byte, op byte, dst, src int) {
	if pre != 0 {
		a.bytes(pre)
	}
	a.bytes(0x0F, op, 0xC0|byte(dst&7)<<3|byte(src&7))
}

// MovqXR / MovqRX move 64-bit patterns between xmm and GP registers.
func (a *Asm) MovqXR(x, r int) {
	a.bytes(0x66, rex(true, x, 0, r), 0x0F, 0x6E, 0xC0|byte(x&7)<<3|byte(r&7))
}
func (a *Asm) MovqRX(r, x int) {
	a.bytes(0x66, rex(true, x, 0, r), 0x0F, 0x7E, 0xC0|byte(x&7)<<3|byte(r&7))
}

// MovdXR / MovdRX are the 32-bit versions (MovdRX zero-extends the GP).
func (a *Asm) MovdXR(x, r int) { a.bytes(0x66, 0x0F, 0x6E, 0xC0|byte(x&7)<<3|byte(r&7)) }
func (a *Asm) MovdRX(r, x int) { a.bytes(0x66, 0x0F, 0x7E, 0xC0|byte(x&7)<<3|byte(r&7)) }

// Rounds emits ROUNDSS/ROUNDSD dst, src, mode (SSE4.1).
func (a *Asm) Rounds(dbl bool, dst, src int, mode byte) {
	op := byte(0x0A)
	if dbl {
		op = 0x0B
	}
	a.bytes(0x66, 0x0F, 0x3A, op, 0xC0|byte(dst&7)<<3|byte(src&7), mode)
}

// Ucomis emits UCOMISS/UCOMISD a, b.
func (a *Asm) Ucomis(dbl bool, x1, x2 int) {
	if dbl {
		a.bytes(0x66)
	}
	a.bytes(0x0F, 0x2E, 0xC0|byte(x1&7)<<3|byte(x2&7))
}

// Cvtsi2f emits CVTSI2SS/SD xmm, r (from64 selects the 64-bit GP source).
func (a *Asm) Cvtsi2f(dbl, from64 bool, x, r int) {
	pre := byte(0xF3)
	if dbl {
		pre = 0xF2
	}
	a.bytes(pre)
	if from64 || r >= 8 {
		a.bytes(rex(from64, x, 0, r))
	}
	a.bytes(0x0F, 0x2A, 0xC0|byte(x&7)<<3|byte(r&7))
}

// Cvttf2i emits CVTTSS2SI/CVTTSD2SI r, xmm (truncating).
func (a *Asm) Cvttf2i(dbl, to64 bool, r, x int) {
	pre := byte(0xF3)
	if dbl {
		pre = 0xF2
	}
	a.bytes(pre)
	if to64 || r >= 8 {
		a.bytes(rex(to64, r, 0, x))
	}
	a.bytes(0x0F, 0x2C, 0xC0|byte(r&7)<<3|byte(x&7))
}

// AndImm32 emits AND reg, imm32.
func (a *Asm) AndImm32(w bool, reg int, v uint32) {
	a.bytes(rex(w, 0, 0, reg), 0x81, 0xC0|4<<3|byte(reg&7))
	a.u32(v)
}

// AndByteAL / OrByteAL combine AL with CL (float eq/ne parity fixups).
func (a *Asm) AndByteAL() { a.bytes(0x20, 0xC8) }
func (a *Asm) OrByteAL()  { a.bytes(0x08, 0xC8) }

// PatchJmpTo / PatchJccTo rewrite a placeholder's rel32 toward an absolute
// code offset (used by the optimizing tier's IR-indexed patches).
func (a *Asm) PatchJmpTo(at, target int) {
	binary.LittleEndian.PutUint32(a.buf[at+1:], uint32(int32(target-(at+5))))
}

func (a *Asm) PatchJccTo(at, target int) {
	binary.LittleEndian.PutUint32(a.buf[at+2:], uint32(int32(target-(at+6))))
}

// ArithImm32 emits an 0x81-family op (sub 0 add, 5 sub, 7 cmp) reg, imm32.
func (a *Asm) ArithImm32(w bool, sub byte, reg int, v uint32) {
	a.bytes(rex(w, 0, 0, reg), 0x81, 0xC0|sub<<3|byte(reg&7))
	a.u32(v)
}

// LeaScaled emits LEA dst, [base + idx*8].
func (a *Asm) LeaScaled(dst, base, idx int) {
	a.bytes(rex(true, dst, idx, base), 0x8D, 0x04|byte(dst&7)<<3,
		3<<6|byte(idx&7)<<3|byte(base&7))
}

// movqXMem / movqMemX move 64-bit patterns between memory and xmm.
func (a *Asm) movqXMem(x, base int, off int32) {
	a.bytes(0xF3)
	if x >= 8 || base >= 8 {
		a.bytes(rex(false, x, 0, base))
	}
	a.bytes(0x0F, 0x7E, 0x80|byte(x&7)<<3|byte(base&7))
	a.u32(uint32(off))
}

func (a *Asm) movqMemX(x, base int, off int32) {
	a.bytes(0x66)
	if x >= 8 || base >= 8 {
		a.bytes(rex(false, x, 0, base))
	}
	a.bytes(0x0F, 0xD6, 0x80|byte(x&7)<<3|byte(base&7))
	a.u32(uint32(off))
}
