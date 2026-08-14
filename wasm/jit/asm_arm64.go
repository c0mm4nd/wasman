//go:build (darwin || linux) && arm64

package jit

import "encoding/binary"

// Asm is a minimal arm64 instruction emitter for the template compiler.
type Asm struct{ buf []byte }

func (a *Asm) word(w uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], w)
	a.buf = append(a.buf, b[:]...)
}

// Bytes returns the emitted code.
func (a *Asm) Bytes() []byte { return a.buf }

// Len returns the current length in bytes.
func (a *Asm) Len() int { return len(a.buf) }

// LdrImm emits LDR Xt, [Xn, #off] (off must be a multiple of 8, < 32760).
func (a *Asm) LdrImm(t, n uint32, off uint32) { a.word(0xF9400000 | (off/8)<<10 | n<<5 | t) }

// StrImm emits STR Xt, [Xn, #off].
func (a *Asm) StrImm(t, n uint32, off uint32) { a.word(0xF9000000 | (off/8)<<10 | n<<5 | t) }

// LdrIdx emits LDR Xt, [Xn, Xm, LSL #3].
func (a *Asm) LdrIdx(t, n, m uint32) { a.word(0xF8607800 | m<<16 | n<<5 | t) }

// StrIdx emits STR Xt, [Xn, Xm, LSL #3].
func (a *Asm) StrIdx(t, n, m uint32) { a.word(0xF8207800 | m<<16 | n<<5 | t) }

// Movz emits MOVZ Xd, #imm16, LSL #(hw*16).
func (a *Asm) Movz(d uint32, imm16 uint32, hw uint32) {
	a.word(0xD2800000 | hw<<21 | imm16<<5 | d)
}

// Movk emits MOVK Xd, #imm16, LSL #(hw*16).
func (a *Asm) Movk(d uint32, imm16 uint32, hw uint32) {
	a.word(0xF2800000 | hw<<21 | imm16<<5 | d)
}

// MovImm64 materializes an arbitrary 64-bit constant into Xd.
func (a *Asm) MovImm64(d uint32, v uint64) {
	a.Movz(d, uint32(v&0xffff), 0)
	if x := uint32(v >> 16 & 0xffff); x != 0 {
		a.Movk(d, x, 1)
	}
	if x := uint32(v >> 32 & 0xffff); x != 0 {
		a.Movk(d, x, 2)
	}
	if x := uint32(v >> 48 & 0xffff); x != 0 {
		a.Movk(d, x, 3)
	}
}

// AddImm emits ADD Xd, Xn, #imm12.
func (a *Asm) AddImm(d, n uint32, imm uint32) { a.word(0x91000000 | imm<<10 | n<<5 | d) }

// SubImm emits SUB Xd, Xn, #imm12.
func (a *Asm) SubImm(d, n uint32, imm uint32) { a.word(0xD1000000 | imm<<10 | n<<5 | d) }

// Ret emits RET.
func (a *Asm) Ret() { a.word(0xD65F03C0) }

// register assignment shared with the trampoline contract
const (
	RegCtx    = 0
	RegStack  = 1
	RegSp     = 2
	RegLocals = 3
	RegMem    = 4
	RegMemLen = 5
	RegT0     = 6
	RegT1     = 7
)

// Prologue loads the working registers from *Ctx (in R0).
func (a *Asm) Prologue() {
	a.LdrImm(RegStack, RegCtx, 0)
	a.LdrImm(RegSp, RegCtx, 8)
	a.LdrImm(RegLocals, RegCtx, 16)
	a.LdrImm(RegMem, RegCtx, 24)
	a.LdrImm(RegMemLen, RegCtx, 32)
}

// Epilogue stores the stack index back and returns the given status.
func (a *Asm) Epilogue(status uint32) {
	a.StrImm(RegSp, RegCtx, 8)
	a.Movz(RegCtx, status, 0) // status into R0
	a.Ret()
}

// PushReg emits stack[sp++] = Xr.
func (a *Asm) PushReg(r uint32) {
	a.StrIdx(r, RegStack, RegSp)
	a.AddImm(RegSp, RegSp, 1)
}

// PopReg emits Xr = stack[--sp].
func (a *Asm) PopReg(r uint32) {
	a.SubImm(RegSp, RegSp, 1)
	a.LdrIdx(r, RegStack, RegSp)
}

// Sxtw emits SXTW Xd, Wn (sign-extend 32->64).
func (a *Asm) Sxtw(d, n uint32) { a.word(0x93407C00 | n<<5 | d) }

// Uxtw emits MOV Wd, Wn (zero upper 32 bits).
func (a *Asm) Uxtw(d, n uint32) { a.word(0x2A0003E0 | n<<16 | d) }
