//go:build (darwin || linux) && arm64

package jit

import "fmt"

// Compile translates a function body to native code, or ErrUnsupported if it
// uses constructs outside the template subset (the caller then falls back to
// the interpreter). Straight-line subset first: locals, integer constants,
// i32/i64 arithmetic, comparisons and conversions; the operand stack lives
// at static offsets from the stack base register.
func Compile(fd *FuncDesc) (*Compiled, error) {
	c := &compiler{fd: fd}
	if err := c.run(); err != nil {
		return nil, err
	}
	code, err := AllocExec(c.a.Bytes())
	if err != nil {
		return nil, err
	}
	return &Compiled{Code: code, MaxHeight: c.maxH}, nil
}

type compiler struct {
	fd   *FuncDesc
	a    Asm
	h    int // static operand-stack height (slots)
	maxH int
}

func (c *compiler) push() int {
	s := c.h
	c.h++
	if c.h > c.maxH {
		c.maxH = c.h
	}
	return s
}

func (c *compiler) pop() int {
	c.h--
	return c.h
}

// slot load/store at a static height
func (c *compiler) ldr(r uint32, slot int) { c.a.LdrImm(r, RegStack, uint32(slot*8)) }
func (c *compiler) str(r uint32, slot int) { c.a.StrImm(r, RegStack, uint32(slot*8)) }

func (c *compiler) run() error {
	c.a.Prologue()
	body := c.fd.Body

	for pc := 0; pc < len(body); pc++ {
		op := body[pc]
		imm := uint64(0)
		if int(op) < len(opHasImm) && opHasImm[op] {
			imm = c.fd.Imms[pc]
			pc = int(c.fd.PcEnd[pc])
		}
		if err := c.emit(op, imm); err != nil {
			return err
		}
	}

	// fall off the end: results are already at slots [0, NumRets)
	if c.h != c.fd.NumRets {
		return fmt.Errorf("%w: end height %d != results %d", ErrUnsupported, c.h, c.fd.NumRets)
	}
	// slots are static, so the runtime stack index is only set once, here
	c.a.MovImm64(RegSp, uint64(c.h))
	c.a.Epilogue(StatusOK)
	return nil
}

// opHasImm marks single-immediate opcodes handled via Imms/PcEnd.
var opHasImm = buildOpHasImm()

func buildOpHasImm() (t [256]bool) {
	for _, op := range []byte{0x20, 0x21, 0x22, 0x41, 0x42, 0x43, 0x44} {
		t[op] = true
	}
	return
}

func (c *compiler) emit(op byte, imm uint64) error {
	a := &c.a
	switch op {
	case 0x01: // nop
	case 0x0b: // end (function level; blocks are not yet in the subset)
	case 0x1a: // drop
		c.pop()

	case 0x20: // local.get
		a.LdrImm(RegT0, RegLocals, uint32(imm*8))
		c.str(RegT0, c.push())
	case 0x21: // local.set
		c.ldr(RegT0, c.pop())
		a.StrImm(RegT0, RegLocals, uint32(imm*8))
	case 0x22: // local.tee
		c.ldr(RegT0, c.h-1)
		a.StrImm(RegT0, RegLocals, uint32(imm*8))

	case 0x41, 0x42, 0x43, 0x44: // i32/i64/f32/f64.const (bits pre-decoded)
		a.MovImm64(RegT0, imm)
		c.str(RegT0, c.push())

	// i32 binops (sign-extended results, mirroring the interpreter)
	case 0x6a, 0x6b, 0x6c, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76:
		c.ldr(RegT1, c.pop()) // v2
		c.ldr(RegT0, c.pop()) // v1
		switch op {
		case 0x6a:
			a.word(0x0B010000 | RegT1<<16 | RegT0<<5 | RegT0) // ADD W0,W0,W1
			a.Sxtw(RegT0, RegT0)
		case 0x6b:
			a.word(0x4B010000 | RegT1<<16 | RegT0<<5 | RegT0) // SUB
			a.Sxtw(RegT0, RegT0)
		case 0x6c:
			a.word(0x1B017C00 | RegT1<<16 | RegT0<<5 | RegT0) // MUL W
			a.Sxtw(RegT0, RegT0)
		case 0x71:
			a.word(0x0A010000 | RegT1<<16 | RegT0<<5 | RegT0) // AND W
			a.Uxtw(RegT0, RegT0)
		case 0x72:
			a.word(0x2A010000 | RegT1<<16 | RegT0<<5 | RegT0) // ORR W
			a.Uxtw(RegT0, RegT0)
		case 0x73:
			a.word(0x4A010000 | RegT1<<16 | RegT0<<5 | RegT0) // EOR W
			a.Uxtw(RegT0, RegT0)
		case 0x74:
			a.word(0x1AC12000 | RegT1<<16 | RegT0<<5 | RegT0) // LSLV W
			a.Uxtw(RegT0, RegT0)
		case 0x75:
			a.word(0x1AC12800 | RegT1<<16 | RegT0<<5 | RegT0) // ASRV W... (see note)
			a.Sxtw(RegT0, RegT0)
		case 0x76:
			a.word(0x1AC12400 | RegT1<<16 | RegT0<<5 | RegT0) // LSRV W
			a.Uxtw(RegT0, RegT0)
		}
		c.str(RegT0, c.push())

	// i64 binops
	case 0x7c, 0x7d, 0x7e, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88:
		c.ldr(RegT1, c.pop())
		c.ldr(RegT0, c.pop())
		switch op {
		case 0x7c:
			a.word(0x8B010000 | RegT1<<16 | RegT0<<5 | RegT0) // ADD X
		case 0x7d:
			a.word(0xCB010000 | RegT1<<16 | RegT0<<5 | RegT0) // SUB X
		case 0x7e:
			a.word(0x9B017C00 | RegT1<<16 | RegT0<<5 | RegT0) // MUL X
		case 0x83:
			a.word(0x8A010000 | RegT1<<16 | RegT0<<5 | RegT0) // AND X
		case 0x84:
			a.word(0xAA010000 | RegT1<<16 | RegT0<<5 | RegT0) // ORR X
		case 0x85:
			a.word(0xCA010000 | RegT1<<16 | RegT0<<5 | RegT0) // EOR X
		case 0x86:
			a.word(0x9AC12000 | RegT1<<16 | RegT0<<5 | RegT0) // LSLV X
		case 0x87:
			a.word(0x9AC12800 | RegT1<<16 | RegT0<<5 | RegT0) // ASRV X
		case 0x88:
			a.word(0x9AC12400 | RegT1<<16 | RegT0<<5 | RegT0) // LSRV X
		}
		c.str(RegT0, c.push())

	// conversions
	case 0xa7: // i32.wrap_i64
		c.ldr(RegT0, c.h-1)
		a.Uxtw(RegT0, RegT0)
		c.str(RegT0, c.h-1)
	case 0xac: // i64.extend_i32_s
		c.ldr(RegT0, c.h-1)
		a.Sxtw(RegT0, RegT0)
		c.str(RegT0, c.h-1)
	case 0xad: // i64.extend_i32_u
		c.ldr(RegT0, c.h-1)
		a.Uxtw(RegT0, RegT0)
		c.str(RegT0, c.h-1)

	default:
		return fmt.Errorf("%w: opcode %#x", ErrUnsupported, op)
	}
	return nil
}
