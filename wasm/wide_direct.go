package wasm

import (
	"github.com/c0mm4nd/wasman/wasm/jit"
	"github.com/c0mm4nd/wasman/wideint"
)

// The reflection-free dispatch for the built-in wide-integer host
// functions: HostFunc.call routes tagged operations here, so every
// execution path — interpreter, JIT host exits, cross-instance — skips
// reflect.Call and its argument marshalling entirely. The optimizing tier
// additionally inlines the cheap operations; this path serves the rest
// (mul on the baseline tier, div/rem/shifts everywhere).

func wideSpan(ins *Instance, ptr uint32, n int) ([]byte, error) {
	end := uint64(ptr) + uint64(n)
	if ins.Memory == nil || end > uint64(len(ins.Memory.Value)) {
		return nil, ErrPtrOutOfBounds
	}
	return ins.Memory.Value[ptr:end:end], nil
}

func wideDirect(ins *Instance, id uint16) error {
	op, wide256 := jit.WideOpKind(id)
	osk := ins.OperandStack
	w := 16
	if wide256 {
		w = 32
	}

	switch op {
	case jit.WideNot: // (dst, a)
		a := uint32(osk.Pop())
		dst := uint32(osk.Pop())
		da, err := wideSpan(ins, a, w)
		if err != nil {
			return err
		}
		dd, err := wideSpan(ins, dst, w)
		if err != nil {
			return err
		}
		if wide256 {
			wideint.U256FromBytes(da).Not().PutBytes(dd)
		} else {
			wideint.U128FromBytes(da).Not().PutBytes(dd)
		}
		return nil

	case jit.WideIsZero: // (a) -> i32
		a := uint32(osk.Pop())
		da, err := wideSpan(ins, a, w)
		if err != nil {
			return err
		}
		zero := false
		if wide256 {
			zero = wideint.U256FromBytes(da).IsZero()
		} else {
			zero = wideint.U128FromBytes(da).IsZero()
		}
		if zero {
			osk.Push(1)
		} else {
			osk.Push(0)
		}
		return nil

	case jit.WideCmpU, jit.WideCmpS: // (a, b) -> i32
		b := uint32(osk.Pop())
		a := uint32(osk.Pop())
		da, err := wideSpan(ins, a, w)
		if err != nil {
			return err
		}
		db, err := wideSpan(ins, b, w)
		if err != nil {
			return err
		}
		var r int
		if wide256 {
			x, y := wideint.U256FromBytes(da), wideint.U256FromBytes(db)
			if op == jit.WideCmpU {
				r = x.CmpU(y)
			} else {
				r = x.CmpS(y)
			}
		} else {
			x, y := wideint.U128FromBytes(da), wideint.U128FromBytes(db)
			if op == jit.WideCmpU {
				r = x.CmpU(y)
			} else {
				r = x.CmpS(y)
			}
		}
		osk.Push(uint64(int64(r)))
		return nil

	case jit.WideShl, jit.WideShrU, jit.WideShrS: // (dst, a, n)
		n := uint(uint32(osk.Pop()))
		a := uint32(osk.Pop())
		dst := uint32(osk.Pop())
		da, err := wideSpan(ins, a, w)
		if err != nil {
			return err
		}
		dd, err := wideSpan(ins, dst, w)
		if err != nil {
			return err
		}
		if wide256 {
			x := wideint.U256FromBytes(da)
			switch op {
			case jit.WideShl:
				x = x.Shl(n)
			case jit.WideShrU:
				x = x.ShrU(n)
			default:
				x = x.ShrS(n)
			}
			x.PutBytes(dd)
		} else {
			x := wideint.U128FromBytes(da)
			switch op {
			case jit.WideShl:
				x = x.Shl(n)
			case jit.WideShrU:
				x = x.ShrU(n)
			default:
				x = x.ShrS(n)
			}
			x.PutBytes(dd)
		}
		return nil

	default: // three-pointer binary operations
		b := uint32(osk.Pop())
		a := uint32(osk.Pop())
		dst := uint32(osk.Pop())
		da, err := wideSpan(ins, a, w)
		if err != nil {
			return err
		}
		db, err := wideSpan(ins, b, w)
		if err != nil {
			return err
		}
		dd, err := wideSpan(ins, dst, w)
		if err != nil {
			return err
		}
		if wide256 {
			x, y := wideint.U256FromBytes(da), wideint.U256FromBytes(db)
			var r wideint.U256
			switch op {
			case jit.WideAdd:
				r = x.Add(y)
			case jit.WideSub:
				r = x.Sub(y)
			case jit.WideMul:
				r = x.Mul(y)
			case jit.WideDivU:
				r = x.DivU(y)
			case jit.WideDivS:
				r = x.DivS(y)
			case jit.WideRemU:
				r = x.RemU(y)
			case jit.WideRemS:
				r = x.RemS(y)
			case jit.WideAnd:
				r = x.And(y)
			case jit.WideOr:
				r = x.Or(y)
			case jit.WideXor:
				r = x.Xor(y)
			}
			r.PutBytes(dd)
		} else {
			x, y := wideint.U128FromBytes(da), wideint.U128FromBytes(db)
			var r wideint.U128
			switch op {
			case jit.WideAdd:
				r = x.Add(y)
			case jit.WideSub:
				r = x.Sub(y)
			case jit.WideMul:
				r = x.Mul(y)
			case jit.WideDivU:
				r = x.DivU(y)
			case jit.WideDivS:
				r = x.DivS(y)
			case jit.WideRemU:
				r = x.RemU(y)
			case jit.WideRemS:
				r = x.RemS(y)
			case jit.WideAnd:
				r = x.And(y)
			case jit.WideOr:
				r = x.Or(y)
			case jit.WideXor:
				r = x.Xor(y)
			}
			r.PutBytes(dd)
		}
		return nil
	}
}
