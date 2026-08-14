package wasman_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
)

// jitInstance instantiates a module file with the JIT enabled (a no-op on
// platforms without native codegen — the test then covers the fallback).
func jitInstance(t *testing.T, path string, enable bool) *wasman.Instance {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mod, err := wasman.NewModule(config.ModuleConfig{EnableJIT: enable}, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	ins, err := wasman.NewInstance(mod, nil)
	if err != nil {
		t.Fatal(err)
	}
	return ins
}

// TestJITMatchesInterpreter runs the benchmark workloads with the JIT on and
// off and requires identical results.
func TestJITMatchesInterpreter(t *testing.T) {
	cases := []struct {
		file, fn string
		arg      uint64
		want     uint64
	}{
		{"bench/testdata/sum.wasm", "sum", 100_000, 5_000_050_000},
		{"bench/testdata/memrw.wasm", "fillsum", 65_536, 8_355_840},
		// fib uses call: outside the JIT subset, exercising the fallback
		{"bench/testdata/fib.wasm", "fib", 20, 6765},
	}
	for _, tc := range cases {
		for _, enable := range []bool{false, true} {
			ins := jitInstance(t, tc.file, enable)
			for i := 0; i < 3; i++ { // repeated calls reuse stack/locals
				rets, _, err := ins.CallExportedFunc(tc.fn, tc.arg)
				if err != nil {
					t.Fatalf("%s (jit=%v): %v", tc.file, enable, err)
				}
				if rets[0] != tc.want {
					t.Fatalf("%s (jit=%v): got %d, want %d", tc.file, enable, rets[0], tc.want)
				}
			}
		}
	}
}

// TestJITOOBTrap checks that a JIT-compiled body traps out-of-bounds accesses
// with the interpreter's error and leaves the instance usable.
func TestJITOOBTrap(t *testing.T) {
	ins := jitInstance(t, "bench/testdata/memrw.wasm", true)
	// fillsum(n) touches [0, n); one page is 65536 bytes, so 70000 overruns
	if _, _, err := ins.CallExportedFunc("fillsum", 70_000); err == nil {
		t.Fatal("expected an out-of-bounds trap")
	}
	// the instance must remain usable after the trap
	rets, _, err := ins.CallExportedFunc("fillsum", 1024)
	if err != nil {
		t.Fatal(err)
	}
	var want uint64
	for i := 0; i < 1024; i++ {
		want += uint64(i & 0xff)
	}
	if rets[0] != want {
		t.Fatalf("got %d, want %d", rets[0], want)
	}
}

// BenchmarkJIT compares the two execution paths on the same workloads.
func BenchmarkJIT(b *testing.B) {
	for _, tc := range []struct {
		name, file, fn string
		arg, want      uint64
	}{
		{"Sum", "bench/testdata/sum.wasm", "sum", 100_000, 5_000_050_000},
		{"MemRW", "bench/testdata/memrw.wasm", "fillsum", 65_536, 8_355_840},
	} {
		for _, mode := range []struct {
			name   string
			enable bool
		}{{"interp", false}, {"jit", true}} {
			b.Run(tc.name+"/"+mode.name, func(b *testing.B) {
				raw, err := os.ReadFile(tc.file)
				if err != nil {
					b.Fatal(err)
				}
				mod, err := wasman.NewModule(config.ModuleConfig{EnableJIT: mode.enable}, bytes.NewReader(raw))
				if err != nil {
					b.Fatal(err)
				}
				ins, err := wasman.NewInstance(mod, nil)
				if err != nil {
					b.Fatal(err)
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					rets, _, err := ins.CallExportedFunc(tc.fn, tc.arg)
					if err != nil {
						b.Fatal(err)
					}
					if rets[0] != tc.want {
						b.Fatalf("got %d", rets[0])
					}
				}
			})
		}
	}
}
