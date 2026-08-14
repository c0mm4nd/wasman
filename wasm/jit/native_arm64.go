//go:build (darwin || linux) && arm64

package jit

// NativeCallsSupported reports whether this backend implements the
// native-call ABI (direct calls between compiled functions).
func NativeCallsSupported() bool { return true }
