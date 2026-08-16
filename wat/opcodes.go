package wat

// opcode is the binary encoding of one instruction; prefixed ops live in
// the 0xFC space (code is the sub-opcode then)
type opcode struct {
	code     byte
	prefixed bool
}

var opcodes = buildOpcodes()

func buildOpcodes() map[string]opcode {
	t := map[string]opcode{}
	put := func(name string, code byte) { t[name] = opcode{code: code} }
	fc := func(name string, sub byte) { t[name] = opcode{code: sub, prefixed: true} }

	// control
	put("unreachable", 0x00)
	put("nop", 0x01)
	put("br", 0x0c)
	put("br_if", 0x0d)
	put("br_table", 0x0e)
	put("return", 0x0f)
	put("call", 0x10)
	put("call_indirect", 0x11)

	// parametric
	put("drop", 0x1a)
	put("select", 0x1b)

	// variable
	put("local.get", 0x20)
	put("local.set", 0x21)
	put("local.tee", 0x22)
	put("global.get", 0x23)
	put("global.set", 0x24)

	// memory
	put("i32.load", 0x28)
	put("i64.load", 0x29)
	put("f32.load", 0x2a)
	put("f64.load", 0x2b)
	put("i32.load8_s", 0x2c)
	put("i32.load8_u", 0x2d)
	put("i32.load16_s", 0x2e)
	put("i32.load16_u", 0x2f)
	put("i64.load8_s", 0x30)
	put("i64.load8_u", 0x31)
	put("i64.load16_s", 0x32)
	put("i64.load16_u", 0x33)
	put("i64.load32_s", 0x34)
	put("i64.load32_u", 0x35)
	put("i32.store", 0x36)
	put("i64.store", 0x37)
	put("f32.store", 0x38)
	put("f64.store", 0x39)
	put("i32.store8", 0x3a)
	put("i32.store16", 0x3b)
	put("i64.store8", 0x3c)
	put("i64.store16", 0x3d)
	put("i64.store32", 0x3e)
	put("memory.size", 0x3f)
	put("memory.grow", 0x40)

	// constants
	put("i32.const", 0x41)
	put("i64.const", 0x42)
	put("f32.const", 0x43)
	put("f64.const", 0x44)

	// i32 comparison
	put("i32.eqz", 0x45)
	put("i32.eq", 0x46)
	put("i32.ne", 0x47)
	put("i32.lt_s", 0x48)
	put("i32.lt_u", 0x49)
	put("i32.gt_s", 0x4a)
	put("i32.gt_u", 0x4b)
	put("i32.le_s", 0x4c)
	put("i32.le_u", 0x4d)
	put("i32.ge_s", 0x4e)
	put("i32.ge_u", 0x4f)

	// i64 comparison
	put("i64.eqz", 0x50)
	put("i64.eq", 0x51)
	put("i64.ne", 0x52)
	put("i64.lt_s", 0x53)
	put("i64.lt_u", 0x54)
	put("i64.gt_s", 0x55)
	put("i64.gt_u", 0x56)
	put("i64.le_s", 0x57)
	put("i64.le_u", 0x58)
	put("i64.ge_s", 0x59)
	put("i64.ge_u", 0x5a)

	// f32 comparison
	put("f32.eq", 0x5b)
	put("f32.ne", 0x5c)
	put("f32.lt", 0x5d)
	put("f32.gt", 0x5e)
	put("f32.le", 0x5f)
	put("f32.ge", 0x60)

	// f64 comparison
	put("f64.eq", 0x61)
	put("f64.ne", 0x62)
	put("f64.lt", 0x63)
	put("f64.gt", 0x64)
	put("f64.le", 0x65)
	put("f64.ge", 0x66)

	// i32 numeric
	put("i32.clz", 0x67)
	put("i32.ctz", 0x68)
	put("i32.popcnt", 0x69)
	put("i32.add", 0x6a)
	put("i32.sub", 0x6b)
	put("i32.mul", 0x6c)
	put("i32.div_s", 0x6d)
	put("i32.div_u", 0x6e)
	put("i32.rem_s", 0x6f)
	put("i32.rem_u", 0x70)
	put("i32.and", 0x71)
	put("i32.or", 0x72)
	put("i32.xor", 0x73)
	put("i32.shl", 0x74)
	put("i32.shr_s", 0x75)
	put("i32.shr_u", 0x76)
	put("i32.rotl", 0x77)
	put("i32.rotr", 0x78)

	// i64 numeric
	put("i64.clz", 0x79)
	put("i64.ctz", 0x7a)
	put("i64.popcnt", 0x7b)
	put("i64.add", 0x7c)
	put("i64.sub", 0x7d)
	put("i64.mul", 0x7e)
	put("i64.div_s", 0x7f)
	put("i64.div_u", 0x80)
	put("i64.rem_s", 0x81)
	put("i64.rem_u", 0x82)
	put("i64.and", 0x83)
	put("i64.or", 0x84)
	put("i64.xor", 0x85)
	put("i64.shl", 0x86)
	put("i64.shr_s", 0x87)
	put("i64.shr_u", 0x88)
	put("i64.rotl", 0x89)
	put("i64.rotr", 0x8a)

	// f32 numeric
	put("f32.abs", 0x8b)
	put("f32.neg", 0x8c)
	put("f32.ceil", 0x8d)
	put("f32.floor", 0x8e)
	put("f32.trunc", 0x8f)
	put("f32.nearest", 0x90)
	put("f32.sqrt", 0x91)
	put("f32.add", 0x92)
	put("f32.sub", 0x93)
	put("f32.mul", 0x94)
	put("f32.div", 0x95)
	put("f32.min", 0x96)
	put("f32.max", 0x97)
	put("f32.copysign", 0x98)

	// f64 numeric
	put("f64.abs", 0x99)
	put("f64.neg", 0x9a)
	put("f64.ceil", 0x9b)
	put("f64.floor", 0x9c)
	put("f64.trunc", 0x9d)
	put("f64.nearest", 0x9e)
	put("f64.sqrt", 0x9f)
	put("f64.add", 0xa0)
	put("f64.sub", 0xa1)
	put("f64.mul", 0xa2)
	put("f64.div", 0xa3)
	put("f64.min", 0xa4)
	put("f64.max", 0xa5)
	put("f64.copysign", 0xa6)

	// conversions
	put("i32.wrap_i64", 0xa7)
	put("i32.trunc_f32_s", 0xa8)
	put("i32.trunc_f32_u", 0xa9)
	put("i32.trunc_f64_s", 0xaa)
	put("i32.trunc_f64_u", 0xab)
	put("i64.extend_i32_s", 0xac)
	put("i64.extend_i32_u", 0xad)
	put("i64.trunc_f32_s", 0xae)
	put("i64.trunc_f32_u", 0xaf)
	put("i64.trunc_f64_s", 0xb0)
	put("i64.trunc_f64_u", 0xb1)
	put("f32.convert_i32_s", 0xb2)
	put("f32.convert_i32_u", 0xb3)
	put("f32.convert_i64_s", 0xb4)
	put("f32.convert_i64_u", 0xb5)
	put("f32.demote_f64", 0xb6)
	put("f64.convert_i32_s", 0xb7)
	put("f64.convert_i32_u", 0xb8)
	put("f64.convert_i64_s", 0xb9)
	put("f64.convert_i64_u", 0xba)
	put("f64.promote_f32", 0xbb)
	put("i32.reinterpret_f32", 0xbc)
	put("i64.reinterpret_f64", 0xbd)
	put("f32.reinterpret_i32", 0xbe)
	put("f64.reinterpret_i64", 0xbf)

	// sign extension
	put("i32.extend8_s", 0xc0)
	put("i32.extend16_s", 0xc1)
	put("i64.extend8_s", 0xc2)
	put("i64.extend16_s", 0xc3)
	put("i64.extend32_s", 0xc4)

	// saturating truncation (0xFC space)
	fc("i32.trunc_sat_f32_s", 0x00)
	fc("i32.trunc_sat_f32_u", 0x01)
	fc("i32.trunc_sat_f64_s", 0x02)
	fc("i32.trunc_sat_f64_u", 0x03)
	fc("i64.trunc_sat_f32_s", 0x04)
	fc("i64.trunc_sat_f32_u", 0x05)
	fc("i64.trunc_sat_f64_s", 0x06)
	fc("i64.trunc_sat_f64_u", 0x07)

	// bulk memory (0xFC space)
	fc("memory.copy", 0x0a)
	fc("memory.fill", 0x0b)

	return t
}
