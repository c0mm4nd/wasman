package instr

type OpCode byte

const (
	// control instruction
	OpCodeUnreachable  OpCode = 0x00
	OpCodeNop          OpCode = 0x01
	OpCodeBlock        OpCode = 0x02
	OpCodeLoop         OpCode = 0x03
	OpCodeIf           OpCode = 0x04
	OpCodeElse         OpCode = 0x05
	OpCodeEnd          OpCode = 0x0b
	OpCodeBr           OpCode = 0x0c
	OpCodeBrIf         OpCode = 0x0d
	OpCodeBrTable      OpCode = 0x0e
	OpCodeReturn       OpCode = 0x0f
	OpCodeCall         OpCode = 0x10
	OpCodeCallIndirect OpCode = 0x11

	// parametric instruction
	OpCodeDrop   OpCode = 0x1a
	OpCodeSelect OpCode = 0x1b

	// variable instruction
	OpCodeLocalGet  OpCode = 0x20
	OpCodeLocalSet  OpCode = 0x21
	OpCodeLocalTee  OpCode = 0x22
	OpCodeGlobalGet OpCode = 0x23
	OpCodeGlobalSet OpCode = 0x24

	// memory instruction
	OpCodeI32Load    OpCode = 0x28
	OpCodeI64Load    OpCode = 0x29
	OpCodeF32Load    OpCode = 0x2a
	OpCodeF64Load    OpCode = 0x2b
	OpCodeI32Load8s  OpCode = 0x2c
	OpCodeI32Load8u  OpCode = 0x2d
	OpCodeI32Load16s OpCode = 0x2e
	OpCodeI32Load16u OpCode = 0x2f
	OpCodeI64Load8s  OpCode = 0x30
	OpCodeI64Load8u  OpCode = 0x31
	OpCodeI64Load16s OpCode = 0x32
	OpCodeI64Load16u OpCode = 0x33
	OpCodeI64Load32s OpCode = 0x34
	OpCodeI64Load32u OpCode = 0x35
	OpCodeI32Store   OpCode = 0x36
	OpCodeI64Store   OpCode = 0x37
	OpCodeF32Store   OpCode = 0x38
	OpCodeF64Store   OpCode = 0x39
	OpCodeI32Store8  OpCode = 0x3a
	OpCodeI32Store16 OpCode = 0x3b
	OpCodeI64Store8  OpCode = 0x3c
	OpCodeI64Store16 OpCode = 0x3d
	OpCodeI64Store32 OpCode = 0x3e
	OpCodeMemorySize OpCode = 0x3f
	OpCodeMemoryGrow OpCode = 0x40

	// numeric instruction
	OpCodeI32Const OpCode = 0x41
	OpCodeI64Const OpCode = 0x42
	OpCodeF32Const OpCode = 0x43
	OpCodeF64Const OpCode = 0x44

	OpCodeI32eqz OpCode = 0x45
	OpCodeI32eq  OpCode = 0x46
	OpCodeI32ne  OpCode = 0x47
	OpCodeI32lts OpCode = 0x48
	OpCodeI32ltu OpCode = 0x49
	OpCodeI32gts OpCode = 0x4a
	OpCodeI32gtu OpCode = 0x4b
	OpCodeI32les OpCode = 0x4c
	OpCodeI32leu OpCode = 0x4d
	OpCodeI32ges OpCode = 0x4e
	OpCodeI32geu OpCode = 0x4f

	OpCodeI64eqz OpCode = 0x50
	OpCodeI64eq  OpCode = 0x51
	OpCodeI64ne  OpCode = 0x52
	OpCodeI64lts OpCode = 0x53
	OpCodeI64ltu OpCode = 0x54
	OpCodeI64gts OpCode = 0x55
	OpCodeI64gtu OpCode = 0x56
	OpCodeI64les OpCode = 0x57
	OpCodeI64leu OpCode = 0x58
	OpCodeI64ges OpCode = 0x59
	OpCodeI64geu OpCode = 0x5a

	OpCodeF32eq OpCode = 0x5b
	OpCodeF32ne OpCode = 0x5c
	OpCodeF32lt OpCode = 0x5d
	OpCodeF32gt OpCode = 0x5e
	OpCodeF32le OpCode = 0x5f
	OpCodeF32ge OpCode = 0x60

	OpCodeF64eq OpCode = 0x61
	OpCodeF64ne OpCode = 0x62
	OpCodeF64lt OpCode = 0x63
	OpCodeF64gt OpCode = 0x64
	OpCodeF64le OpCode = 0x65
	OpCodeF64ge OpCode = 0x66

	OpCodeI32clz    OpCode = 0x67
	OpCodeI32ctz    OpCode = 0x68
	OpCodeI32popcnt OpCode = 0x69
	OpCodeI32add    OpCode = 0x6a
	OpCodeI32sub    OpCode = 0x6b
	OpCodeI32mul    OpCode = 0x6c
	OpCodeI32divs   OpCode = 0x6d
	OpCodeI32divu   OpCode = 0x6e
	OpCodeI32rems   OpCode = 0x6f
	OpCodeI32remu   OpCode = 0x70
	OpCodeI32and    OpCode = 0x71
	OpCodeI32or     OpCode = 0x72
	OpCodeI32xor    OpCode = 0x73
	OpCodeI32shl    OpCode = 0x74
	OpCodeI32shrs   OpCode = 0x75
	OpCodeI32shru   OpCode = 0x76
	OpCodeI32rotl   OpCode = 0x77
	OpCodeI32rotr   OpCode = 0x78

	OpCodeI64clz    OpCode = 0x79
	OpCodeI64ctz    OpCode = 0x7a
	OpCodeI64popcnt OpCode = 0x7b
	OpCodeI64add    OpCode = 0x7c
	OpCodeI64sub    OpCode = 0x7d
	OpCodeI64mul    OpCode = 0x7e
	OpCodeI64divs   OpCode = 0x7f
	OpCodeI64divu   OpCode = 0x80
	OpCodeI64rems   OpCode = 0x81
	OpCodeI64remu   OpCode = 0x82
	OpCodeI64and    OpCode = 0x83
	OpCodeI64or     OpCode = 0x84
	OpCodeI64xor    OpCode = 0x85
	OpCodeI64shl    OpCode = 0x86
	OpCodeI64shrs   OpCode = 0x87
	OpCodeI64shru   OpCode = 0x88
	OpCodeI64rotl   OpCode = 0x89
	OpCodeI64rotr   OpCode = 0x8a

	OpCodeF32abs      OpCode = 0x8b
	OpCodeF32neg      OpCode = 0x8c
	OpCodeF32ceil     OpCode = 0x8d
	OpCodeF32floor    OpCode = 0x8e
	OpCodeF32trunc    OpCode = 0x8f
	OpCodeF32nearest  OpCode = 0x90
	OpCodeF32sqrt     OpCode = 0x91
	OpCodeF32add      OpCode = 0x92
	OpCodeF32sub      OpCode = 0x93
	OpCodeF32mul      OpCode = 0x94
	OpCodeF32div      OpCode = 0x95
	OpCodeF32min      OpCode = 0x96
	OpCodeF32max      OpCode = 0x97
	OpCodeF32copysign OpCode = 0x98

	OpCodeF64abs      OpCode = 0x99
	OpCodeF64neg      OpCode = 0x9a
	OpCodeF64ceil     OpCode = 0x9b
	OpCodeF64floor    OpCode = 0x9c
	OpCodeF64trunc    OpCode = 0x9d
	OpCodeF64nearest  OpCode = 0x9e
	OpCodeF64sqrt     OpCode = 0x9f
	OpCodeF64add      OpCode = 0xa0
	OpCodeF64sub      OpCode = 0xa1
	OpCodeF64mul      OpCode = 0xa2
	OpCodeF64div      OpCode = 0xa3
	OpCodeF64min      OpCode = 0xa4
	OpCodeF64max      OpCode = 0xa5
	OpCodeF64copysign OpCode = 0xa6

	OpCodeI32wrapI64   OpCode = 0xa7
	OpCodeI32truncf32s OpCode = 0xa8
	OpCodeI32truncf32u OpCode = 0xa9
	OpCodeI32truncf64s OpCode = 0xaa
	OpCodeI32truncf64u OpCode = 0xab

	OpCodeI64Extendi32s OpCode = 0xac
	OpCodeI64Extendi32u OpCode = 0xad
	OpCodeI64TruncF32s  OpCode = 0xae
	OpCodeI64TruncF32u  OpCode = 0xaf
	OpCodeI64Truncf64s  OpCode = 0xb0
	OpCodeI64Truncf64u  OpCode = 0xb1

	OpCodeF32Converti32s OpCode = 0xb2
	OpCodeF32Converti32u OpCode = 0xb3
	OpCodeF32Converti64s OpCode = 0xb4
	OpCodeF32Converti64u OpCode = 0xb5
	OpCodeF32Demotef64   OpCode = 0xb6

	OpCodeF64Converti32s OpCode = 0xb7
	OpCodeF64Converti32u OpCode = 0xb8
	OpCodeF64Converti64s OpCode = 0xb9
	OpCodeF64Converti64u OpCode = 0xba
	OpCodeF64Promotef32  OpCode = 0xbb

	OpCodeI32reinterpretf32 OpCode = 0xbc
	OpCodeI64reinterpretf64 OpCode = 0xbd
	OpCodeF32reinterpreti32 OpCode = 0xbe
	OpCodeF64reinterpreti64 OpCode = 0xbf
)
