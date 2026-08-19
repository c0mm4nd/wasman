package tollstation_test

import (
	"errors"
	"math"
	"testing"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/tollstation"
)

// compile-time check: SimpleTollStation must implement TollStation
var _ tollstation.TollStation = (*tollstation.SimpleTollStation)(nil)

func TestNewSimpleTollStation(t *testing.T) {
	// zero max falls back to math.MaxUint64
	ts := tollstation.NewSimpleTollStation(0)
	if ts.GetMax() != uint64(math.MaxUint64) {
		t.Fail()
	}
	if ts.GetToll() != 0 {
		t.Fail()
	}

	// explicit max is kept as-is
	ts = tollstation.NewSimpleTollStation(42)
	if ts.GetMax() != 42 {
		t.Fail()
	}
	if ts.GetToll() != 0 {
		t.Fail()
	}
}

func TestGetOpPrice(t *testing.T) {
	ts := tollstation.NewSimpleTollStation(0)
	// SimpleTollStation charges 1 unit per op, whatever the opcode is
	for _, op := range []expr.OpCode{
		expr.OpCodeI32Const,
		expr.OpCodeI64Const,
		expr.OpCodeGlobalGet,
		expr.OpCodeEnd,
	} {
		if ts.GetOpPrice(op) != 1 {
			t.Fail()
		}
	}
}

func TestAddToll(t *testing.T) {
	ts := tollstation.NewSimpleTollStation(10)

	if err := ts.AddToll(4); err != nil {
		t.Fail()
	}
	if ts.GetToll() != 4 {
		t.Fail()
	}

	// exactly reaching max is allowed
	if err := ts.AddToll(6); err != nil {
		t.Fail()
	}
	if ts.GetToll() != 10 {
		t.Fail()
	}

	// total+toll > max must fail with ErrTollOverflow and not increment
	err := ts.AddToll(1)
	if !errors.Is(err, tollstation.ErrTollOverflow) {
		t.Fail()
	}
	if ts.GetToll() != 10 {
		t.Fail()
	}
}

func TestChargeOp(t *testing.T) {
	ts := tollstation.NewSimpleTollStation(2)

	if err := ts.ChargeOp(expr.OpCodeI32Const); err != nil {
		t.Fail()
	}
	if ts.GetToll() != 1 {
		t.Fail()
	}

	// exactly reaching max is allowed
	if err := ts.ChargeOp(expr.OpCodeI64Const); err != nil {
		t.Fail()
	}
	if ts.GetToll() != 2 {
		t.Fail()
	}

	// total+price > max must fail with ErrTollOverflow and not increment
	err := ts.ChargeOp(expr.OpCodeEnd)
	if !errors.Is(err, tollstation.ErrTollOverflow) {
		t.Fail()
	}
	if ts.GetToll() != 2 {
		t.Fail()
	}

	// a further charge keeps failing and the total stays frozen
	err = ts.ChargeOp(expr.OpCodeEnd)
	if !errors.Is(err, tollstation.ErrTollOverflow) {
		t.Fail()
	}
	if ts.GetToll() != 2 {
		t.Fail()
	}
}
