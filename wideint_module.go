package wasman

import (
	"errors"

	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/wasm"
	"github.com/c0mm4nd/wasman/wasm/jit"
	"github.com/c0mm4nd/wasman/wideint"
)

// ErrWideMulDivOverflow traps a u256.mul_div whose exact quotient does not
// fit in 256 bits (a caller-side logic error, like Uniswap's mulDiv
// overflow revert). c == 0 is not an overflow: it writes 0.
var ErrWideMulDivOverflow = errors.New("u256.mul_div: quotient overflows 256 bits")

// The optional wide-integer extension: config.ModuleConfig{EnableWideInt:
// true} makes two host modules importable, "u128" and "u256". Values are
// little-endian limb sequences in linear memory (16 or 32 bytes) passed by
// pointer, the layout __int128 and common big-number libraries use;
// signed variants carry the _s suffix, so the namespaces cover i128/i256
// as well. Division follows EVM conventions (by-zero yields zero,
// truncated signed division, MinValue / -1 wraps); out-of-bounds pointers
// trap like any other memory access.
//
//	(import "u256" "add"   (func (param i32 i32 i32)))       ;; dst, a, b
//	(import "u256" "cmp_s" (func (param i32 i32) (result i32)))
//	(import "u128" "shl"   (func (param i32 i32 i32)))       ;; dst, a, bits

// span bounds-checks ptr..ptr+n in the instance's linear memory.
func wideSpan(ins *Instance, ptr uint32, n int) ([]byte, error) {
	end := uint64(ptr) + uint64(n)
	if ins.Memory == nil || end > uint64(len(ins.Memory.Value)) {
		return nil, wasm.ErrPtrOutOfBounds
	}
	return ins.Memory.Value[ptr:end:end], nil
}

