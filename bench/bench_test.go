// Package bench compares wasman against other Go WebAssembly runtimes on
// three workloads: call-heavy recursion (fib), arithmetic looping (sum) and
// linear-memory traffic (memrw), plus module instantiation cost.
//
// It lives in its own module so the main wasman module keeps zero
// dependencies. Run: go test -bench . -benchmem ./bench
package bench

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

var (
	fibWasm   = mustRead("testdata/fib.wasm")
	sumWasm   = mustRead("testdata/sum.wasm")
	memrwWasm = mustRead("testdata/memrw.wasm")
)

const (
	fibN   = 20      // 13529 calls
	sumN   = 100_000 // loop iterations
	memrwN = 65_536  // bytes written + read
)

func mustRead(p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		panic(err)
	}
	return b
}

// --- wasman ---

func wasmanInstance(b *testing.B, wasm []byte) *wasman.Instance {
	b.Helper()
	mod, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(wasm))
	if err != nil {
		b.Fatal(err)
	}
	ins, err := wasman.NewInstance(mod, nil)
	if err != nil {
		b.Fatal(err)
	}
	return ins
}

func benchWasman(b *testing.B, wasm []byte, fn string, arg, want uint64) {
	ins := wasmanInstance(b, wasm)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rets, _, err := ins.CallExportedFunc(fn, arg)
		if err != nil {
			b.Fatal(err)
		}
		if rets[0] != want {
			b.Fatalf("got %d, want %d", rets[0], want)
		}
	}
}

// --- wazero (interpreter and compiler modes) ---

func wazeroFunc(b *testing.B, cfg wazero.RuntimeConfig, wasm []byte, fn string) (api.Function, func()) {
	b.Helper()
	ctx := context.Background()
	r := wazero.NewRuntimeWithConfig(ctx, cfg)
	mod, err := r.Instantiate(ctx, wasm)
	if err != nil {
		b.Fatal(err)
	}
	return mod.ExportedFunction(fn), func() { _ = r.Close(ctx) }
}

func benchWazero(b *testing.B, cfg wazero.RuntimeConfig, wasm []byte, fn string, arg, want uint64) {
	f, done := wazeroFunc(b, cfg, wasm, fn)
	defer done()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rets, err := f.Call(ctx, arg)
		if err != nil {
			b.Fatal(err)
		}
		if rets[0] != want {
			b.Fatalf("got %d, want %d", rets[0], want)
		}
	}
}

// --- workloads ---

const (
	fibWant   = 6765 // fib(20)
	sumWant   = uint64(sumN) * (sumN + 1) / 2
	memrwWant = 8_355_840 // sum of (i & 0xff) for i in [0,65536)
)

func BenchmarkFib(b *testing.B) {
	b.Run("wasman", func(b *testing.B) { benchWasman(b, fibWasm, "fib", fibN, fibWant) })
	b.Run("wazero-interp", func(b *testing.B) {
		benchWazero(b, wazero.NewRuntimeConfigInterpreter(), fibWasm, "fib", fibN, fibWant)
	})
	b.Run("wazero-compiler", func(b *testing.B) {
		benchWazero(b, wazero.NewRuntimeConfigCompiler(), fibWasm, "fib", fibN, fibWant)
	})
}

func BenchmarkSum(b *testing.B) {
	b.Run("wasman", func(b *testing.B) { benchWasman(b, sumWasm, "sum", sumN, sumWant) })
	b.Run("wazero-interp", func(b *testing.B) {
		benchWazero(b, wazero.NewRuntimeConfigInterpreter(), sumWasm, "sum", sumN, sumWant)
	})
	b.Run("wazero-compiler", func(b *testing.B) {
		benchWazero(b, wazero.NewRuntimeConfigCompiler(), sumWasm, "sum", sumN, sumWant)
	})
}

func BenchmarkMemRW(b *testing.B) {
	b.Run("wasman", func(b *testing.B) { benchWasman(b, memrwWasm, "fillsum", memrwN, memrwWant) })
	b.Run("wazero-interp", func(b *testing.B) {
		benchWazero(b, wazero.NewRuntimeConfigInterpreter(), memrwWasm, "fillsum", memrwN, memrwWant)
	})
	b.Run("wazero-compiler", func(b *testing.B) {
		benchWazero(b, wazero.NewRuntimeConfigCompiler(), memrwWasm, "fillsum", memrwN, memrwWant)
	})
}

func BenchmarkInstantiate(b *testing.B) {
	b.Run("wasman", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mod, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(fibWasm))
			if err != nil {
				b.Fatal(err)
			}
			if _, err := wasman.NewInstance(mod, nil); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("wazero-interp", func(b *testing.B) {
		ctx := context.Background()
		for i := 0; i < b.N; i++ {
			r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter())
			if _, err := r.Instantiate(ctx, fibWasm); err != nil {
				b.Fatal(err)
			}
			_ = r.Close(ctx)
		}
	})
	b.Run("wazero-compiler", func(b *testing.B) {
		ctx := context.Background()
		for i := 0; i < b.N; i++ {
			r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigCompiler())
			if _, err := r.Instantiate(ctx, fibWasm); err != nil {
				b.Fatal(err)
			}
			_ = r.Close(ctx)
		}
	})
}
