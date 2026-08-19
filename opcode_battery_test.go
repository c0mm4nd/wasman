package wasman_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/tollstation"
	"github.com/c0mm4nd/wasman/wat"
)

// batteryCase is one expression with its expected result bits (i64-widened).
type batteryCase struct {
	name string
	expr string // a wat expression yielding one value
	typ  string // result type
	want uint64 // expected raw bits (i32/f32 in the low half)
}

// opcodeBattery enumerates (nearly) every numeric, comparison, conversion,
// parametric and memory opcode with fixed operands. The interpreter's three
// dispatch modes (inline fast path, metered table dispatch, canonicalizing
// table dispatch) must all produce these exact bits — the fast path and the
// per-op table handlers are separate code and must never drift apart.
var opcodeBattery = []batteryCase{
	// i32 arithmetic / bit ops
	{"i32.add", "(i32.add (i32.const 7) (i32.const -3))", "i32", 4},
	{"i32.sub", "(i32.sub (i32.const 3) (i32.const 7))", "i32", 0xfffffffc},
	{"i32.mul", "(i32.mul (i32.const -6) (i32.const 7))", "i32", 0xffffffd6},
	{"i32.div_s", "(i32.div_s (i32.const -7) (i32.const 2))", "i32", 0xfffffffd},
	{"i32.div_u", "(i32.div_u (i32.const -7) (i32.const 2))", "i32", 0x7ffffffc},
	{"i32.rem_s", "(i32.rem_s (i32.const -7) (i32.const 2))", "i32", 0xffffffff},
	{"i32.rem_u", "(i32.rem_u (i32.const -7) (i32.const 2))", "i32", 1},
	{"i32.and", "(i32.and (i32.const 0xff00ff) (i32.const 0x0ff0f0))", "i32", 0x0f00f0},
	{"i32.or", "(i32.or (i32.const 0xff00ff) (i32.const 0x0ff0f0))", "i32", 0xfff0ff},
	{"i32.xor", "(i32.xor (i32.const 0xff00ff) (i32.const 0x0ff0f0))", "i32", 0xf0f00f},
	{"i32.shl", "(i32.shl (i32.const 1) (i32.const 33))", "i32", 2},
	{"i32.shr_s", "(i32.shr_s (i32.const -8) (i32.const 1))", "i32", 0xfffffffc},
	{"i32.shr_u", "(i32.shr_u (i32.const -8) (i32.const 1))", "i32", 0x7ffffffc},
	{"i32.rotl", "(i32.rotl (i32.const 0x80000001) (i32.const 1))", "i32", 3},
	{"i32.rotr", "(i32.rotr (i32.const 0x80000001) (i32.const 1))", "i32", 0xc0000000},
	{"i32.clz", "(i32.clz (i32.const 0x00ffffff))", "i32", 8},
	{"i32.ctz", "(i32.ctz (i32.const 0x00ff0000))", "i32", 16},
	{"i32.popcnt", "(i32.popcnt (i32.const 0xf0f0f0f0))", "i32", 16},
	{"i32.eqz", "(i32.eqz (i32.const 0))", "i32", 1},
	{"i32.eq", "(i32.eq (i32.const 5) (i32.const 5))", "i32", 1},
	{"i32.ne", "(i32.ne (i32.const 5) (i32.const 5))", "i32", 0},
	{"i32.lt_s", "(i32.lt_s (i32.const -1) (i32.const 0))", "i32", 1},
	{"i32.lt_u", "(i32.lt_u (i32.const -1) (i32.const 0))", "i32", 0},
	{"i32.gt_s", "(i32.gt_s (i32.const -1) (i32.const 0))", "i32", 0},
	{"i32.gt_u", "(i32.gt_u (i32.const -1) (i32.const 0))", "i32", 1},
	{"i32.le_s", "(i32.le_s (i32.const -1) (i32.const -1))", "i32", 1},
	{"i32.le_u", "(i32.le_u (i32.const 2) (i32.const 1))", "i32", 0},
	{"i32.ge_s", "(i32.ge_s (i32.const -2) (i32.const -1))", "i32", 0},
	{"i32.ge_u", "(i32.ge_u (i32.const -2) (i32.const -1))", "i32", 0},
	// i64
	{"i64.add", "(i64.add (i64.const 7) (i64.const -3))", "i64", 4},
	{"i64.sub", "(i64.sub (i64.const 3) (i64.const 7))", "i64", 0xfffffffffffffffc},
	{"i64.mul", "(i64.mul (i64.const -6) (i64.const 7))", "i64", 0xffffffffffffffd6},
	{"i64.div_s", "(i64.div_s (i64.const -7) (i64.const 2))", "i64", 0xfffffffffffffffd},
	{"i64.div_u", "(i64.div_u (i64.const -7) (i64.const 2))", "i64", 0x7ffffffffffffffc},
	{"i64.rem_s", "(i64.rem_s (i64.const -7) (i64.const 2))", "i64", 0xffffffffffffffff},
	{"i64.rem_u", "(i64.rem_u (i64.const -7) (i64.const 2))", "i64", 1},
	{"i64.and", "(i64.and (i64.const 0xff00ff) (i64.const 0x0ff0f0))", "i64", 0x0f00f0},
	{"i64.or", "(i64.or (i64.const 0xff00ff) (i64.const 0x0ff0f0))", "i64", 0xfff0ff},
	{"i64.xor", "(i64.xor (i64.const 0xff00ff) (i64.const 0x0ff0f0))", "i64", 0xf0f00f},
	{"i64.shl", "(i64.shl (i64.const 1) (i64.const 65))", "i64", 2},
	{"i64.shr_s", "(i64.shr_s (i64.const -8) (i64.const 1))", "i64", 0xfffffffffffffffc},
	{"i64.shr_u", "(i64.shr_u (i64.const -8) (i64.const 1))", "i64", 0x7ffffffffffffffc},
	{"i64.rotl", "(i64.rotl (i64.const 0x8000000000000001) (i64.const 1))", "i64", 3},
	{"i64.rotr", "(i64.rotr (i64.const 0x8000000000000001) (i64.const 1))", "i64", 0xc000000000000000},
	{"i64.clz", "(i64.clz (i64.const 0x00ffffffffffffff))", "i64", 8},
	{"i64.ctz", "(i64.ctz (i64.const 0x00ff000000000000))", "i64", 48},
	{"i64.popcnt", "(i64.popcnt (i64.const 0xf0f0f0f0f0f0f0f0))", "i64", 32},
	{"i64.eqz", "(i64.eqz (i64.const 1))", "i32", 0},
	{"i64.eq", "(i64.eq (i64.const 5) (i64.const 5))", "i32", 1},
	{"i64.ne", "(i64.ne (i64.const 5) (i64.const 5))", "i32", 0},
	{"i64.lt_s", "(i64.lt_s (i64.const -1) (i64.const 0))", "i32", 1},
	{"i64.lt_u", "(i64.lt_u (i64.const -1) (i64.const 0))", "i32", 0},
	{"i64.gt_s", "(i64.gt_s (i64.const -1) (i64.const 0))", "i32", 0},
	{"i64.gt_u", "(i64.gt_u (i64.const -1) (i64.const 0))", "i32", 1},
	{"i64.le_s", "(i64.le_s (i64.const -1) (i64.const -1))", "i32", 1},
	{"i64.le_u", "(i64.le_u (i64.const 2) (i64.const 1))", "i32", 0},
	{"i64.ge_s", "(i64.ge_s (i64.const -2) (i64.const -1))", "i32", 0},
	{"i64.ge_u", "(i64.ge_u (i64.const -2) (i64.const -1))", "i32", 0},
	// conversions & extensions
	{"i32.wrap_i64", "(i32.wrap_i64 (i64.const 0x1_0000_0005))", "i32", 5},
	{"i64.extend_i32_s", "(i64.extend_i32_s (i32.const -2))", "i64", 0xfffffffffffffffe},
	{"i64.extend_i32_u", "(i64.extend_i32_u (i32.const -2))", "i64", 0xfffffffe},
	{"i32.extend8_s", "(i32.extend8_s (i32.const 0x80))", "i32", 0xffffff80},
	{"i32.extend16_s", "(i32.extend16_s (i32.const 0x8000))", "i32", 0xffff8000},
	{"i64.extend8_s", "(i64.extend8_s (i64.const 0x80))", "i64", 0xffffffffffffff80},
	{"i64.extend16_s", "(i64.extend16_s (i64.const 0x8000))", "i64", 0xffffffffffff8000},
	{"i64.extend32_s", "(i64.extend32_s (i64.const 0x80000000))", "i64", 0xffffffff80000000},
	// float arithmetic (fixed, exactly-representable operands)
	{"f32.add", "(i32.reinterpret_f32 (f32.add (f32.const 1.5) (f32.const 2.25)))", "i32", 0x40700000},
	{"f32.sub", "(i32.reinterpret_f32 (f32.sub (f32.const 1.5) (f32.const 2.5)))", "i32", 0xbf800000},
	{"f32.mul", "(i32.reinterpret_f32 (f32.mul (f32.const 3) (f32.const 0.5)))", "i32", 0x3fc00000},
	{"f32.div", "(i32.reinterpret_f32 (f32.div (f32.const 3) (f32.const 2)))", "i32", 0x3fc00000},
	{"f32.abs", "(i32.reinterpret_f32 (f32.abs (f32.const -2)))", "i32", 0x40000000},
	{"f32.neg", "(i32.reinterpret_f32 (f32.neg (f32.const 2)))", "i32", 0xc0000000},
	{"f32.sqrt", "(i32.reinterpret_f32 (f32.sqrt (f32.const 4)))", "i32", 0x40000000},
	{"f32.min", "(i32.reinterpret_f32 (f32.min (f32.const 1) (f32.const 2)))", "i32", 0x3f800000},
	{"f32.max", "(i32.reinterpret_f32 (f32.max (f32.const 1) (f32.const 2)))", "i32", 0x40000000},
	{"f32.copysign", "(i32.reinterpret_f32 (f32.copysign (f32.const 1) (f32.const -2)))", "i32", 0xbf800000},
	{"f32.ceil", "(i32.reinterpret_f32 (f32.ceil (f32.const 1.25)))", "i32", 0x40000000},
	{"f32.floor", "(i32.reinterpret_f32 (f32.floor (f32.const 1.75)))", "i32", 0x3f800000},
	{"f32.trunc", "(i32.reinterpret_f32 (f32.trunc (f32.const -1.75)))", "i32", 0xbf800000},
	{"f32.nearest", "(i32.reinterpret_f32 (f32.nearest (f32.const 2.5)))", "i32", 0x40000000},
	{"f32.eq", "(f32.eq (f32.const 1) (f32.const 1))", "i32", 1},
	{"f32.ne", "(f32.ne (f32.const 1) (f32.const 1))", "i32", 0},
	{"f32.lt", "(f32.lt (f32.const 1) (f32.const 2))", "i32", 1},
	{"f32.gt", "(f32.gt (f32.const 1) (f32.const 2))", "i32", 0},
	{"f32.le", "(f32.le (f32.const 2) (f32.const 2))", "i32", 1},
	{"f32.ge", "(f32.ge (f32.const 1) (f32.const 2))", "i32", 0},
	{"f64.add", "(i64.reinterpret_f64 (f64.add (f64.const 1.5) (f64.const 2.25)))", "i64", 0x400e000000000000},
	{"f64.sub", "(i64.reinterpret_f64 (f64.sub (f64.const 1.5) (f64.const 2.5)))", "i64", 0xbff0000000000000},
	{"f64.mul", "(i64.reinterpret_f64 (f64.mul (f64.const 3) (f64.const 0.5)))", "i64", 0x3ff8000000000000},
	{"f64.div", "(i64.reinterpret_f64 (f64.div (f64.const 3) (f64.const 2)))", "i64", 0x3ff8000000000000},
	{"f64.abs", "(i64.reinterpret_f64 (f64.abs (f64.const -2)))", "i64", 0x4000000000000000},
	{"f64.neg", "(i64.reinterpret_f64 (f64.neg (f64.const 2)))", "i64", 0xc000000000000000},
	{"f64.sqrt", "(i64.reinterpret_f64 (f64.sqrt (f64.const 4)))", "i64", 0x4000000000000000},
	{"f64.min", "(i64.reinterpret_f64 (f64.min (f64.const 1) (f64.const 2)))", "i64", 0x3ff0000000000000},
	{"f64.max", "(i64.reinterpret_f64 (f64.max (f64.const 1) (f64.const 2)))", "i64", 0x4000000000000000},
	{"f64.copysign", "(i64.reinterpret_f64 (f64.copysign (f64.const 1) (f64.const -2)))", "i64", 0xbff0000000000000},
	{"f64.ceil", "(i64.reinterpret_f64 (f64.ceil (f64.const 1.25)))", "i64", 0x4000000000000000},
	{"f64.floor", "(i64.reinterpret_f64 (f64.floor (f64.const 1.75)))", "i64", 0x3ff0000000000000},
	{"f64.trunc", "(i64.reinterpret_f64 (f64.trunc (f64.const -1.75)))", "i64", 0xbff0000000000000},
	{"f64.nearest", "(i64.reinterpret_f64 (f64.nearest (f64.const 2.5)))", "i64", 0x4000000000000000},
	{"f64.eq", "(f64.eq (f64.const 1) (f64.const 1))", "i32", 1},
	{"f64.ne", "(f64.ne (f64.const 1) (f64.const 1))", "i32", 0},
	{"f64.lt", "(f64.lt (f64.const 1) (f64.const 2))", "i32", 1},
	{"f64.gt", "(f64.gt (f64.const 1) (f64.const 2))", "i32", 0},
	{"f64.le", "(f64.le (f64.const 2) (f64.const 2))", "i32", 1},
	{"f64.ge", "(f64.ge (f64.const 1) (f64.const 2))", "i32", 0},
	// float <-> int conversions (in-range operands)
	{"i32.trunc_f32_s", "(i32.trunc_f32_s (f32.const -2.75))", "i32", 0xfffffffe},
	{"i32.trunc_f32_u", "(i32.trunc_f32_u (f32.const 2.75))", "i32", 2},
	{"i32.trunc_f64_s", "(i32.trunc_f64_s (f64.const -2.75))", "i32", 0xfffffffe},
	{"i32.trunc_f64_u", "(i32.trunc_f64_u (f64.const 2.75))", "i32", 2},
	{"i64.trunc_f32_s", "(i64.trunc_f32_s (f32.const -2.75))", "i64", 0xfffffffffffffffe},
	{"i64.trunc_f32_u", "(i64.trunc_f32_u (f32.const 2.75))", "i64", 2},
	{"i64.trunc_f64_s", "(i64.trunc_f64_s (f64.const -2.75))", "i64", 0xfffffffffffffffe},
	{"i64.trunc_f64_u", "(i64.trunc_f64_u (f64.const 2.75))", "i64", 2},
	{"f32.convert_i32_s", "(i32.reinterpret_f32 (f32.convert_i32_s (i32.const -2)))", "i32", 0xc0000000},
	{"f32.convert_i32_u", "(i32.reinterpret_f32 (f32.convert_i32_u (i32.const 2)))", "i32", 0x40000000},
	{"f32.convert_i64_s", "(i32.reinterpret_f32 (f32.convert_i64_s (i64.const -2)))", "i32", 0xc0000000},
	{"f32.convert_i64_u", "(i32.reinterpret_f32 (f32.convert_i64_u (i64.const 2)))", "i32", 0x40000000},
	{"f64.convert_i32_s", "(i64.reinterpret_f64 (f64.convert_i32_s (i32.const -2)))", "i64", 0xc000000000000000},
	{"f64.convert_i32_u", "(i64.reinterpret_f64 (f64.convert_i32_u (i32.const 2)))", "i64", 0x4000000000000000},
	{"f64.convert_i64_s", "(i64.reinterpret_f64 (f64.convert_i64_s (i64.const -2)))", "i64", 0xc000000000000000},
	{"f64.convert_i64_u", "(i64.reinterpret_f64 (f64.convert_i64_u (i64.const 2)))", "i64", 0x4000000000000000},
	{"f32.demote_f64", "(i32.reinterpret_f32 (f32.demote_f64 (f64.const 1.5)))", "i32", 0x3fc00000},
	{"f64.promote_f32", "(i64.reinterpret_f64 (f64.promote_f32 (f32.const 1.5)))", "i64", 0x3ff8000000000000},
	{"f32.reinterpret_i32", "(i32.reinterpret_f32 (f32.reinterpret_i32 (i32.const 0x3fc00000)))", "i32", 0x3fc00000},
	{"f64.reinterpret_i64", "(i64.reinterpret_f64 (f64.reinterpret_i64 (i64.const 0x3ff8000000000000)))", "i64", 0x3ff8000000000000},
	// trunc_sat family (0xfc 0..7): in-range, clamped, and NaN->0
	{"i32.trunc_sat_f32_s", "(i32.trunc_sat_f32_s (f32.const -2.75))", "i32", 0xfffffffe},
	{"i32.trunc_sat_f32_u.clamp", "(i32.trunc_sat_f32_u (f32.const -1))", "i32", 0},
	{"i32.trunc_sat_f64_s.clamp", "(i32.trunc_sat_f64_s (f64.const 1e300))", "i32", 0x7fffffff},
	{"i32.trunc_sat_f64_u", "(i32.trunc_sat_f64_u (f64.const 7.5))", "i32", 7},
	{"i64.trunc_sat_f32_s.nan", "(i64.trunc_sat_f32_s (f32.const nan))", "i64", 0},
	{"i64.trunc_sat_f32_u.clamp", "(i64.trunc_sat_f32_u (f32.const 1e30))", "i64", 0xffffffffffffffff},
	{"i64.trunc_sat_f64_s", "(i64.trunc_sat_f64_s (f64.const -7.5))", "i64", 0xfffffffffffffff9},
	{"i64.trunc_sat_f64_u.clamp", "(i64.trunc_sat_f64_u (f64.const -1))", "i64", 0},
	// parametric + memory (all load/store widths)
	{"select.t", "(select (i32.const 11) (i32.const 22) (i32.const 1))", "i32", 11},
	{"select.f", "(select (i32.const 11) (i32.const 22) (i32.const 0))", "i32", 22},
	{"mem.i32", "(i32.store (i32.const 0) (i32.const -2)) (i32.load (i32.const 0))", "i32", 0xfffffffe},
	{"mem.i32load8_s", "(i32.store8 (i32.const 8) (i32.const 0x80)) (i32.load8_s (i32.const 8))", "i32", 0xffffff80},
	{"mem.i32load8_u", "(i32.store8 (i32.const 8) (i32.const 0x80)) (i32.load8_u (i32.const 8))", "i32", 0x80},
	{"mem.i32load16_s", "(i32.store16 (i32.const 16) (i32.const 0x8000)) (i32.load16_s (i32.const 16))", "i32", 0xffff8000},
	{"mem.i32load16_u", "(i32.store16 (i32.const 16) (i32.const 0x8000)) (i32.load16_u (i32.const 16))", "i32", 0x8000},
	{"mem.i64", "(i64.store (i32.const 24) (i64.const -2)) (i64.load (i32.const 24))", "i64", 0xfffffffffffffffe},
	{"mem.i64load8_s", "(i64.store8 (i32.const 32) (i64.const 0x80)) (i64.load8_s (i32.const 32))", "i64", 0xffffffffffffff80},
	{"mem.i64load8_u", "(i64.store8 (i32.const 32) (i64.const 0x80)) (i64.load8_u (i32.const 32))", "i64", 0x80},
	{"mem.i64load16_s", "(i64.store16 (i32.const 40) (i64.const 0x8000)) (i64.load16_s (i32.const 40))", "i64", 0xffffffffffff8000},
	{"mem.i64load16_u", "(i64.store16 (i32.const 40) (i64.const 0x8000)) (i64.load16_u (i32.const 40))", "i64", 0x8000},
	{"mem.i64load32_s", "(i64.store32 (i32.const 48) (i64.const 0x80000000)) (i64.load32_s (i32.const 48))", "i64", 0xffffffff80000000},
	{"mem.i64load32_u", "(i64.store32 (i32.const 48) (i64.const 0x80000000)) (i64.load32_u (i32.const 48))", "i64", 0x80000000},
	{"mem.f32", "(f32.store (i32.const 56) (f32.const 1.5)) (i32.reinterpret_f32 (f32.load (i32.const 56)))", "i32", 0x3fc00000},
	{"mem.f64", "(f64.store (i32.const 64) (f64.const 1.5)) (i64.reinterpret_f64 (f64.load (i32.const 64)))", "i64", 0x3ff8000000000000},
	{"mem.size", "(memory.size)", "i32", 2},
	{"mem.grow", "(drop (memory.grow (i32.const 1))) (memory.size)", "i32", 3},
	// local/global plumbing through the slow path
	{"local.tee", "(local.set 0 (i32.const 0)) (drop (local.tee 0 (i32.const 9))) (local.get 0)", "i32", 9},
	{"global.rw", "(global.set 0 (i64.const 77)) (global.get 0)", "i64", 77},
}

