//go:build !((darwin || linux) && arm64)

package jit

// NativeCallsSupported reports whether this backend implements the
// native-call ABI.
func NativeCallsSupported() bool { return false }