func wideIntModules() map[string]*Module {
	l := NewLinker(config.LinkerConfig{})

	// tag inlinable operations so the optimizing tier can compile them to
	// native carry chains instead of host calls
	tag := func(modName, funcName string, op int, wide256 bool) {
		mod := l.Modules[modName]
		idx := mod.ExportSection[funcName].Desc.Index
		if hf, ok := mod.IndexSpace.Functions[idx].(*wasm.HostFunc); ok {
			hf.SetWideOp(jit.WideOpID(op, wide256))
		}
	}

	bin128 := func(name string, op func(a, b wideint.U128) wideint.U128) {
		_ = l.DefineAdvancedFunc("u128", name, func(ins *Instance) interface{} {
			return func(dst, a, b uint32) error {
				dd, err := wideSpan(ins, dst, 16)
				if err != nil {
					return err
				}
				da, err := wideSpan(ins, a, 16)
				if err != nil {
					return err
				}
				db, err := wideSpan(ins, b, 16)
				if err != nil {
					return err
				}
				op(wideint.U128FromBytes(da), wideint.U128FromBytes(db)).PutBytes(dd)
				return nil
			}
		})
	}
	shift128 := func(name string, op func(a wideint.U128, n uint) wideint.U128) {
		_ = l.DefineAdvancedFunc("u128", name, func(ins *Instance) interface{} {
			return func(dst, a, n uint32) error {
				dd, err := wideSpan(ins, dst, 16)
				if err != nil {
					return err
				}
				da, err := wideSpan(ins, a, 16)
				if err != nil {
					return err
				}
				op(wideint.U128FromBytes(da), uint(n)).PutBytes(dd)
				return nil
			}
		})
	}
	bin256 := func(name string, op func(a, b wideint.U256) wideint.U256) {
		_ = l.DefineAdvancedFunc("u256", name, func(ins *Instance) interface{} {
			return func(dst, a, b uint32) error {
				dd, err := wideSpan(ins, dst, 32)
				if err != nil {
					return err
				}
				da, err := wideSpan(ins, a, 32)
				if err != nil {
					return err
				}
				db, err := wideSpan(ins, b, 32)
				if err != nil {
					return err
				}
				op(wideint.U256FromBytes(da), wideint.U256FromBytes(db)).PutBytes(dd)
				return nil
			}
		})
	}
	shift256 := func(name string, op func(a wideint.U256, n uint) wideint.U256) {
		_ = l.DefineAdvancedFunc("u256", name, func(ins *Instance) interface{} {
			return func(dst, a, n uint32) error {
				dd, err := wideSpan(ins, dst, 32)
				if err != nil {
					return err
				}
				da, err := wideSpan(ins, a, 32)
				if err != nil {
					return err
				}
				op(wideint.U256FromBytes(da), uint(n)).PutBytes(dd)
				return nil
			}
		})
	}

	bin128("add", wideint.U128.Add)
	bin128("sub", wideint.U128.Sub)
	bin128("mul", wideint.U128.Mul)
	bin128("div_u", wideint.U128.DivU)
	bin128("rem_u", wideint.U128.RemU)
	bin128("div_s", wideint.U128.DivS)
	bin128("rem_s", wideint.U128.RemS)
	bin128("and", wideint.U128.And)
	bin128("or", wideint.U128.Or)
	bin128("xor", wideint.U128.Xor)
	shift128("shl", wideint.U128.Shl)
	shift128("shr_u", wideint.U128.ShrU)
	shift128("shr_s", wideint.U128.ShrS)
	_ = l.DefineAdvancedFunc("u128", "not", func(ins *Instance) interface{} {
		return func(dst, a uint32) error {
			dd, err := wideSpan(ins, dst, 16)
			if err != nil {
				return err
			}
			da, err := wideSpan(ins, a, 16)
			if err != nil {
				return err
			}
			wideint.U128FromBytes(da).Not().PutBytes(dd)
			return nil
		}
	})
	_ = l.DefineAdvancedFunc("u128", "cmp_u", func(ins *Instance) interface{} {
		return func(a, b uint32) (int32, error) {
			da, err := wideSpan(ins, a, 16)
			if err != nil {
				return 0, err
			}
			db, err := wideSpan(ins, b, 16)
			if err != nil {
				return 0, err
			}
			return int32(wideint.U128FromBytes(da).CmpU(wideint.U128FromBytes(db))), nil
		}
	})
	_ = l.DefineAdvancedFunc("u128", "cmp_s", func(ins *Instance) interface{} {
		return func(a, b uint32) (int32, error) {
			da, err := wideSpan(ins, a, 16)
			if err != nil {
				return 0, err
			}
			db, err := wideSpan(ins, b, 16)
			if err != nil {
				return 0, err
			}
			return int32(wideint.U128FromBytes(da).CmpS(wideint.U128FromBytes(db))), nil
		}
	})
	_ = l.DefineAdvancedFunc("u128", "iszero", func(ins *Instance) interface{} {
		return func(a uint32) (uint32, error) {
			da, err := wideSpan(ins, a, 16)
			if err != nil {
				return 0, err
			}
			if wideint.U128FromBytes(da).IsZero() {
				return 1, nil
			}
			return 0, nil
		}
	})

	bin256("add", wideint.U256.Add)
	bin256("sub", wideint.U256.Sub)
	bin256("mul", wideint.U256.Mul)
	bin256("div_u", wideint.U256.DivU)
	bin256("rem_u", wideint.U256.RemU)
	bin256("div_s", wideint.U256.DivS)
	bin256("rem_s", wideint.U256.RemS)
	bin256("and", wideint.U256.And)
	bin256("or", wideint.U256.Or)
	bin256("xor", wideint.U256.Xor)
	shift256("shl", wideint.U256.Shl)
	shift256("shr_u", wideint.U256.ShrU)
	shift256("shr_s", wideint.U256.ShrS)
	_ = l.DefineAdvancedFunc("u256", "not", func(ins *Instance) interface{} {
		return func(dst, a uint32) error {
			dd, err := wideSpan(ins, dst, 32)
			if err != nil {
				return err
			}
			da, err := wideSpan(ins, a, 32)
			if err != nil {
				return err
			}
			wideint.U256FromBytes(da).Not().PutBytes(dd)
			return nil
		}
	})
	_ = l.DefineAdvancedFunc("u256", "cmp_u", func(ins *Instance) interface{} {
		return func(a, b uint32) (int32, error) {
			da, err := wideSpan(ins, a, 32)
			if err != nil {
				return 0, err
			}
			db, err := wideSpan(ins, b, 32)
			if err != nil {
				return 0, err
			}
			return int32(wideint.U256FromBytes(da).CmpU(wideint.U256FromBytes(db))), nil
		}
	})
	_ = l.DefineAdvancedFunc("u256", "cmp_s", func(ins *Instance) interface{} {
		return func(a, b uint32) (int32, error) {
			da, err := wideSpan(ins, a, 32)
			if err != nil {
				return 0, err
			}
			db, err := wideSpan(ins, b, 32)
			if err != nil {
				return 0, err
			}
			return int32(wideint.U256FromBytes(da).CmpS(wideint.U256FromBytes(db))), nil
		}
	})
	_ = l.DefineAdvancedFunc("u256", "iszero", func(ins *Instance) interface{} {
		return func(a uint32) (uint32, error) {
			da, err := wideSpan(ins, a, 32)
			if err != nil {
				return 0, err
			}
			if wideint.U256FromBytes(da).IsZero() {
				return 1, nil
			}
			return 0, nil
		}
	})
	// mul_div: floor(a*b/c) over the full 512-bit product; c == 0 writes 0
	// (EVM convention), and a quotient wider than 256 bits traps as an
	// overflow (matching Uniswap's revert-on-overflow mulDiv).
	_ = l.DefineAdvancedFunc("u256", "mul_div", func(ins *Instance) interface{} {
		return func(dst, a, b, c uint32) error {
			dd, err := wideSpan(ins, dst, 32)
			if err != nil {
				return err
			}
			da, err := wideSpan(ins, a, 32)
			if err != nil {
				return err
			}
			db, err := wideSpan(ins, b, 32)
			if err != nil {
				return err
			}
			dc, err := wideSpan(ins, c, 32)
			if err != nil {
				return err
			}
			cv := wideint.U256FromBytes(dc)
			res, ok := wideint.U256FromBytes(da).MulDiv(wideint.U256FromBytes(db), cv)
			if !ok && !cv.IsZero() {
				return ErrWideMulDivOverflow
			}
			res.PutBytes(dd)
			return nil
		}
	})
	// isqrt: floor(sqrt(a))
	_ = l.DefineAdvancedFunc("u256", "isqrt", func(ins *Instance) interface{} {
		return func(dst, a uint32) error {
			dd, err := wideSpan(ins, dst, 32)
			if err != nil {
				return err
			}
			da, err := wideSpan(ins, a, 32)
			if err != nil {
				return err
			}
			wideint.U256FromBytes(da).Sqrt().PutBytes(dd)
			return nil
		}
	})

	for i, wide := range []bool{false, true} {
		ns := [2]string{"u128", "u256"}[i]
		tag(ns, "add", jit.WideAdd, wide)
		tag(ns, "sub", jit.WideSub, wide)
		tag(ns, "and", jit.WideAnd, wide)
		tag(ns, "or", jit.WideOr, wide)
		tag(ns, "xor", jit.WideXor, wide)
		tag(ns, "not", jit.WideNot, wide)
		tag(ns, "iszero", jit.WideIsZero, wide)
		tag(ns, "cmp_u", jit.WideCmpU, wide)
		tag(ns, "cmp_s", jit.WideCmpS, wide)
		tag(ns, "mul", jit.WideMul, wide)
		tag(ns, "div_u", jit.WideDivU, wide)
		tag(ns, "div_s", jit.WideDivS, wide)
		tag(ns, "rem_u", jit.WideRemU, wide)
		tag(ns, "rem_s", jit.WideRemS, wide)
		tag(ns, "shl", jit.WideShl, wide)
		tag(ns, "shr_u", jit.WideShrU, wide)
		tag(ns, "shr_s", jit.WideShrS, wide)
	}

	return l.Modules
}
