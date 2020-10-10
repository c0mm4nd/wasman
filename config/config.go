package config

const (
	// magic = 0x0061736D
	DefaultPageSize = 65536
)

// ModuleConfig is the config applied to the module
type ModuleConfig struct {
	DisableFloatPoint bool
	TollStation       TollStation
}
