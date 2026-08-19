//go:build (darwin || linux) && arm64

package jit

import (
	"encoding/binary"
	"errors"
	"testing"
)

// golden encodings, externally validated with capstone:
//
//	f8627823  ldr x3, [x1, x2, lsl #3]
//	f8227824  str x4, [x1, x2, lsl #3]
//	d1000442  sub x2, x2, #1
//	f8627825  ldr x5, [x1, x2, lsl #3]
//	35000046  cbnz w6, #+8
//	1e214041  fneg s1, s2
//	1e614083  fneg d3, d4
func TestAsmHelperEncodings(t *testing.T) {
	var a Asm
	a.LdrIdx(3, 1, 2)
	a.StrIdx(4, 1, 2)
	a.PopReg(5)
	a.Cbnz32(6, 8)
	a.FUn2(false, 0x1E214000, 1, 2)
	a.FUn2(true, 0x1E214000, 3, 4)
	want := []uint32{0xf8627823, 0xf8227824, 0xd1000442, 0xf8627825,
		0x35000046, 0x1e214041, 0x1e614083}
	buf := a.Bytes()
	if len(buf) != len(want)*4 {
		t.Fatalf("emitted %d bytes, want %d", len(buf), len(want)*4)
	}
	for i, w := range want {
		if got := binary.LittleEndian.Uint32(buf[i*4:]); got != w {
			t.Errorf("word %d: got %08x want %08x", i, got, w)
		}
	}
}

func TestAllocExec(t *testing.T) {
	if _, err := AllocExec(nil); err == nil {
		t.Fatal("empty code accepted")
	}
	// RET; executing the mapping proves it became executable
	code := []byte{0xc0, 0x03, 0x5f, 0xd6}
	mem, err := AllocExec(code)
	if err != nil {
		t.Fatal(err)
	}
	if &mem[0] == &code[0] {
		t.Fatal("AllocExec returned the input, not a fresh mapping")
	}
	if err := Free(mem); err != nil {
		t.Fatal(err)
	}
}

// addSubR1's large-offset arm materializes the offset in a scratch register.
func TestAddSubR1LargeOffset(t *testing.T) {
	for _, add := range []bool{true, false} {
		g := &optGen{}
		g.addSubR1(0, add)
		if len(g.a.Bytes()) != 0 {
			t.Fatal("offset 0 emitted code")
		}
		g.addSubR1(8, add)
		small := len(g.a.Bytes())
		if small == 0 {
			t.Fatal("small offset emitted nothing")
		}
		g.addSubR1(5000, add)
		if len(g.a.Bytes()) <= small+4 {
			t.Fatalf("add=%v: large offset did not take the materialize path", add)
		}
	}
}

// loc reports the allocated register of a compacted value, or absence.
func TestOptGenLoc(t *testing.T) {
	g := &optGen{al: &allocation{
		compact: map[int]int{7: 0},
		loc:     []vloc{{reg: 3}},
	}}
	if r, ok := g.loc(7); !ok || r != 3 {
		t.Fatalf("loc(7) = %d,%v", r, ok)
	}
	if _, ok := g.loc(8); ok {
		t.Fatal("loc(8) resolved an unallocated value")
	}
}

// both tiers must refuse bodies with unskippable/unsupported constructs.
func TestCompileRejects(t *testing.T) {
	fd := &FuncDesc{
		Body:  []byte{0xfe}, // no such opcode: no PcEnd entry, not single-byte
		Imms:  make([]uint64, 1),
		PcEnd: make([]uint32, 1),
	}
	if _, err := CompileBaseline(fd); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("baseline accepted an unknown opcode: %v", err)
	}
	if _, err := CompileOpt(fd); err == nil {
		t.Fatal("opt tier accepted an unknown opcode")
	}
	// a body that ends with an open block
	fd2 := &FuncDesc{
		Body:  []byte{0x02, 0x40},
		Imms:  make([]uint64, 2),
		PcEnd: []uint32{1, 0},
	}
	if _, err := CompileBaseline(fd2); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("baseline accepted an unterminated block: %v", err)
	}
}
