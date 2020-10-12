package config

import (
	"errors"
	"github.com/c0mm4nd/wasman/tollstation"
)

const (
	DefaultPageSize = 65536
)

var (
	ErrShadowing = errors.New("shadowing is disabled")
)

// ModuleConfig is the config applied to the wasman.Module
type ModuleConfig struct {
	DisableFloatPoint bool
	TollStation       tollstation.TollStation
}

// ModuleConfig is the config applied to the wasman.Linker
type LinkerConfig struct {
	DisableShadowing bool // false by default
}
