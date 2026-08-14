//go:build (darwin || linux) && (arm64 || amd64)

package jit

import "os"

// compileUnderTest routes every semantic test through the tier selected by
// WASMAN_TEST_TIER: "opt" exercises the optimizing compiler (falling back
// to baseline outside its subset, mirroring the engine's tiering), any
// other value pins the baseline tier.
func compileUnderTest(fd *FuncDesc) (*Compiled, error) {
	if os.Getenv("WASMAN_TEST_TIER") == "opt" {
		if cd, err := CompileOpt(fd); err == nil {
			return cd, nil
		}
	}
	return CompileBaseline(fd)
}
