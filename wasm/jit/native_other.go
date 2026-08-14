//go:build !((darwin || linux) && (arm64 || amd64))

package jit

// NativeCallsSupported reports whether this backend implements the
// native-call ABI.
func NativeCallsSupported() bool { return false }
