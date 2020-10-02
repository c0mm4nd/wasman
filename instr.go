package wasman

import (
	"github.com/c0mm4nd/wasman/instr"
)

type wasmContext struct {
	PC         uint64
	Func       *wasmFunc
	Locals     []uint64
	LabelStack *labelStack
}

var instructions = [256]func(ins *Instance) error{
	instr.OpCodeUnreachable:       unreachable,
	instr.OpCodeNop:               nop,
	instr.OpCodeBlock:             block,
	instr.OpCodeLoop:              loop,
	instr.OpCodeIf:                ifOp,
	instr.OpCodeElse:              elseOp,
	instr.OpCodeEnd:               end,
	instr.OpCodeBr:                br,
	instr.OpCodeBrIf:              brIf,
	instr.OpCodeBrTable:           brTable,
	instr.OpCodeReturn:            _return,
	instr.OpCodeCall:              call,
	instr.OpCodeCallIndirect:      callIndirect,
	instr.OpCodeDrop:              drop,
	instr.OpCodeSelect:            selectOp,
	instr.OpCodeLocalGet:          getLocal,
	instr.OpCodeLocalSet:          setLocal,
	instr.OpCodeLocalTee:          teeLocal,
	instr.OpCodeGlobalGet:         getGlobal,
	instr.OpCodeGlobalSet:         setGlobal,
	instr.OpCodeI32Load:           i32Load,
	instr.OpCodeI64Load:           i64Load,
	instr.OpCodeF32Load:           f32Load,
	instr.OpCodeF64Load:           f64Load,
	instr.OpCodeI32Load8s:         i32Load8s,
	instr.OpCodeI32Load8u:         i32Load8u,
	instr.OpCodeI32Load16s:        i32Load16s,
	instr.OpCodeI32Load16u:        i32Load16u,
	instr.OpCodeI64Load8s:         i64Load8s,
	instr.OpCodeI64Load8u:         i64Load8u,
	instr.OpCodeI64Load16s:        i64Load16s,
	instr.OpCodeI64Load16u:        i64Load16u,
	instr.OpCodeI64Load32s:        i64Load32s,
	instr.OpCodeI64Load32u:        i64Load32u,
	instr.OpCodeI32Store:          i32Store,
	instr.OpCodeI64Store:          i64Store,
	instr.OpCodeF32Store:          f32Store,
	instr.OpCodeF64Store:          f64Store,
	instr.OpCodeI32Store8:         i32Store8,
	instr.OpCodeI32Store16:        i32Store16,
	instr.OpCodeI64Store8:         i64Store8,
	instr.OpCodeI64Store16:        i64Store16,
	instr.OpCodeI64Store32:        i64Store32,
	instr.OpCodeMemorySize:        memorySize,
	instr.OpCodeMemoryGrow:        memoryGrow,
	instr.OpCodeI32Const:          i32Const,
	instr.OpCodeI64Const:          i64Const,
	instr.OpCodeF32Const:          f32Const,
	instr.OpCodeF64Const:          f64Const,
	instr.OpCodeI32Eqz:            i32eqz,
	instr.OpCodeI32Eq:             i32eq,
	instr.OpCodeI32Ne:             i32ne,
	instr.OpCodeI32LtS:            i32lts,
	instr.OpCodeI32LtU:            i32ltu,
	instr.OpCodeI32GtS:            i32gts,
	instr.OpCodeI32GtU:            i32gtu,
	instr.OpCodeI32LeS:            i32les,
	instr.OpCodeI32LeU:            i32leu,
	instr.OpCodeI32GeS:            i32ges,
	instr.OpCodeI32GeU:            i32geu,
	instr.OpCodeI64Eqz:            i64eqz,
	instr.OpCodeI64Eq:             i64eq,
	instr.OpCodeI64Ne:             i64ne,
	instr.OpCodeI64LtS:            i64lts,
	instr.OpCodeI64LtU:            i64ltu,
	instr.OpCodeI64GtS:            i64gts,
	instr.OpCodeI64GtU:            i64gtu,
	instr.OpCodeI64LeS:            i64les,
	instr.OpCodeI64LeU:            i64leu,
	instr.OpCodeI64GeS:            i64ges,
	instr.OpCodeI64GeU:            i64geu,
	instr.OpCodeF32Eq:             f32eq,
	instr.OpCodeF32Ne:             f32ne,
	instr.OpCodeF32Lt:             f32lt,
	instr.OpCodeF32Gt:             f32gt,
	instr.OpCodeF32Le:             f32le,
	instr.OpCodeF32Ge:             f32ge,
	instr.OpCodeF64Eq:             f64eq,
	instr.OpCodeF64Ne:             f64ne,
	instr.OpCodeF64Lt:             f64lt,
	instr.OpCodeF64Gt:             f64gt,
	instr.OpCodeF64Le:             f64le,
	instr.OpCodeF64Ge:             f64ge,
	instr.OpCodeI32Clz:            i32clz,
	instr.OpCodeI32Ctz:            i32ctz,
	instr.OpCodeI32PopCnt:         i32popcnt,
	instr.OpCodeI32Add:            i32add,
	instr.OpCodeI32Sub:            i32sub,
	instr.OpCodeI32Mul:            i32mul,
	instr.OpCodeI32DivS:           i32divs,
	instr.OpCodeI32DivU:           i32divu,
	instr.OpCodeI32RemS:           i32rems,
	instr.OpCodeI32RemU:           i32remu,
	instr.OpCodeI32And:            i32and,
	instr.OpCodeI32Or:             i32or,
	instr.OpCodeI32Xor:            i32xor,
	instr.OpCodeI32Shl:            i32shl,
	instr.OpCodeI32ShrS:           i32shrs,
	instr.OpCodeI32ShrU:           i32shru,
	instr.OpCodeI32RotL:           i32rotl,
	instr.OpCodeI32RotR:           i32rotr,
	instr.OpCodeI64Clz:            i64clz,
	instr.OpCodeI64Ctz:            i64ctz,
	instr.OpCodeI64PopCnt:         i64popcnt,
	instr.OpCodeI64Add:            i64add,
	instr.OpCodeI64Sub:            i64sub,
	instr.OpCodeI64Mul:            i64mul,
	instr.OpCodeI64DivS:           i64divs,
	instr.OpCodeI64DivU:           i64divu,
	instr.OpCodeI64RemS:           i64rems,
	instr.OpCodeI64RemU:           i64remu,
	instr.OpCodeI64And:            i64and,
	instr.OpCodeI64Or:             i64or,
	instr.OpCodeI64Xor:            i64xor,
	instr.OpCodeI64Shl:            i64shl,
	instr.OpCodeI64ShrS:           i64shrs,
	instr.OpCodeI64ShrU:           i64shru,
	instr.OpCodeI64RotL:           i64rotl,
	instr.OpCodeI64RotR:           i64rotr,
	instr.OpCodeF32Abs:            f32abs,
	instr.OpCodeF32Neg:            f32neg,
	instr.OpCodeF32Ceil:           f32ceil,
	instr.OpCodeF32Floor:          f32floor,
	instr.OpCodeF32Trunc:          f32trunc,
	instr.OpCodeF32Nearest:        f32nearest,
	instr.OpCodeF32Sqrt:           f32sqrt,
	instr.OpCodeF32Add:            f32add,
	instr.OpCodeF32Sub:            f32sub,
	instr.OpCodeF32Mul:            f32mul,
	instr.OpCodeF32Div:            f32div,
	instr.OpCodeF32Min:            f32min,
	instr.OpCodeF32Max:            f32max,
	instr.OpCodeF32CopySign:       f32copysign,
	instr.OpCodeF64Abs:            f64abs,
	instr.OpCodeF64Neg:            f64neg,
	instr.OpCodeF64Ceil:           f64ceil,
	instr.OpCodeF64Floor:          f64floor,
	instr.OpCodeF64Trunc:          f64trunc,
	instr.OpCodeF64Nearest:        f64nearest,
	instr.OpCodeF64Sqrt:           f64sqrt,
	instr.OpCodeF64Add:            f64add,
	instr.OpCodeF64Sub:            f64sub,
	instr.OpCodeF64Mul:            f64mul,
	instr.OpCodeF64Div:            f64div,
	instr.OpCodeF64Min:            f64min,
	instr.OpCodeF64Max:            f64max,
	instr.OpCodeF64CopySign:       f64copysign,
	instr.OpCodeI32WrapI64:        i32wrapi64,
	instr.OpCodeI32TruncF32S:      i32truncf32s,
	instr.OpCodeI32TruncF32U:      i32truncf32u,
	instr.OpCodeI32truncF64S:      i32truncf64s,
	instr.OpCodeI32truncF64U:      i32truncf64u,
	instr.OpCodeI64ExtendI32S:     i64extendi32s,
	instr.OpCodeI64ExtendI32U:     i64extendi32u,
	instr.OpCodeI64TruncF32S:      i64truncf32s,
	instr.OpCodeI64TruncF32U:      i64truncf32u,
	instr.OpCodeI64TruncF64S:      i64truncf64s,
	instr.OpCodeI64TruncF64U:      i64truncf64u,
	instr.OpCodeF32ConvertI32S:    f32converti32s,
	instr.OpCodeF32ConvertI32U:    f32converti32u,
	instr.OpCodeF32ConvertI64S:    f32converti64s,
	instr.OpCodeF32ConvertI64U:    f32converti64u,
	instr.OpCodeF32DemoteF64:      f32demotef64,
	instr.OpCodeF64ConvertI32S:    f64converti32s,
	instr.OpCodeF64ConvertI32U:    f64converti32u,
	instr.OpCodeF64ConvertI64S:    f64converti64s,
	instr.OpCodeF64ConvertI64U:    f64converti64u,
	instr.OpCodeF64PromoteF32:     f64promotef32,
	instr.OpCodeI32ReinterpretF32: nop,
	instr.OpCodeI64ReinterpretF64: nop,
	instr.OpCodeF32ReinterpretI32: nop,
	instr.OpCodeF64ReinterpretI64: nop,
}
