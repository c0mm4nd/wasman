//go:build (darwin || linux) && arm64

package jit

import (
	"testing"
	"unsafe"
)

func runWithMem(t *testing.T, fd *FuncDesc, locals, mem []uint64, memBytes int) ([]uint64, uint32) {
	t.Helper()
	cd, err := Compile(fd)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Free(cd.Code) })
	stack := make([]uint64, cd.MaxHeight+1)
	ctx := &Ctx{Stack: ptrOf(stack), MemLen: uint64(memBytes)}
	if len(locals) > 0 {
		ctx.Locals = ptrOf(locals)
	}
	if len(mem) > 0 {
		ctx.Mem = uintptr(unsafe.Pointer(&mem[0]))
	}
	st := Call(cd.Code, ctx)
	return stack[:ctx.Sp], st
}

// TestLoadStore covers widths, sign/zero extension and offsets.
func TestLoadStore(t *testing.T) {
	mem := make([]uint64, 16) // 128 bytes
	// store8 0xff at addr 5, load back signed and unsigned
	fd := assemble([]ins{
		{0x41, 5, 1},    // addr
		{0x41, 0xff, 1}, // value
		{0x3a, 0, 1},    // i32.store8 (offset 0)
		{0x41, 5, 1},
		{0x2c, 0, 1}, // i32.load8_s -> 0xffffffff (sign to 32, zero to 64)
		{0x41, 5, 1},
		{0x2d, 0, 1}, // i32.load8_u -> 0xff
		{0x41, 0, 1},
		{0x30, 5, 1}, // i64.load8_s offset=5 -> sign to 64
		{0x0b, 0, 0},
	}, 0, 0, 3)
	got, st := runWithMem(t, fd, nil, mem, 128)
	if st != StatusOK {
		t.Fatalf("status %d", st)
	}
	want := []uint64{0xffffffff, 0xff, 0xffffffffffffffff}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("slot %d: got %#x, want %#x", i, got[i], w)
		}
	}
}

// TestLoadStoreWide covers 32/64-bit round trips at unaligned addresses.
func TestLoadStoreWide(t *testing.T) {
	mem := make([]uint64, 16)
	fd := assemble([]ins{
		{0x41, 3, 1},                  // unaligned addr
		{0x42, 0x1122334455667788, 1}, // value
		{0x37, 0, 1},                  // i64.store
		{0x41, 3, 1},
		{0x29, 0, 1}, // i64.load
		{0x41, 3, 1},
		{0x28, 0, 1}, // i32.load -> low 32 zero-extended
		{0x41, 3, 1},
		{0x34, 0, 1}, // i64.load32_s -> sign-extended low 32
		{0x0b, 0, 0},
	}, 0, 0, 3)
	got, st := runWithMem(t, fd, nil, mem, 128)
	if st != StatusOK {
		t.Fatalf("status %d", st)
	}
	if got[0] != 0x1122334455667788 || got[1] != 0x55667788 || got[2] != 0x55667788 {
		t.Fatalf("got %#x", got)
	}
}

// TestOOBTrap checks the shared bounds-trap stub, including offset overflow.
func TestOOBTrap(t *testing.T) {
	mem := make([]uint64, 2) // 16 bytes
	for _, tc := range []struct {
		addr, off uint64
	}{
		{16, 0},         // addr at the boundary
		{13, 0},         // width crosses the boundary (i32.load needs 4)
		{0, 16},         // offset pushes past
		{0xffffffff, 8}, // large u32 addr + offset (no wrap)
	} {
		fd := assemble([]ins{
			{0x41, tc.addr, 1},
			{0x28, tc.off, 1}, // i32.load
			{0x1a, 0, 0},      // drop
			{0x0b, 0, 0},
		}, 0, 0, 0)
		_, st := runWithMem(t, fd, nil, mem, 16)
		if st != StatusMemOOB {
			t.Fatalf("addr=%d off=%d: status %d, want OOB", tc.addr, tc.off, st)
		}
	}
	// and the last valid access must still succeed
	fd := assemble([]ins{
		{0x41, 12, 1}, {0x28, 0, 1}, {0x1a, 0, 0}, {0x0b, 0, 0},
	}, 0, 0, 0)
	if _, st := runWithMem(t, fd, nil, mem, 16); st != StatusOK {
		t.Fatalf("in-bounds access trapped: %d", st)
	}
}

// TestMemFillSum runs a memrw-style loop: fill n bytes then sum them.
func TestMemFillSum(t *testing.T) {
	const n = 4096
	mem := make([]uint64, n/8)
	// locals: 0=i, 1=acc, param n at 2
	fd := assemble([]ins{
		// fill: for i in 0..n: mem[i] = i & 0xff
		{0x02, blk(0, 0), 1},
		{0x03, blk(0, 0), 1},
		{0x20, 0, 1}, {0x20, 2, 1}, {0x4e, 0, 0}, {0x0d, 1, 1}, // i >= n? break
		{0x20, 0, 1}, {0x20, 0, 1}, {0x3a, 0, 1}, // mem[i] = i (store8 truncates)
		{0x20, 0, 1}, {0x41, 1, 1}, {0x6a, 0, 0}, {0x21, 0, 1}, // i++
		{0x0c, 0, 1}, {0x0b, 0, 0}, {0x0b, 0, 0},
		// sum: reset i, accumulate
		{0x41, 0, 1}, {0x21, 0, 1},
		{0x02, blk(0, 0), 1},
		{0x03, blk(0, 0), 1},
		{0x20, 0, 1}, {0x20, 2, 1}, {0x4e, 0, 0}, {0x0d, 1, 1},
		{0x20, 1, 1}, {0x20, 0, 1}, {0x2d, 0, 1}, {0x6a, 0, 0}, {0x21, 1, 1},
		{0x20, 0, 1}, {0x41, 1, 1}, {0x6a, 0, 0}, {0x21, 0, 1},
		{0x0c, 0, 1}, {0x0b, 0, 0}, {0x0b, 0, 0},
		{0x20, 1, 1},
		{0x0b, 0, 0},
	}, 3, 0, 1)
	var want uint64
	for i := 0; i < n; i++ {
		want += uint64(i & 0xff)
	}
	got, st := runWithMem(t, fd, []uint64{0, 0, n}, mem, n)
	if st != StatusOK {
		t.Fatalf("status %d", st)
	}
	if got[0] != want {
		t.Fatalf("got %d, want %d", got[0], want)
	}
}
