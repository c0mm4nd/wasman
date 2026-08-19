package wasm

import (
	"strings"
	"testing"

	"github.com/c0mm4nd/wasman/types"
	"github.com/c0mm4nd/wasman/utils"
)

func TestMemoryHelpers(t *testing.T) {
	if got := MemoryPagesToBytesNum(3); got != 3*65536 {
		t.Fatalf("MemoryPagesToBytesNum(3) = %d", got)
	}
	m := &Memory{Value: make([]byte, 2*65536)}
	if got := m.PageSize(); got != 2 {
		t.Fatalf("PageSize = %d, want 2", got)
	}
	// growth within an explicit max returns the OLD page count
	m.Max = utils.Uint32Ptr(4)
	if got := m.Grow(2); got != 2 {
		t.Fatalf("Grow(2) = %d, want old size 2", got)
	}
	if got := m.PageSize(); got != 4 {
		t.Fatalf("PageSize after grow = %d, want 4", got)
	}
	// exceeding max fails with the wasm -1 sentinel and leaves size intact
	if got := m.Grow(1); got != 0xffffffff {
		t.Fatalf("Grow past max = %#x, want 0xffffffff", got)
	}
	if got := m.PageSize(); got != 4 {
		t.Fatalf("failed grow changed the size to %d", got)
	}
	// nil max: unbounded
	m2 := &Memory{Value: nil}
	if got := m2.Grow(1); got != 0 {
		t.Fatalf("Grow on empty = %d, want 0", got)
	}
	if got := m2.PageSize(); got != 1 {
		t.Fatalf("PageSize = %d, want 1", got)
	}
}

func TestEvalConstExpr(t *testing.T) {
	g := &Global{GlobalType: &types.GlobalType{ValType: types.ValueTypeI64}, Val: int64(40)}
	ins := &Instance{IndexSpace: &IndexSpace{Globals: []*Global{g}}}

	ok := []struct {
		name string
		raw  []byte
		want interface{}
	}{
		{"i32.const", []byte{0x41, 0x05}, int32(5)},
		{"i64.const", []byte{0x42, 0x7f}, int64(-1)},
		{"f32.const", []byte{0x43, 0x00, 0x00, 0xc0, 0x3f}, float32(1.5)},
		{"f64.const", []byte{0x44, 0, 0, 0, 0, 0, 0, 0xf8, 0x3f}, float64(1.5)},
		{"global.get", []byte{0x23, 0x00}, int64(40)},
		{"i32.add", []byte{0x41, 0x02, 0x41, 0x03, 0x6a}, int32(5)},
		{"i32.sub", []byte{0x41, 0x02, 0x41, 0x03, 0x6b}, int32(-1)},
		{"i32.mul", []byte{0x41, 0x02, 0x41, 0x03, 0x6c}, int32(6)},
		{"i64.add", []byte{0x42, 0x02, 0x42, 0x03, 0x7c}, int64(5)},
		{"i64.sub", []byte{0x42, 0x02, 0x42, 0x03, 0x7d}, int64(-1)},
		{"i64.mul", []byte{0x42, 0x02, 0x42, 0x03, 0x7e}, int64(6)},
		{"global+const", []byte{0x23, 0x00, 0x42, 0x02, 0x7c}, int64(42)},
	}
	for _, c := range ok {
		v, err := ins.evalConstExpr(c.raw)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if v != c.want {
			t.Errorf("%s: got %v want %v", c.name, v, c.want)
		}
	}

	bad := []struct {
		name string
		raw  []byte
		msg  string
	}{
		{"i32 truncated", []byte{0x41}, "read i32"},
		{"i64 truncated", []byte{0x42}, "read i64"},
		{"f32 truncated", []byte{0x43, 0x00}, "read f32"},
		{"f64 truncated", []byte{0x44, 0x00}, "read f64"},
		{"global idx truncated", []byte{0x23}, "read global index"},
		{"global out of range", []byte{0x23, 0x07}, "out of range"},
		{"arith underflow", []byte{0x41, 0x01, 0x6a}, "underflow"},
		{"invalid opcode", []byte{0x0b}, "invalid const opcode"},
		{"two values left", []byte{0x41, 0x01, 0x41, 0x02}, "left 2 values"},
		{"i32 arith on i64", []byte{0x42, 0x01, 0x42, 0x02, 0x6a}, "non-i32 operands"},
		{"i64 arith on i32", []byte{0x41, 0x01, 0x41, 0x02, 0x7c}, "non-i64 operands"},
	}
	for _, c := range bad {
		_, err := ins.evalConstExpr(c.raw)
		if err == nil {
			t.Errorf("%s: no error", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.msg) {
			t.Errorf("%s: error %q does not mention %q", c.name, err, c.msg)
		}
	}
}

// fetch* decode immediates at the frame's PC; a malformed body must error
// instead of panicking.
func TestFetchImmediateErrors(t *testing.T) {
	mk := func(body []byte) *Instance {
		return &Instance{Active: &Frame{Func: &wasmFunc{body: body}}}
	}
	// 0x80 starts a multi-byte LEB that never terminates
	if _, err := mk([]byte{0x80}).fetchInt32(); err == nil {
		t.Error("fetchInt32 on truncated LEB: no error")
	}
	if _, err := mk([]byte{0x80}).fetchUint32(); err == nil {
		t.Error("fetchUint32 on truncated LEB: no error")
	}
	if _, err := mk([]byte{0x80}).fetchInt64(); err == nil {
		t.Error("fetchInt64 on truncated LEB: no error")
	}
	// happy path through the same helpers
	ins := mk([]byte{0x2a})
	if v, err := ins.fetchInt32(); err != nil || v != 42 {
		t.Errorf("fetchInt32 = %d, %v", v, err)
	}
}

// the 0xfc misc prefix with an unknown sub-opcode must report
// ErrUnknownOpcode through the slow (no pre-decoded immediates) path.
func TestMiscPrefixUnknown(t *testing.T) {
	ins := &Instance{Active: &Frame{Func: &wasmFunc{body: []byte{0xfc, 0x63}}}}
	err := miscPrefix(ins)
	if err == nil || !strings.Contains(err.Error(), "unknown opcode") {
		t.Fatalf("want unknown-opcode error, got %v", err)
	}
	// truncated sub-opcode
	ins2 := &Instance{Active: &Frame{Func: &wasmFunc{body: []byte{0xfc}}}}
	if err := miscPrefix(ins2); err == nil {
		t.Fatal("truncated misc sub-opcode: no error")
	}
}
