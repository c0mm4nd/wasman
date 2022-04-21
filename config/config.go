package config

import (
	"errors"

	"github.com/c0mm4nd/wasman/tollstation"
)

const (
	// DefaultPageSize is 1<<16
	DefaultPageSize = 65536
)

var (
	// ErrShadowing wont appear if LinkerConfig.DisableShadowing is default false
	ErrShadowing = errors.New("shadowing is disabled")
)

// ModuleConfig is the config applied to the wasman.Module
type ModuleConfig struct {
	DisableFloatPoint bool
	TollStation       tollstation.TollStation
	CallDepthLimit    *uint64
	Recover           bool // avoid panic inside vm
}

// LinkerConfig is the config applied to the wasman.Linker
type LinkerConfig struct {
	DisableShadowing bool // false by default
}
