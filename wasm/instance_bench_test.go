package wasm

import (
	"testing"

	"github.com/c0mm4nd/wasman/stacks"
)

// BenchmarkFetchUint32 exercises the immediate-decoding hot path used by
// local.get/set, global.get/set, call, br, const, etc. It should report
// 0 allocs/op now that the bytes.Reader is reused across calls.
func BenchmarkFetchUint32(b *testing.B) {
	// 0x80 0x80 0x01 is a 3-byte LEB128 for 16384.
	body := []byte{0x80, 0x80, 0x01}
	vm := &Instance{
		Active: &Frame{
			Func: &wasmFunc{body: body},
		},
		OperandStack: stacks.NewOperandStack(),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vm.Active.PC = 0
		if _, err := vm.fetchUint32(); err != nil {
			b.Fatal(err)
		}
	}
}