// TestOpcodeBatteryDispatchParity runs the battery under the three
// interpreter dispatch modes and asserts every case yields the exact
// expected bits in each — the inline fast path and the table handlers are
// duplicate implementations and must agree opcode by opcode.
func TestOpcodeBatteryDispatchParity(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("(module (memory 2 4) (global (mut i64) (i64.const 0))\n")
	for i, c := range opcodeBattery {
		fmt.Fprintf(&b, "(func (export \"c%d\") (result %s) (local i32) %s)\n", i, c.typ, c.expr)
	}
	b.WriteString(")")
	bin, err := wat.Compile(b.Bytes())
	if err != nil {
		t.Fatalf("battery wat: %v", err)
	}
	modes := []struct {
		name string
		cfg  func() config.ModuleConfig
	}{
		{"fast", func() config.ModuleConfig { return config.ModuleConfig{} }},
		{"metered", func() config.ModuleConfig {
			return config.ModuleConfig{TollStation: tollstation.NewSimpleTollStation(1 << 40)}
		}},
		{"canon", func() config.ModuleConfig { return config.ModuleConfig{CanonicalizeNaNs: true} }},
	}
	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			mod, err := wasman.NewModule(m.cfg(), bytes.NewReader(bin))
			if err != nil {
				t.Fatal(err)
			}
			ins, err := wasman.NewInstance(mod, nil)
			if err != nil {
				t.Fatal(err)
			}
			for i, c := range opcodeBattery {
				r, _, err := ins.CallExportedFunc(fmt.Sprintf("c%d", i))
				if err != nil {
					t.Errorf("%s: %v", c.name, err)
					continue
				}
				got := r[0]
				if c.typ == "i32" {
					got = uint64(uint32(got))
				}
				if got != c.want {
					t.Errorf("%s: got %#x want %#x", c.name, got, c.want)
				}
			}
		})
	}
}
