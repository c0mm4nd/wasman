package expr

type OpCode = byte

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

	OpCodeI32Eqz OpCode = 0x45
	OpCodeI32Eq  OpCode = 0x46
	OpCodeI32Ne  OpCode = 0x47
	OpCodeI32LtS OpCode = 0x48
	OpCodeI32LtU OpCode = 0x49
	OpCodeI32GtS OpCode = 0x4a
	OpCodeI32GtU OpCode = 0x4b
	OpCodeI32LeS OpCode = 0x4c
	OpCodeI32LeU OpCode = 0x4d
	OpCodeI32GeS OpCode = 0x4e
	OpCodeI32GeU OpCode = 0x4f

	OpCodeI64Eqz OpCode = 0x50
	OpCodeI64Eq  OpCode = 0x51
	OpCodeI64Ne  OpCode = 0x52
	OpCodeI64LtS OpCode = 0x53
	OpCodeI64LtU OpCode = 0x54
	OpCodeI64GtS OpCode = 0x55
	OpCodeI64GtU OpCode = 0x56
	OpCodeI64LeS OpCode = 0x57
	OpCodeI64LeU OpCode = 0x58
	OpCodeI64GeS OpCode = 0x59
	OpCodeI64GeU OpCode = 0x5a

	OpCodeF32Eq OpCode = 0x5b
	OpCodeF32Ne OpCode = 0x5c
	OpCodeF32Lt OpCode = 0x5d
	OpCodeF32Gt OpCode = 0x5e
	OpCodeF32Le OpCode = 0x5f
	OpCodeF32Ge OpCode = 0x60

	OpCodeF64Eq OpCode = 0x61
	OpCodeF64Ne OpCode = 0x62
	OpCodeF64Lt OpCode = 0x63
	OpCodeF64Gt OpCode = 0x64
	OpCodeF64Le OpCode = 0x65
	OpCodeF64Ge OpCode = 0x66

	OpCodeI32Clz    OpCode = 0x67
	OpCodeI32Ctz    OpCode = 0x68
	OpCodeI32PopCnt OpCode = 0x69
	OpCodeI32Add    OpCode = 0x6a
	OpCodeI32Sub    OpCode = 0x6b
	OpCodeI32Mul    OpCode = 0x6c
	OpCodeI32DivS   OpCode = 0x6d
	OpCodeI32DivU   OpCode = 0x6e
	OpCodeI32RemS   OpCode = 0x6f
	OpCodeI32RemU   OpCode = 0x70
	OpCodeI32And    OpCode = 0x71
	OpCodeI32Or     OpCode = 0x72
	OpCodeI32Xor    OpCode = 0x73
	OpCodeI32Shl    OpCode = 0x74
	OpCodeI32ShrS   OpCode = 0x75
	OpCodeI32ShrU   OpCode = 0x76
	OpCodeI32RotL   OpCode = 0x77
	OpCodeI32RotR   OpCode = 0x78

	OpCodeI64Clz    OpCode = 0x79
	OpCodeI64Ctz    OpCode = 0x7a
	OpCodeI64PopCnt OpCode = 0x7b
	OpCodeI64Add    OpCode = 0x7c
	OpCodeI64Sub    OpCode = 0x7d
	OpCodeI64Mul    OpCode = 0x7e
	OpCodeI64DivS   OpCode = 0x7f
	OpCodeI64DivU   OpCode = 0x80
	OpCodeI64RemS   OpCode = 0x81
	OpCodeI64RemU   OpCode = 0x82
	OpCodeI64And    OpCode = 0x83
	OpCodeI64Or     OpCode = 0x84
	OpCodeI64Xor    OpCode = 0x85
	OpCodeI64Shl    OpCode = 0x86
	OpCodeI64ShrS   OpCode = 0x87
	OpCodeI64ShrU   OpCode = 0x88
	OpCodeI64RotL   OpCode = 0x89
	OpCodeI64RotR   OpCode = 0x8a

	OpCodeF32Abs      OpCode = 0x8b
	OpCodeF32Neg      OpCode = 0x8c
	OpCodeF32Ceil     OpCode = 0x8d
	OpCodeF32Floor    OpCode = 0x8e
	OpCodeF32Trunc    OpCode = 0x8f
	OpCodeF32Nearest  OpCode = 0x90
	OpCodeF32Sqrt     OpCode = 0x91
	OpCodeF32Add      OpCode = 0x92
	OpCodeF32Sub      OpCode = 0x93
	OpCodeF32Mul      OpCode = 0x94
	OpCodeF32Div      OpCode = 0x95
	OpCodeF32Min      OpCode = 0x96
	OpCodeF32Max      OpCode = 0x97
	OpCodeF32CopySign OpCode = 0x98

	OpCodeF64Abs      OpCode = 0x99
	OpCodeF64Neg      OpCode = 0x9a
	OpCodeF64Ceil     OpCode = 0x9b
	OpCodeF64Floor    OpCode = 0x9c
	OpCodeF64Trunc    OpCode = 0x9d
	OpCodeF64Nearest  OpCode = 0x9e
	OpCodeF64Sqrt     OpCode = 0x9f
	OpCodeF64Add      OpCode = 0xa0
	OpCodeF64Sub      OpCode = 0xa1
	OpCodeF64Mul      OpCode = 0xa2
	OpCodeF64Div      OpCode = 0xa3
	OpCodeF64Min      OpCode = 0xa4
	OpCodeF64Max      OpCode = 0xa5
	OpCodeF64CopySign OpCode = 0xa6

	OpCodeI32WrapI64   OpCode = 0xa7
	OpCodeI32TruncF32S OpCode = 0xa8
	OpCodeI32TruncF32U OpCode = 0xa9
	OpCodeI32truncF64S OpCode = 0xaa
	OpCodeI32truncF64U OpCode = 0xab

	OpCodeI64ExtendI32S OpCode = 0xac
	OpCodeI64ExtendI32U OpCode = 0xad
	OpCodeI64TruncF32S  OpCode = 0xae
	OpCodeI64TruncF32U  OpCode = 0xaf
	OpCodeI64TruncF64S  OpCode = 0xb0
	OpCodeI64TruncF64U  OpCode = 0xb1

	OpCodeF32ConvertI32S OpCode = 0xb2
	OpCodeF32ConvertI32U OpCode = 0xb3
	OpCodeF32ConvertI64S OpCode = 0xb4
	OpCodeF32ConvertI64U OpCode = 0xb5
	OpCodeF32DemoteF64   OpCode = 0xb6

	OpCodeF64ConvertI32S OpCode = 0xb7
	OpCodeF64ConvertI32U OpCode = 0xb8
	OpCodeF64ConvertI64S OpCode = 0xb9
	OpCodeF64ConvertI64U OpCode = 0xba
	OpCodeF64PromoteF32  OpCode = 0xbb

	OpCodeI32ReinterpretF32 OpCode = 0xbc
	OpCodeI64ReinterpretF64 OpCode = 0xbd
	OpCodeF32ReinterpretI32 OpCode = 0xbe
	OpCodeF64ReinterpretI64 OpCode = 0xbf

	OpCodeI32Extend8S  OpCode = 0xc0
	OpCodeI32Extend16S OpCode = 0xc1
	OpCodeI64Extend8S  OpCode = 0xc2
	OpCodeI64Extend16S OpCode = 0xc3
	OpCodeI64Extend32S OpCode = 0xc4

	OpCodeNull   OpCode = 0xd0
	OpCodeIsNull OpCode = 0xd1
	OpCodeFunc   OpCode = 0xd2

	// TODO: 0xfc
)
