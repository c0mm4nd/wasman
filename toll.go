package wasman

import (
	"errors"
	"math"

	"github.com/c0mm4nd/wasman/instr"
)

var (
	//  ErrTollOverflow occurs when the module's toll is larger than its cap
	ErrTollOverflow = errors.New("toll overflow")
)

// TollStation is an interface which acts as a toll counter on the cost of one module
type TollStation interface {
	GetOpPrice(instr.OpCode) uint64
	GetToll() uint64
	AddToll(uint64) error
}

// SimpleTollStation is a simple toll station which charge 1 unit toll per op/instr
type SimpleTollStation struct {
	max   uint64
	total uint64
}

// NewSimpleTollStation creates a new SimpleTollStation, by default the cap/max of toll is math.MaxUint64
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

// GetOpPrice will get the price of one opcode
func (ts *SimpleTollStation) GetOpPrice(_ instr.OpCode) uint64 {
	return 1
}

// GetToll returns the total count in the toll station
func (ts *SimpleTollStation) GetToll() uint64 {
	return ts.total
}

// AddToll adds 1 unit toll per opcode
func (ts *SimpleTollStation) AddToll(toll uint64) error {
	if ts.total > ts.max-toll {
		return ErrTollOverflow
	}

	ts.total += toll
	return nil
}

// GetToll is a helper func for module instance to get the total count of toll
func (ins *Instance) GetToll() uint64 {
	if ins.ModuleConfig != nil && ins.ModuleConfig.TollStation != nil {
		return ins.ModuleConfig.TollStation.GetToll()
	}

	return 0
}

// ManuallyAddToll is a helper func for module instance to forcibly add toll
func (ins *Instance) ManuallyAddToll(toll uint64) error {
	if ins.ModuleConfig != nil && ins.ModuleConfig.TollStation != nil {
		return ins.ModuleConfig.TollStation.AddToll(toll)
	}

	// no error when no TollStation
	return nil
}
