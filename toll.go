package wasman

import (
	"errors"
	"github.com/c0mm4nd/wasman/instr"
	"math"
)

type TollStation interface {
	GetToll() uint64
	AddToll(instr.OpCode) error
}

var (
	ErrCostOverflow = errors.New("cost overflow")
)

type SimpleTollStation struct {
	max   uint64
	total uint64
}

func NewSimpleTollStation(max uint64) *SimpleTollStation {
	if max == 0 {
		max = math.MaxUint64
	}

	totalCost := uint64(0)

	return &SimpleTollStation{
		max:   max,
		total: totalCost,
	}
}

func (cp *SimpleTollStation) GetToll() uint64 {
	return cp.total
}

func (cp *SimpleTollStation) AddToll(_ instr.OpCode) error {
	cost := uint64(1)

	if cp.total > cp.max-cost {
		return ErrCostOverflow
	}

	cp.total+= cost
	return nil
}

func (ins *Instance) GetToll() uint64 {
	if ins.ModuleConfig != nil && ins.ModuleConfig.TollStation != nil {
		return ins.ModuleConfig.TollStation.GetToll()
	}

	return 0
}
