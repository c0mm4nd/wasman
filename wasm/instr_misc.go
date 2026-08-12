package wasm

import (
	"fmt"
	"math"

	"github.com/c0mm4nd/wasman/expr"
)

// miscInstructions dispatches the 0xfc-prefixed sub-opcodes (keyed by the
// LEB128 uint32 that follows the prefix byte).
var miscInstructions = map[uint32]func(ins *Instance) error{
	expr.OpCodeMiscI32TruncSatF32S: i32truncSatF32S,
	expr.OpCodeMiscI32TruncSatF32U: i32truncSatF32U,
	expr.OpCodeMiscI32TruncSatF64S: i32truncSatF64S,
	expr.OpCodeMiscI32TruncSatF64U: i32truncSatF64U,
	expr.OpCodeMiscI64TruncSatF32S: i64truncSatF32S,
	expr.OpCodeMiscI64TruncSatF32U: i64truncSatF32U,
	expr.OpCodeMiscI64TruncSatF64S: i64truncSatF64S,
	expr.OpCodeMiscI64TruncSatF64U: i64truncSatF64U,
}

// miscPrefix handles the 0xfc prefix: it reads the following LEB128 sub-opcode
// and dispatches to the matching handler.
func miscPrefix(ins *Instance) error {
	ins.Active.PC++
	sub, err := ins.fetchUint32()
	if err != nil {
		return err
	}
	fn := miscInstructions[sub]
	if fn == nil {
		return fmt.Errorf("%w: 0xfc %d", ErrUnknownOpcode, sub)
	}
	return fn(ins)
}

// Saturating float->int conversions never trap: NaN maps to 0 and
// out-of-range values clamp to the destination min/max.

func satI32S(v float64) int32 {
	if math.IsNaN(v) {
		return 0
	}
	t := math.Trunc(v)
	if t < -2147483648.0 {
		return math.MinInt32
	}
	if t >= 2147483648.0 {
		return math.MaxInt32
	}
	return int32(t)
}

func satU32(v float64) uint32 {
	if math.IsNaN(v) {
		return 0
	}
	t := math.Trunc(v)
	if t < 0 {
		return 0
	}
	if t >= 4294967296.0 {
		return math.MaxUint32
	}
	return uint32(t)
}

func satI64S(v float64) int64 {
	if math.IsNaN(v) {
		return 0
	}
	t := math.Trunc(v)
	if t < -9223372036854775808.0 {
		return math.MinInt64
	}
	if t >= 9223372036854775808.0 {
		return math.MaxInt64
	}
	return int64(t)
}

func satU64(v float64) uint64 {
	if math.IsNaN(v) {
		return 0
	}
	t := math.Trunc(v)
	if t < 0 {
		return 0
	}
	if t >= 18446744073709551616.0 {
		return math.MaxUint64
	}
	return uint64(t)
}

func i32truncSatF32S(ins *Instance) error {
	v := float64(math.Float32frombits(uint32(ins.OperandStack.Pop())))
	ins.OperandStack.Push(uint64(uint32(satI32S(v))))

	return nil
}

func i32truncSatF32U(ins *Instance) error {
	v := float64(math.Float32frombits(uint32(ins.OperandStack.Pop())))
	ins.OperandStack.Push(uint64(satU32(v)))

	return nil
}

func i32truncSatF64S(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64(uint32(satI32S(v))))

	return nil
}

func i32truncSatF64U(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64(satU32(v)))

	return nil
}

func i64truncSatF32S(ins *Instance) error {
	v := float64(math.Float32frombits(uint32(ins.OperandStack.Pop())))
	ins.OperandStack.Push(uint64(satI64S(v)))

	return nil
}

func i64truncSatF32U(ins *Instance) error {
	v := float64(math.Float32frombits(uint32(ins.OperandStack.Pop())))
	ins.OperandStack.Push(satU64(v))

	return nil
}

func i64truncSatF64S(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.Pop())
	ins.OperandStack.Push(uint64(satI64S(v)))

	return nil
}

func i64truncSatF64U(ins *Instance) error {
	v := math.Float64frombits(ins.OperandStack.Pop())
	ins.OperandStack.Push(satU64(v))

	return nil
}
